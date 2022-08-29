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
	GetItem(value string) (*ItemResult, error)
	GetItemByID(ID string) (*ItemResult, error)
	GetUserHistory(userID int) (History, error)
	MarkDeleteBatchItems(ids []string) error
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
	deletedURLs        map[string]bool
	sfm                *sourceFileManager
}

type History []URLConversion

// ItemResult returning results of query in storage
type ItemResult struct {
	Item            string
	HaveDeletedFlag bool
}

var ErrEmptyResult = errors.New("storage: empty result")

func NewDataStorage(source string) *dataStorage {
	if source == "" {
		return &dataStorage{make(map[int][]URLConversion),
			make(map[string]string),
			make(map[string]bool),
			nil}
	}
	file, err := os.OpenFile(source, os.O_RDWR|os.O_CREATE, 0777)
	if err != nil {
		log.Printf("Cannot open data file. Path:%s\n", source)
		return &dataStorage{make(map[int][]URLConversion),
			make(map[string]string),
			make(map[string]bool),
			nil}
	}
	sfm := sourceFileManager{
		file:    file,
		encoder: json.NewEncoder(file),
		decoder: json.NewDecoder(file)}
	ds := dataStorage{make(map[int][]URLConversion),
		map[string]string{},
		make(map[string]bool),
		&sfm}
	if err := ds.loadItems(); err != nil {
		return &ds
	}
	return &ds
}

func (ms *dataStorage) AddItem(id string, value string, userID int) error {
	log.Printf("Add item to storage. Short URL:%s|Original URL:%s|User ID:%d\n", value, id, userID)
	if _, ok := ms.storage[id]; ok {
		log.Println("Result: conflict. Item already exist")
		return errors.New("already exists")
	}
	ms.storage[id] = value
	ms.deletedURLs[id] = false
	if ms.sfm != nil {
		if err := ms.sfm.file.Truncate(0); err != nil {
			log.Printf("Error processing \"Truncate\". Error message:%s", err.Error())
			return err
		}
		if _, err := ms.sfm.file.Seek(0, 0); err != nil {
			log.Printf("Error processing \"Seek\". Error message:%s", err.Error())
			return err
		}
		if err := ms.sfm.encoder.Encode(&ms.storage); err != nil {
			log.Printf("Error processing \"Encode\" URLs convertions. Error message:%s", err.Error())
			return err
		}
		if err := ms.sfm.encoder.Encode(&ms.deletedURLs); err != nil {
			log.Printf("Error processing \"Encode\" deleted URLs. Error message:%s", err.Error())
			return err
		}
	}
	ms.addItemUserHistory(id, value, userID)
	return nil
}

func (ms *dataStorage) AddBatchItems(ids []string, values []string, userID int) error {
	log.Printf("Add batch items to storage.\n")
	if len(ids) != len(values) {
		err := errors.New("number of id and values is not equal")
		log.Printf("Error adding batch items. Error message:%s\n", err.Error())
		return err
	}
	for i := 0; i < len(ids); i++ {
		err := ms.AddItem(ids[i], values[i], userID)
		if err != nil {
			return err
		}
	}
	return nil
}

func (ms *dataStorage) MarkDeleteBatchItems(ids []string) error {
	log.Printf("Mark delete batch items in storage.\n")
	for _, id := range ids {
		ms.deletedURLs[id] = true
	}
	return nil
}

func (ms *dataStorage) GetItem(value string) (*ItemResult, error) {
	log.Printf("Get original URL by short URL. Short URL:%s\n", value)
	for key, val := range ms.storage {
		if val == value {
			return &ItemResult{key, ms.deletedURLs[key]}, nil
		}
	}
	err := ErrEmptyResult
	log.Printf("Item not found. Error message:%s\n", err.Error())
	return nil, err
}

func (ms *dataStorage) GetItemByID(ID string) (*ItemResult, error) {
	log.Printf("Get short URL by original URL. Original URL:%s\n", ID)
	for key, val := range ms.storage {
		if key == ID {
			return &ItemResult{val, ms.deletedURLs[key]}, nil
		}
	}
	err := ErrEmptyResult
	log.Printf("Item not found. Error message:%s\n", err.Error())
	return nil, err
}

func (ms *dataStorage) loadItems() error {
	log.Printf("Loading storage items\n")
	if ms.sfm == nil {
		return nil
	}
	if err := ms.sfm.decoder.Decode(&ms.storage); err != nil {
		log.Printf("Error loading items from storage. Error message:%s\n", err.Error())
		return err
	}
	if err := ms.sfm.decoder.Decode(&ms.deletedURLs); err != nil {
		log.Printf("Error loading items from storage 11111. Error message:%s\n", err.Error())
		return err
	}
	return nil
}

func (ms *dataStorage) addItemUserHistory(id string, value string, userID int) {
	log.Printf("Add item to user history. Short URL:%s|Original URL:%s|User ID:%d\n", value, id, userID)
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
	log.Printf("Get user history. User ID:%d\n", userID)
	history, ok := ms.userHistoryStorage[userID]
	if !ok {
		return make(History, 0), ErrEmptyResult
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
		"(short_url character varying(2048) NOT NULL PRIMARY KEY, orig_url character varying(2048) NOT NULL, deleted boolean)"
	if _, err := dbpool.Exec(context.Background(), queryCreateConv); err != nil {
		log.Printf("Cannot create convertions table")
		return nil, err
	}
	queryCreateHistories := "CREATE TABLE IF NOT EXISTS histories " +
		"(user_id integer NOT NULL PRIMARY KEY, history text NOT NULL)"
	if _, err := dbpool.Exec(context.Background(), queryCreateHistories); err != nil {
		log.Printf("Cannot create histories table")
		return nil, err
	}
	return &databaseStorage{dbpool}, nil
}

func (dbs *databaseStorage) Close() error {
	dbs.pool.Close()
	return nil
}

func (dbs *databaseStorage) AddItem(id string, value string, userID int) error {
	// Я рассматривал вариант, чтобы сделать ON CONFLICT DO UPDATE, но мне показалось,
	// что логика будет менее очевидной. В итоге остановился на текущем варианте,
	// тем более что на выбор предлагались оба варианта.
	log.Printf("Add item to database. Short URL:%s|Original URL:%s|User ID:%d\n", value, id, userID)
	if _, err := dbs.pool.Exec(context.Background(),
		"INSERT INTO convertions (short_url, orig_url, deleted) VALUES ($1, $2, $3)", value, id, false); err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			if pgErr.Code == pgerrcode.UniqueViolation {
				log.Println("Result: conflict. Item already exist")
				return errors.New("already exists")
			}
			log.Printf("Result: error. Error message:%s\n", err.Error())
			return err
		}
		log.Printf("Result: error. Error message:%s\n", err.Error())
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
	log.Printf("Add batch items to database.\n")
	for i := 0; i < len(ids); i++ {
		batch.Queue("INSERT INTO convertions (short_url, orig_url, deleted) VALUES ($1, $2, $3)", values[i], ids[i], false)
	}
	ctx, cancelFunc := context.WithTimeout(context.Background(), time.Second*2)
	defer cancelFunc()

	batchRes := dbs.pool.SendBatch(ctx, batch)
	defer batchRes.Close()

	// Не совсем понял, нужно ли дополнять историю юзера в этом случае. Пока не делаю, т.к. некоторые вещи в этом не очевидны

	return nil
}

func (dbs *databaseStorage) MarkDeleteBatchItems(ids []string) error {
	batch := &pgx.Batch{}
	log.Printf("Mark delete batch items in database: %s.\n", ids)
	for i := 0; i < len(ids); i++ {
		batch.Queue("UPDATE convertions SET deleted = $1 WHERE short_url = $2", true, ids[i])
	}
	ctx, cancelFunc := context.WithTimeout(context.Background(), time.Second*2)
	defer cancelFunc()

	batchRes := dbs.pool.SendBatch(ctx, batch)
	defer batchRes.Close()
	return nil
}

func (dbs *databaseStorage) addItemUserHistory(id string, value string, userID int) error {
	log.Printf("Add item to user history. Short URL:%s|Original URL:%s|User ID:%d\n", value, id, userID)
	history, err := dbs.GetUserHistory(userID)
	if err != nil && !errors.Is(err, ErrEmptyResult) {
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
			log.Printf("Exec update query error. Error message:%s\n", err.Error())
			return err
		}
		return nil
	}
	history = append(history, URLConversion{value, id})
	if _, err := dbs.pool.Exec(context.Background(),
		"INSERT INTO histories (user_id, history) VALUES ($1, $2)", userID, history); err != nil {
		log.Printf("Exec insert query error. Error message:%s\n", err.Error())
		return err
	}
	return nil
}

func (dbs *databaseStorage) GetItem(value string) (*ItemResult, error) {
	var origURL string
	var deleted bool
	log.Printf("Get original URL by short URL. Short URL:%s\n", value)
	err := dbs.pool.QueryRow(context.Background(), "SELECT orig_url, deleted FROM convertions WHERE short_url = $1", value).Scan(&origURL, &deleted)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			log.Printf("Exec select query error. Error message:%s\n", err.Error())
			return nil, ErrEmptyResult
		}
		log.Printf("Exec select query error. Error message:%s\n", err.Error())
		return nil, err
	}
	return &ItemResult{origURL, deleted}, nil
}

func (dbs *databaseStorage) GetItemByID(ID string) (*ItemResult, error) {
	var shortURL string
	var deleted bool
	log.Printf("Get short URL by original URL. Original URL:%s\n", ID)
	err := dbs.pool.QueryRow(context.Background(), "SELECT short_url, deleted FROM convertions WHERE orig_url = $1", ID).Scan(&shortURL, &deleted)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			log.Printf("Exec select query error. Error message:%s\n", err.Error())
			return nil, ErrEmptyResult
		}
		log.Printf("Exec select query error. Error message:%s\n", err.Error())
		return nil, err
	}
	return &ItemResult{shortURL, deleted}, nil
}

func (dbs *databaseStorage) GetUserHistory(userID int) (History, error) {
	var history History
	log.Printf("Get user history. User ID:%d\n", userID)
	err := dbs.pool.QueryRow(context.Background(), "SELECT history FROM histories WHERE user_id = $1", userID).Scan(&history)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			log.Printf("Exec select query error. Error message:%s\n", err.Error())
			return make(History, 0), ErrEmptyResult
		}
		log.Printf("Exec select query error. Error message:%s\n", err.Error())
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
		log.Printf("Cannot scan value. Error message:%s\n", err.Error())
		return fmt.Errorf("cannot scan value. %w", err)
	}
	v, ok := sv.(string)
	if !ok {
		err := errors.New("cannot scan value. cannot convert value to string")
		log.Printf("Cannot scan value. Cannot convert value to string. Error message:%s\n", err.Error())
		return err
	}
	textPairs := strings.Split(v, "|")
	for _, textPair := range textPairs {
		convertion := strings.Split(textPair, " ")
		*history = append(*history, URLConversion{convertion[0], convertion[1]})
	}
	return nil
}
