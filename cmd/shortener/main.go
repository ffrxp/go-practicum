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
	for origUrl, shortUrl := range converter {
		if shortUrl == paramURL {
			w.Header().Set("Location", origUrl)
			w.WriteHeader(307)
			return
		}
	}
	http.Error(w, "Cannot find full URL for this short URL", http.StatusBadRequest)
	return
}

func postURL(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	shortUrl := MakeShortURL(string(body))
	converter[string(body)] = shortUrl
	w.WriteHeader(201)
	outputFullShortURL := fmt.Sprintf("http://localhost:8080/%s", shortUrl)
	w.Write([]byte(outputFullShortURL))
}

func badRequest(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(400)
	return
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
