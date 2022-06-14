package main

import (
	"encoding/json"
	"errors"
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

type memStorage struct {
	storage map[string]string
}

func (ms *memStorage) addItem(id, value string) error {
	ms.storage[id] = value
	return nil
}

func (ms *memStorage) getItem(value string) (string, error) {
	for key, val := range ms.storage {
		if val == value {
			return key, nil
		}
	}
	return "", errors.New("not found")
}

type shortenerApp struct {
	storage repository
}

// Create short URL and return it in full version
func (sa *shortenerApp) createShortURL(url string) (string, error) {
	shortURL := sa.makeShortURL(url)
	err := sa.storage.addItem(url, shortURL)
	if err != nil {
		return "", err
	}
	outputFullShortURL := fmt.Sprintf("%s%s", GetBaseAddress(), shortURL)
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
const DefaultBaseAddress = "http://localhost:8080/"

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

var converter map[string]string = make(map[string]string)

func main() {
	fmt.Println(GetServerAddress())
	sa := shortenerApp{storage: &memStorage{converter}}
	log.Fatal(http.ListenAndServe(GetServerAddress(), newShortenerHandler(&sa)))
}
