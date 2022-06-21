package main

import (
	"fmt"
	"github.com/go-chi/chi/v5"
	"hash/crc32"
	"io"
	"log"
	"net/http"
	"strings"
)

var converter map[string]string = make(map[string]string)

func getURL(w http.ResponseWriter, r *http.Request) {
	paramURL := chi.URLParam(r, "shortURL")
	if strings.Contains(paramURL, "/") {
		http.Error(w, "URL contains invalid symbol", http.StatusBadRequest)
	}
	for origURL, shortURL := range converter {
		if shortURL == paramURL {
			w.Header().Set("Location", origURL)
			w.WriteHeader(307)
			return
		}
	}
	http.Error(w, "Cannot find full URL for this short URL", http.StatusBadRequest)
}

func postURL(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	shortURL := MakeShortURL(string(body))
	converter[string(body)] = shortURL
	w.WriteHeader(201)
	outputFullShortURL := fmt.Sprintf("http://localhost:8080/%s", shortURL)
	w.Write([]byte(outputFullShortURL))
}

func badRequest(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(400)
}

func MakeShortURL(url string) string {
	crc := crc32.ChecksumIEEE([]byte(url))
	return fmt.Sprint(crc)
}

func NewRouter() chi.Router {
	r := chi.NewRouter()
	r.Post("/", postURL)
	r.Post("/{.+}", badRequest)
	r.Get("//", badRequest)
	r.Get("//*", badRequest)
	r.Get("//{}/*", badRequest)
	r.Get("/{}/*", badRequest)
	r.Get("/{shortURL}", getURL)

	return r
}

func main() {
	log.Fatal(http.ListenAndServe(":8080", NewRouter()))
}
