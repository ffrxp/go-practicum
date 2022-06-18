package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"github.com/go-chi/chi/v5"
	"hash/crc32"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
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

func newDataStorage(source string) *dataStorage {
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

func (ms *dataStorage) close() error {
	if ms.sfm != nil {
		return ms.sfm.file.Close()
	}
	return nil
}

type shortenerApp struct {
	storage     repository
	baseAddress string
}

// Create short URL and return it in full version
func (sa *shortenerApp) createShortURL(url string) (string, error) {
	shortURL := sa.makeShortURL(url)
	err := sa.storage.addItem(url, shortURL)
	if err != nil {
		return "", err
	}
	outputFullShortURL := fmt.Sprintf("%s/%s", sa.baseAddress, shortURL)
	return outputFullShortURL, nil
}

func (sa *shortenerApp) getOrigURL(shortURL string) (string, error) {
	origURL, err := sa.storage.getItem(shortURL)
	if err != nil {
		if err.Error() == "not found" {
			return "", errors.New("cannot find full URL for this short URL")
		}
		return "", err
	}
	return origURL, nil
}

func (sa *shortenerApp) makeShortURL(url string) string {
	crc := crc32.ChecksumIEEE([]byte(url))
	return fmt.Sprint(crc)
}

/*
func (sa *shortenerApp) getBaseAddress() string {
	return sa.baseAddress
}
*/
type shortenerHandler struct {
	*chi.Mux
	app *shortenerApp
}

func newShortenerHandler(sa *shortenerApp) *shortenerHandler {
	h := &shortenerHandler{
		Mux: chi.NewMux(),
		app: sa,
	}
	h.Post("/", h.postURL())
	h.Post("/api/shorten", h.postURL())
	// TODO: move checking bad endpoints to handlers?
	h.Post("/{.+}", h.badRequest())
	h.Get("//", h.badRequest())
	h.Get("//*", h.badRequest())
	h.Get("//{}/*", h.badRequest())
	h.Get("/{}/*", h.badRequest())
	h.Get("/{shortURL}", h.getURL())

	return h
}

func (h *shortenerHandler) postURL() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		defer r.Body.Close()

		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if r.URL.String() == "/api/shorten" {
			ct := r.Header.Get("content-type")
			if ct != "application/json" {
				http.Error(w, "Invalid content type of request", http.StatusBadRequest)
				return
			}
			requestParsedBody := struct {
				URL string `json:"url"`
			}{URL: ""}
			if err := json.Unmarshal(body, &requestParsedBody); err != nil {
				http.Error(w, "Cannot unmarshal JSON request", http.StatusBadRequest)
				return
			}
			shortURL, errCreating := h.app.createShortURL(requestParsedBody.URL)
			if errCreating != nil {
				http.Error(w, errCreating.Error(), http.StatusBadRequest)
				return
			}
			resultRespBody := struct {
				Result string `json:"result"`
			}{Result: shortURL}
			resp, err := json.Marshal(resultRespBody)
			if err != nil {
				http.Error(w, "Cannot marshal JSON response", http.StatusBadRequest)
				return
			}

			w.Header().Set("content-type", "application/json")
			w.WriteHeader(201)
			_, errWrite := w.Write(resp)
			if errWrite != nil {
				log.Printf("Writting error")
				return
			}
			return
		}

		shortURL, errCreating := h.app.createShortURL(string(body))
		if errCreating != nil {
			http.Error(w, errCreating.Error(), http.StatusBadRequest)
			return
		}
		w.WriteHeader(201)
		_, errWrite := w.Write([]byte(shortURL))
		if errWrite != nil {
			log.Printf("Writting error")
			return
		}
	}
}

func (h *shortenerHandler) getURL() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		paramURL := chi.URLParam(r, "shortURL")
		if strings.Contains(paramURL, "/") {
			http.Error(w, "URL contains invalid symbol", http.StatusBadRequest)
		}
		origURL, err := h.app.getOrigURL(paramURL)
		if err != nil {
			http.Error(w, "Cannot find full URL for this short URL", http.StatusBadRequest)
			return
		}
		w.Header().Set("Location", origURL)
		w.WriteHeader(307)
	}
}

func (h *shortenerHandler) badRequest() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(400)
	}
}

const DefaultServerAddress = ":8080"
const DefaultBaseAddress = "http://localhost:8080"

func GetServerAddress() string {
	val, ok := os.LookupEnv("SERVER_ADDRESS")
	if !ok || val == "" {
		return DefaultServerAddress
	}
	return val
}

func GetBaseAddress() string {
	val, ok := os.LookupEnv("BASE_URL")
	if !ok || val == "" {
		return DefaultBaseAddress
	}
	return val
}

func GetStoragePath() string {
	path, ok := os.LookupEnv("FILE_STORAGE_PATH")
	if !ok {
		return ""
	}
	return path
}

//var converter map[string]string = make(map[string]string)

func main() {
	serverAddress := flag.String("a", GetServerAddress(), "Start server address.")
	baseAddress := flag.String("b", GetBaseAddress(), "Base address for short URLs")
	storagePath := flag.String("f", GetStoragePath(), "Path for storage of short URLs")
	flag.Parse()

	storage := newDataStorage(*storagePath)
	defer storage.close()
	sa := shortenerApp{storage: storage, baseAddress: *baseAddress}
	log.Fatal(http.ListenAndServe(*serverAddress, newShortenerHandler(&sa)))
}
