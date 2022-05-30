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
		http.Error(w, "invalid URL", http.StatusBadRequest)
		return
	}
	urlPath := r.URL.Path
	switch r.Method {
	case http.MethodGet:
		path := strings.TrimPrefix(r.URL.Path, "/")
		if strings.Contains(path, "/") {
			http.Error(w, "URL contains invalid symbol", http.StatusBadRequest)
		}
		for origUrl, shortUrl := range h.converter {
			if shortUrl == path {
				w.Header().Set("Location", origUrl)
				w.WriteHeader(307)
				//w.Write([]byte(origUrl))
				return
			}
		}
		http.Error(w, "Cannot find full URL for this short URL", http.StatusBadRequest)
		return
	case http.MethodPost:
		if urlPath != "/" {
			http.Error(w, "invalid URL", http.StatusBadRequest)
			return
		}
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		shortUrl := h.MakeShortURL(string(body))
		h.converter[string(body)] = shortUrl
		w.WriteHeader(201)
		outputFullShortURL := fmt.Sprintf("http://localhost:8080/%s", shortUrl)
		w.Write([]byte(outputFullShortURL))
	default:
		http.Error(w, "Unsupported HTTP method", http.StatusBadRequest)
		return
	}

}

func (h ShorterHandler) MakeShortURL(url string) string {
	crc := crc32.ChecksumIEEE([]byte(url))
	return fmt.Sprint(crc)
}

func main() {
	shorterHandler := ShorterHandler{make(map[string]string)}

	server := &http.Server{
		Handler: shorterHandler,
		Addr:    "localhost:8080",
	}
	server.ListenAndServe()
}
