package main

import (
	"fmt"
	"hash/crc32"
	"io"
	"net/http"
	"strings"
)

type ShorterHandler struct {
	converter map[string]string
}

func (h ShorterHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if len(r.URL.Query()) > 0 {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	url_path := r.URL.Path
	switch r.Method {
	case http.MethodGet:
		path := strings.TrimPrefix(r.URL.Path, "/")

		for origUrl, shortUrl := range h.converter {
			if shortUrl == path {
				w.Header().Set("location", origUrl)
				w.WriteHeader(307)
				return
			}
		}
		w.WriteHeader(http.StatusBadRequest)
		return
	case http.MethodPost:
		if url_path != "/" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		body, err := io.ReadAll(r.Body)
		if err != nil {
			//w.WriteHeader(http.StatusBadRequest)
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		shortUrl := h.MakeShortURL(string(body))
		h.converter[string(body)] = shortUrl
		w.WriteHeader(201)
		w.Write([]byte(shortUrl))
	default:
		w.WriteHeader(http.StatusBadRequest)
		return
	}

}

func (h ShorterHandler) MakeShortURL(url string) string {
	crc := crc32.ChecksumIEEE([]byte(url))
	return fmt.Sprint(crc)
}

func main() {
	shorterHandler := ShorterHandler{make(map[string]string)}
	testUrl := "ya.ru"
	shorterHandler.converter[testUrl] = shorterHandler.MakeShortURL(testUrl)

	server := &http.Server{
		Handler: shorterHandler,
		Addr:    "localhost:8080",
	}
	server.ListenAndServe()
}
