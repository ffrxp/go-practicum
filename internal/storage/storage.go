package storage

import (
	"context"
	"database/sql/driver"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/jackc/pgconn"
	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v4"
	"github.com/jackc/pgx/v4/pgxpool"
	"log"
	"os"
	"strings"
	"time"
)

type Repository interface {
	AddItem(id string, value string, userID int) error
	AddBatchItems(ids []string, values []string, userID int) error
	GetItem(value string) (string, error)
	GetItemByID(ID string) (string, error)
	GetUserHistory(userID int) (History, error)
	Close() error
}

type sourceFileManager struct {
	file    *os.File
	encoder *json.Encoder
	decoder *json.Decoder
}

type URLConversion struct {
	ShortURL string `json:"short_url"`
	OrigURL  string `json:"original_url"`
}

type dataStorage struct {
	userHistoryStorage map[int][]URLConversion
	storage            map[string]string
	sfm                *sourceFileManager
}

type History []URLConversion

func NewDataStorage(source string) *dataStorage {
	if source == "" {
		return &dataStorage{make(map[int][]URLConversion), make(map[string]string), nil}
	}
	file, err := os.OpenFile(source, os.O_RDWR|os.O_CREATE, 0777)
	if err != nil {
		log.Printf("Cannot open data file")
		return &dataStorage{make(map[int][]URLConversion), make(map[string]string), nil}
	}
	sfm := sourceFileManager{
		file:    file,
		encoder: json.NewEncoder(file),
		decoder: json.NewDecoder(file)}
	ds := dataStorage{make(map[int][]URLConversion), map[string]string{}, &sfm}
	if err := ds.loadItems(); err != nil {
		return &ds
	}
	return &ds
}

func (ms *dataStorage) AddItem(id string, value string, userID int) error {
	if _, ok := ms.storage[id]; ok {
		return errors.New("already exists")
	}
	ms.storage[id] = value
	if ms.sfm != nil {
		if err := ms.sfm.file.Truncate(0); err != nil {
			return err
		}
		if _, err := ms.sfm.file.Seek(0, 0); err != nil {
			return err
		}
		if err := ms.sfm.encoder.Encode(&ms.storage); err != nil {
			return err
		}
	}
	ms.addItemUserHistory(id, value, userID)
	return nil
}

func (ms *dataStorage) AddBatchItems(ids []string, values []string, userID int) error {
	if len(ids) != len(values) {
		return errors.New("number of id and values is not equal")
	}
	for i := 0; i < len(ids); i++ {
		err := ms.AddItem(ids[i], values[i], userID)
		if err != nil {
			return err
		}
	}
	return nil
}

func (ms *dataStorage) GetItem(value string) (string, error) {
	for key, val := range ms.storage {
		if val == value {
			return key, nil
		}
	}
	return "", errors.New("not found")
}

func (ms *dataStorage) GetItemByID(ID string) (string, error) {
	for key, val := range ms.storage {
		if key == ID {
			return val, nil
		}
	}
	return "", errors.New("not found")
}

func (ms *dataStorage) loadItems() error {
	if ms.sfm == nil {
		return nil
	}
	if err := ms.sfm.decoder.Decode(&ms.storage); err != nil {
		return err
	}
	return nil
}

func (ms *dataStorage) addItemUserHistory(id string, value string, userID int) {
	history, ok := ms.userHistoryStorage[userID]
	if ok {
		found := false
		for _, historyElem := range history {
			if historyElem.ShortURL == value && historyElem.OrigURL == id {
				found = true
				break
			}
		}
		if found {
			return
		}
		URLConv := URLConversion{value, id}
		ms.userHistoryStorage[userID] = append(ms.userHistoryStorage[userID], URLConv)
		return
	}
	ms.userHistoryStorage[userID] = make([]URLConversion, 0)
	ms.userHistoryStorage[userID] = append(ms.userHistoryStorage[userID], URLConversion{value, id})
}

func (ms *dataStorage) GetUserHistory(userID int) (History, error) {
	history, ok := ms.userHistoryStorage[userID]
	if !ok {
		return make(History, 0), nil
	}
	return history, nil
}

func (ms *dataStorage) Close() error {
	if ms.sfm != nil {
		return ms.sfm.file.Close()
	}
	return nil
}

type databaseStorage struct {
	pool *pgxpool.Pool
}

func NewDatabaseStorage(source string) (*databaseStorage, error) {
	dbpool, err := pgxpool.Connect(context.Background(), source)
	if err != nil {
		log.Printf("Cannot connect to database")
		return nil, err
	}
	queryCreateConv := "CREATE TABLE IF NOT EXISTS convertions " +
		"(short_url character varying(2048) NOT NULL PRIMARY KEY, orig_url character varying(2048) NOT NULL)"
	if _, err := dbpool.Exec(context.Background(), queryCreateConv); err != nil {
		return nil, err
	}
	queryCreateHistories := "CREATE TABLE IF NOT EXISTS histories " +
		"(user_id integer NOT NULL PRIMARY KEY, history text NOT NULL)"
	if _, err := dbpool.Exec(context.Background(), queryCreateHistories); err != nil {
		return nil, err
	}

	return &databaseStorage{dbpool}, nil
}

func (dbs *databaseStorage) Close() error {
	dbs.pool.Close()
	return nil
}

func (dbs *databaseStorage) AddItem(id string, value string, userID int) error {
	if _, err := dbs.pool.Exec(context.Background(),
		"INSERT INTO convertions (short_url, orig_url) VALUES ($1, $2)", value, id); err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			if pgErr.Code == pgerrcode.UniqueViolation {
				return errors.New("already exists")
			}
			return err
		}
		return err
	}
	if err := dbs.addItemUserHistory(id, value, userID); err != nil {
		return err
	}
	return nil
}

func (dbs *databaseStorage) AddBatchItems(ids []string, values []string, userID int) error {
	// В документации pgx рекомендовано задавать в контексте ограничение по времени,
	// т.к. при большом количестве запросов в batch возможен deadlock
	batch := &pgx.Batch{}
	for i := 0; i < len(ids); i++ {
		batch.Queue("INSERT INTO convertions (short_url, orig_url) VALUES ($1, $2)", values[i], ids[i])
	}
	ctx, cancelFunc := context.WithTimeout(context.Background(), time.Second*2)
	defer cancelFunc()

	batchRes := dbs.pool.SendBatch(ctx, batch)
	defer batchRes.Close()

	// Не совсем понял, нужно ли дополнять историю юзера в этом случае. Пока не делаю, т.к. некоторые вещи в этом не очевидны

	return nil
}

func (dbs *databaseStorage) addItemUserHistory(id string, value string, userID int) error {
	history, err := dbs.GetUserHistory(userID)
	if err != nil && err.Error() != "not found" {
		fmt.Println(err.Error())
		return err
	}
	if len(history) > 0 {
		found := false
		for _, historyElem := range history {
			if historyElem.ShortURL == value && historyElem.OrigURL == id {
				found = true
				break
			}
		}
		if found {
			return nil
		}
		history = append(history, URLConversion{value, id})
		if _, err := dbs.pool.Exec(context.Background(),
			"UPDATE histories SET history = $1 WHERE user_id = $2", history, userID); err != nil {
			return err
		}
		return nil
	}
	history = append(history, URLConversion{value, id})
	if _, err := dbs.pool.Exec(context.Background(),
		"INSERT INTO histories (user_id, history) VALUES ($1, $2)", userID, history); err != nil {
		return err
	}
	return nil
}

func (dbs *databaseStorage) GetItem(value string) (string, error) {
	var origURL string
	err := dbs.pool.QueryRow(context.Background(), "SELECT orig_url FROM convertions WHERE short_url = $1", value).Scan(&origURL)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", errors.New("not found")
		}
		return "", err
	}
	return origURL, nil
}

func (dbs *databaseStorage) GetItemByID(ID string) (string, error) {
	var shortURL string
	err := dbs.pool.QueryRow(context.Background(), "SELECT short_url FROM convertions WHERE orig_url = $1", ID).Scan(&shortURL)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", errors.New("not found")
		}
		return "", err
	}
	return shortURL, nil
}

func (dbs *databaseStorage) GetUserHistory(userID int) (History, error) {
	var history History
	err := dbs.pool.QueryRow(context.Background(), "SELECT history FROM histories WHERE user_id = $1", userID).Scan(&history)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return make(History, 0), errors.New("not found")
		}
		return make(History, 0), err
	}
	return history, nil
}

func (history History) Value() (driver.Value, error) {
	if len(history) == 0 {
		return "", nil
	}
	pairs := make([]string, len(history))
	for i := 0; i < len(history); i++ {
		pairs[i] = fmt.Sprintf("%s %s", history[i].ShortURL, history[i].OrigURL)
	}
	return strings.Join(pairs, "|"), nil
}

func (history *History) Scan(value interface{}) error {
	if value == nil {
		*history = History{}
		return nil
	}
	sv, err := driver.String.ConvertValue(value)
	if err != nil {
		return fmt.Errorf("cannot scan value. %w", err)
	}
	v, ok := sv.(string)
	if !ok {
		return errors.New("cannot scan value. cannot convert value to string")
	}
	textPairs := strings.Split(v, "|")
	for _, textPair := range textPairs {
		convertion := strings.Split(textPair, " ")
		*history = append(*history, URLConversion{convertion[0], convertion[1]})
	}
	return nil
}
