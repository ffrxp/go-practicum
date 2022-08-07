package app

import (
	"encoding/json"
	"errors"
	"fmt"
	"hash/crc32"
)

type ShortenerApp struct {
	Storage     repository
	BaseAddress string
}

// Create short URL and return it in full version
func (sa *ShortenerApp) createShortURL(url, userID string) (string, error) {
	shortURL := sa.makeShortURL(url)
	err := sa.Storage.addItem(url, shortURL, userID)
	if err != nil {
		return "", err
	}
	outputFullShortURL := fmt.Sprintf("%s/%s", sa.BaseAddress, shortURL)
	return outputFullShortURL, nil
}

func (sa *ShortenerApp) getOrigURL(shortURL string) (string, error) {
	origURL, err := sa.Storage.getItem(shortURL)
	if err != nil {
		if err.Error() == "not found" {
			return "", errors.New("cannot find full URL for this short URL")
		}
		return "", err
	}
	return origURL, nil
}

func (sa *ShortenerApp) getHistoryURLsForUser(userID string) ([]byte, error) {
	history := sa.Storage.getUserHistory(userID)
	historyByJSON, err := json.Marshal(history)
	if err != nil {
		return make([]byte, 0), err
	}
	return historyByJSON, nil
}

func (sa *ShortenerApp) userHaveHistoryURLs(userID string) bool {
	history := sa.Storage.getUserHistory(userID)
	return len(history) != 0
}

func (sa *ShortenerApp) makeShortURL(url string) string {
	crc := crc32.ChecksumIEEE([]byte(url))
	return fmt.Sprint(crc)
}
