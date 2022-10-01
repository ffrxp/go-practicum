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

var ErrURLDeleted = errors.New("app: URL deleted")
var ErrCantFindURL = errors.New("app: cannot find URL")

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
	itemRes, err := sa.Storage.GetItem(shortURL)
	if err != nil {
		if errors.Is(err, storage.ErrEmptyResult) {
			return "", ErrCantFindURL
		}
		return "", err
	}
	if itemRes.HaveDeletedFlag {
		return "", ErrURLDeleted
	}
	return itemRes.Item, nil
}

func (sa *ShortenerApp) GetExistShortURL(origURL string) (string, error) {
	itemRes, err := sa.Storage.GetItemByID(origURL)
	if err != nil {
		if errors.Is(err, storage.ErrEmptyResult) {
			return "", ErrCantFindURL
		}
		return "", err
	}
	if itemRes.HaveDeletedFlag {
		return "", ErrURLDeleted
	}
	outputFullShortURL := fmt.Sprintf("%s/%s", sa.BaseAddress, itemRes.Item)
	return outputFullShortURL, nil
}

func (sa *ShortenerApp) ShortURLExist(shortURL string) (bool, error) {
	// До конца не уверен, что использовать error в рамках штатной работы алгоритма является хорошей идеей,
	// но пока не успеваю обдумать другие варианты
	_, err := sa.Storage.GetItem(shortURL)
	if err != nil {
		if errors.Is(err, storage.ErrEmptyResult) {
			return false, nil
		}
		return false, err
	}
	return true, nil
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

func (sa *ShortenerApp) UserHaveURLinHistory(userID int, URL string) (bool, error) {
	history, err := sa.Storage.GetUserHistory(userID)
	if err != nil {
		if errors.Is(err, storage.ErrEmptyResult) {
			return false, nil
		}
		return false, err
	}
	for i := 0; i < len(history); i++ {
		if history[i].ShortURL == URL {
			return true, nil
		}
	}
	return false, nil
}

func (sa *ShortenerApp) UserHaveHistoryURLs(userID int) (bool, error) {
	history, err := sa.Storage.GetUserHistory(userID)
	if err != nil {
		if errors.Is(err, storage.ErrEmptyResult) {
			return false, nil
		}
		return len(history) != 0, err
	}

	return true, nil
}

func (sa *ShortenerApp) MarkDeleteBatchURLs(urls []string) error {
	err := sa.Storage.MarkDeleteBatchItems(urls)
	return err
}

func (sa *ShortenerApp) makeShortURL(url string) string {
	crc := crc32.ChecksumIEEE([]byte(url))
	return fmt.Sprint(crc)
}
