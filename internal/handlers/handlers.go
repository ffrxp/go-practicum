package app

import (
	"compress/gzip"
	"encoding/json"
	"github.com/go-chi/chi/v5"
	"io"
	"log"
	"net/http"
	"strings"
)

type shortenerHandler struct {
	*chi.Mux
	app *ShortenerApp
}

func NewShortenerHandler(sa *ShortenerApp) *shortenerHandler {
	h := &shortenerHandler{
		Mux: chi.NewMux(),
		app: sa,
	}
	h.Post("/", h.middlewareUnpacker(h.postURL()))
	h.Post("/api/shorten", h.middlewareUnpacker(h.postURL()))
	h.Mux.NotFound(h.badRequest())
	h.Mux.MethodNotAllowed(h.badRequest())
	h.Get("/{shortURL}", h.middlewareUnpacker(h.getURL()))

	return h
}

type gzipWriter struct {
	http.ResponseWriter
	Writer io.Writer
}

func (w gzipWriter) Write(b []byte) (int, error) {
	return w.Writer.Write(b)
}

func (h *shortenerHandler) middlewareUnpacker(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") {
			next(w, r)
			return
		}

		gz, err := gzip.NewWriterLevel(w, gzip.BestSpeed)
		if err != nil {
			io.WriteString(w, err.Error())
			return
		}
		defer gz.Close()

		w.Header().Set("Content-Encoding", "gzip")
		next(gzipWriter{ResponseWriter: w, Writer: gz}, r)
	}
}

func (h *shortenerHandler) postURL() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var reader io.Reader
		if r.Header.Get(`Content-Encoding`) == `gzip` {
			gz, err := gzip.NewReader(r.Body)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			reader = gz
			defer gz.Close()
		} else {
			reader = r.Body
		}
		body, err := io.ReadAll(reader)
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
