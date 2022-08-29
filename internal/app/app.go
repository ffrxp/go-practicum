package app

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/ffrxp/go-practicum/internal/storage"
	"hash/crc32"
)

type ShortenerApp struct {
	Storage      storage.Repository
	BaseAddress  string
	DatabasePath string
}

// CreateShortURL creates short URL and return it in full version
func (sa *ShortenerApp) CreateShortURL(url string, userID int) (string, error) {
	shortURL := sa.makeShortURL(url)
	err := sa.Storage.AddItem(url, shortURL, userID)
	if err != nil {
		return "", err
	}
	outputFullShortURL := fmt.Sprintf("%s/%s", sa.BaseAddress, shortURL)
	return outputFullShortURL, nil
}

// CreateShortURLs creates short URLs for batch of URLs. Short URLs will return in same sequence
func (sa *ShortenerApp) CreateShortURLs(urls []string, userID int) ([]string, error) {
	var shortURLs []string
	for _, URL := range urls {
		shortURLs = append(shortURLs, sa.makeShortURL(URL))
	}
	err := sa.Storage.AddBatchItems(urls, shortURLs, userID)
	if err != nil {
		return make([]string, 0), err
	}
	var fullShortURLs []string
	for _, shortURL := range shortURLs {
		fullShortURLs = append(fullShortURLs, fmt.Sprintf("%s/%s", sa.BaseAddress, shortURL))
	}
	return fullShortURLs, nil
}

func (sa *ShortenerApp) GetOrigURL(shortURL string) (string, error) {
	origURL, err := sa.Storage.GetItem(shortURL)
	if err != nil {
		if err.Error() == "not found" {
			return "", errors.New("cannot find full URL for this short URL")
		}
		return "", err
	}
	return origURL, nil
}

func (sa *ShortenerApp) GetExistShortURL(origURL string) (string, error) {
	shortURL, err := sa.Storage.GetItemByID(origURL)
	if err != nil {
		if err.Error() == "not found" {
			return "", errors.New("cannot find short URL for this full URL")
		}
		return "", err
	}
	outputFullShortURL := fmt.Sprintf("%s/%s", sa.BaseAddress, shortURL)
	return outputFullShortURL, nil
}

func (sa *ShortenerApp) GetHistoryURLsForUser(userID int) ([]byte, error) {
	history, err := sa.Storage.GetUserHistory(userID)
	if err != nil {
		return make([]byte, 0), err
	}
	for i := 0; i < len(history); i++ {
		FullShortURL := fmt.Sprintf("%s/%s", sa.BaseAddress, history[i].ShortURL)
		history[i].ShortURL = FullShortURL
	}
	historyByJSON, err := json.Marshal(history)
	if err != nil {
		return make([]byte, 0), err
	}
	return historyByJSON, nil
}

func (sa *ShortenerApp) UserHaveHistoryURLs(userID int) (bool, error) {
	history, err := sa.Storage.GetUserHistory(userID)
	return len(history) != 0, err
}

func (sa *ShortenerApp) makeShortURL(url string) string {
	crc := crc32.ChecksumIEEE([]byte(url))
	return fmt.Sprint(crc)
}
