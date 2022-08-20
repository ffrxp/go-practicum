package app

import (
	"encoding/json"
	"errors"
	"log"
	"os"
)

type repository interface {
	addItem(id, value string) error
	getItem(value string) (string, error)
}

type sourceFileManager struct {
	file    *os.File
	encoder *json.Encoder
	decoder *json.Decoder
}

type dataStorage struct {
	storage map[string]string
	sfm     *sourceFileManager
}

func NewDataStorage(source string) *dataStorage {
	if source == "" {
		return &dataStorage{make(map[string]string), nil}
	} else {
		file, err := os.OpenFile(source, os.O_RDWR|os.O_CREATE, 0777)
		if err != nil {
			log.Printf("Cannot open data file")
			return &dataStorage{make(map[string]string), nil}
		}
		sfm := sourceFileManager{
			file:    file,
			encoder: json.NewEncoder(file),
			decoder: json.NewDecoder(file)}
		ds := dataStorage{map[string]string{}, &sfm}
		if err := ds.loadItems(); err != nil {
			return &ds
		}
		return &ds
	}
}

func (ms *dataStorage) addItem(id, value string) error {
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

func (ms *dataStorage) Close() error {
	if ms.sfm != nil {
		return ms.sfm.file.Close()
	}
	return nil
}
