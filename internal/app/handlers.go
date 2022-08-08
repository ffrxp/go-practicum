package app

import (
	"compress/gzip"
	"crypto/hmac"
	"encoding/json"
	"errors"
	"github.com/go-chi/chi/v5"
	"io"
	"log"
	"math/rand"
	"net/http"
	"net/url"
	"strings"
)

type shortenerHandler struct {
	*chi.Mux
	app    *ShortenerApp
	secKey []byte
}

func NewShortenerHandler(sa *ShortenerApp) *shortenerHandler {
	h := &shortenerHandler{
		Mux: chi.NewMux(),
		app: sa,
	}
	h.Post("/", h.middlewareUnpacker(h.postURLCommon()))
	h.Post("/api/shorten", h.middlewareUnpacker(h.postURLByJSON()))
	h.Mux.NotFound(h.badRequest())
	h.Mux.MethodNotAllowed(h.badRequest())
	h.Get("/{shortURL}", h.middlewareUnpacker(h.getURL()))
	h.Get("/api/user/urls", h.middlewareUnpacker(h.returnUserURLs()))
	h.secKey = []byte("some_secret_key")
	return h
}

type gzipWriter struct {
	http.ResponseWriter
	Writer io.Writer
}

func (w gzipWriter) Write(b []byte) (int, error) {
	return w.Writer.Write(b)
}

type CookieData struct {
	UserID int
	Token  []byte
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

func (h *shortenerHandler) postURLCommon() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := rand.Int()

		// Process cookies
		cookieName := "token"
		userCookie, err := r.Cookie(cookieName)
		if err != nil {
			if !errors.Is(err, http.ErrNoCookie) {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			userCookie, err = h.createCookie(cookieName, userID)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
		} else {
			// Checking sign of cookie
			curCookieValue := CookieData{}
			cookieValueUnescaped, err := url.QueryUnescape(userCookie.Value)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			err = json.Unmarshal([]byte(cookieValueUnescaped), &curCookieValue)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			expectedToken := GetUserToken(curCookieValue.UserID)
			signedExpectedToken := SignMsg([]byte(expectedToken), h.secKey)
			if hmac.Equal(curCookieValue.Token, signedExpectedToken) {
				userID = curCookieValue.UserID
			} else {
				userCookie, err = h.createCookie(cookieName, userID)
				if err != nil {
					http.Error(w, err.Error(), http.StatusInternalServerError)
					return
				}
			}
		}

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

		shortURL, errCreating := h.app.createShortURL(string(body), userID)
		if errCreating != nil {
			http.Error(w, errCreating.Error(), http.StatusBadRequest)
			return
		}

		http.SetCookie(w, userCookie)
		w.WriteHeader(201)
		_, errWrite := w.Write([]byte(shortURL))
		if errWrite != nil {
			log.Printf("Writting error")
			return
		}
	}
}

func (h *shortenerHandler) postURLByJSON() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := rand.Int()

		// Process cookies
		cookieName := "token"
		userCookie, err := r.Cookie(cookieName)
		if err != nil {
			if !errors.Is(err, http.ErrNoCookie) {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}

			userCookie, err = h.createCookie(cookieName, userID)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
		} else {
			// Checking sign of cookie
			curCookieValue := CookieData{}
			cookieValueUnescaped, err := url.QueryUnescape(userCookie.Value)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			err = json.Unmarshal([]byte(cookieValueUnescaped), &curCookieValue)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			expectedToken := GetUserToken(curCookieValue.UserID)
			signedExpectedToken := SignMsg([]byte(expectedToken), h.secKey)
			if hmac.Equal(curCookieValue.Token, signedExpectedToken) {
				userID = curCookieValue.UserID
			} else {
				userCookie, err = h.createCookie(cookieName, userID)
				if err != nil {
					http.Error(w, err.Error(), http.StatusInternalServerError)
					return
				}
			}
		}

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
		shortURL, errCreating := h.app.createShortURL(requestParsedBody.URL, userID)
		if errCreating != nil {
			http.Error(w, errCreating.Error(), http.StatusBadRequest)
			return
		}

		resultRespBody := struct {
			Result string `json:"result"`
		}{Result: shortURL}
		resp, err := json.Marshal(resultRespBody)
		if err != nil {
			http.Error(w, "Cannot marshal JSON response", http.StatusInternalServerError)
			return
		}

		http.SetCookie(w, userCookie)
		w.Header().Set("content-type", "application/json")
		w.WriteHeader(201)
		_, errWrite := w.Write(resp)
		if errWrite != nil {
			log.Printf("Writting error")
			return
		}
	}
}

func (h *shortenerHandler) getURL() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := rand.Int()

		// Process cookies
		cookieName := "token"
		userCookie, err := r.Cookie(cookieName)
		if err != nil {
			if !errors.Is(err, http.ErrNoCookie) {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}

			userCookie, err = h.createCookie(cookieName, userID)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
		} else {
			// Checking sign of cookie
			curCookieValue := CookieData{}
			cookieValueUnescaped, err := url.QueryUnescape(userCookie.Value)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			err = json.Unmarshal([]byte(cookieValueUnescaped), &curCookieValue)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			expectedToken := GetUserToken(curCookieValue.UserID)
			signedExpectedToken := SignMsg([]byte(expectedToken), h.secKey)
			if !hmac.Equal(curCookieValue.Token, signedExpectedToken) {
				userCookie, err = h.createCookie(cookieName, userID)
				if err != nil {
					http.Error(w, err.Error(), http.StatusInternalServerError)
					return
				}
			}
		}

		paramURL := chi.URLParam(r, "shortURL")
		if strings.Contains(paramURL, "/") {
			http.Error(w, "URL contains invalid symbol", http.StatusBadRequest)
		}
		origURL, err := h.app.getOrigURL(paramURL)
		if err != nil {
			http.Error(w, "Cannot find full URL for this short URL", http.StatusBadRequest)
			return
		}
		http.SetCookie(w, userCookie)
		w.Header().Set("Location", origURL)
		w.WriteHeader(307)
	}
}

func (h *shortenerHandler) returnUserURLs() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := rand.Int()

		// Process cookies
		cookieName := "token"
		userCookie, err := r.Cookie(cookieName)
		if err != nil {
			if !errors.Is(err, http.ErrNoCookie) {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			userCookie, err = h.createCookie(cookieName, userID)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
		} else {
			// Checking sign of cookie
			curCookieValue := CookieData{}
			cookieValueUnescaped, err := url.QueryUnescape(userCookie.Value)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			err = json.Unmarshal([]byte(cookieValueUnescaped), &curCookieValue)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			expectedToken := GetUserToken(curCookieValue.UserID)
			signedExpectedToken := SignMsg([]byte(expectedToken), h.secKey)

			if hmac.Equal(curCookieValue.Token, signedExpectedToken) {
				userID = curCookieValue.UserID
			} else {
				userCookie, err = h.createCookie(cookieName, userID)
				if err != nil {
					http.Error(w, err.Error(), http.StatusInternalServerError)
					return
				}
			}
		}
		if !h.app.userHaveHistoryURLs(userID) {
			http.SetCookie(w, userCookie)
			w.WriteHeader(204)
			return
		}
		history, err := h.app.getHistoryURLsForUser(userID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		http.SetCookie(w, userCookie)
		w.Header().Set("content-type", "application/json")
		w.WriteHeader(200)
		_, errWrite := w.Write(history)
		if errWrite != nil {
			log.Printf("Writting error")
			return
		}
	}
}

func (h *shortenerHandler) badRequest() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := rand.Int()

		// Process cookies
		cookieName := "token"
		userCookie, err := r.Cookie(cookieName)
		if err != nil {
			if !errors.Is(err, http.ErrNoCookie) {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}

			userCookie, err = h.createCookie(cookieName, userID)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
		} else {
			// Checking sign of cookie
			curCookieValue := CookieData{}
			cookieValueUnescaped, err := url.QueryUnescape(userCookie.Value)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			err = json.Unmarshal([]byte(cookieValueUnescaped), &curCookieValue)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			expectedToken := GetUserToken(curCookieValue.UserID)
			signedExpectedToken := SignMsg([]byte(expectedToken), h.secKey)
			if !hmac.Equal(curCookieValue.Token, signedExpectedToken) {
				userCookie, err = h.createCookie(cookieName, userID)
				if err != nil {
					http.Error(w, err.Error(), http.StatusInternalServerError)
					return
				}
			}
		}
		http.SetCookie(w, userCookie)
		w.WriteHeader(400)
	}
}

func (h *shortenerHandler) createCookie(cookieName string, userID int) (*http.Cookie, error) {
	token := GetUserToken(userID)
	signedToken := SignMsg([]byte(token), h.secKey)

	JSONCookieBody, err := json.Marshal(CookieData{userID, signedToken})
	if err != nil {
		return nil, err
	}
	userCookie := &http.Cookie{
		Name:   cookieName,
		Value:  url.QueryEscape(string(JSONCookieBody)),
		MaxAge: 1200,
	}

	return userCookie, nil
}
