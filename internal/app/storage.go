package app

import (
	"encoding/json"
	"errors"
	"log"
	"os"
)

type repository interface {
	addItem(id, value, userID string) error
	getItem(value string) (string, error)
	getUserHistory(userID string) []URLConversion
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
	userHistoryStorage map[string][]URLConversion
	storage            map[string]string
	sfm                *sourceFileManager
}

func NewDataStorage(source string) *dataStorage {
	if source == "" {
		return &dataStorage{make(map[string][]URLConversion), make(map[string]string), nil}
	}
	file, err := os.OpenFile(source, os.O_RDWR|os.O_CREATE, 0777)
	if err != nil {
		log.Printf("Cannot open data file")
		return &dataStorage{make(map[string][]URLConversion), make(map[string]string), nil}
	}
	sfm := sourceFileManager{
		file:    file,
		encoder: json.NewEncoder(file),
		decoder: json.NewDecoder(file)}
	ds := dataStorage{make(map[string][]URLConversion), map[string]string{}, &sfm}
	if err := ds.loadItems(); err != nil {
		return &ds
	}
	return &ds
}

func (ms *dataStorage) addItem(id, value, userID string) error {
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
	if userID != "" {
		ms.addItemUserHistory(id, value, userID)
	}
	return nil
}

func (ms *dataStorage) getItem(value string) (string, error) {
	for key, val := range ms.storage {
		if val == value {
			return key, nil
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

func (ms *dataStorage) addItemUserHistory(id, value, userID string) {
	history, ok := ms.userHistoryStorage[userID]
	if ok {
		found := false
		for i, _ := range history {
			if history[i].ShortURL == id && history[i].OrigURL == value {
				found = true
				break
			}
		}
		if found {
			return
		}
		URLConv := URLConversion{id, value}
		ms.userHistoryStorage[userID] = append(ms.userHistoryStorage[userID], URLConv)
	}
	ms.userHistoryStorage[userID] = make([]URLConversion, 0)
	ms.userHistoryStorage[userID] = append(ms.userHistoryStorage[userID], URLConversion{id, value})
}

func (ms *dataStorage) getUserHistory(userID string) []URLConversion {
	history, ok := ms.userHistoryStorage[userID]
	if !ok {
		return make([]URLConversion, 0)
	}
	return history
}

func (ms *dataStorage) Close() error {
	if ms.sfm != nil {
		return ms.sfm.file.Close()
	}
	return nil
}
