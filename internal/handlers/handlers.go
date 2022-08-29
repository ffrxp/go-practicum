package handlers

import (
	"compress/gzip"
	"context"
	"crypto/hmac"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/ffrxp/go-practicum/internal/app"
	"github.com/ffrxp/go-practicum/internal/common"
	"github.com/ffrxp/go-practicum/internal/storage"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v4/pgxpool"
	"golang.org/x/sync/errgroup"
	"io"
	"log"
	"math/rand"
	"net/http"
	"net/url"
	"strings"
)

type shortenerHandler struct {
	*chi.Mux
	app    *app.ShortenerApp
	secKey []byte
}

func NewShortenerHandler(sa *app.ShortenerApp) shortenerHandler {
	h := shortenerHandler{
		Mux: chi.NewMux(),
		app: sa,
	}
	h.Post("/", h.middlewareGzipper(h.postURLCommon()))
	h.Post("/api/shorten", h.middlewareGzipper(h.postURLByJSON()))
	h.Post("/api/shorten/batch", h.middlewareGzipper(h.postURLBatch()))
	h.Mux.NotFound(h.badRequest())
	h.Mux.MethodNotAllowed(h.badRequest())
	h.Get("/{shortURL}", h.middlewareGzipper(h.getURL()))
	h.Get("/ping", h.middlewareGzipper(h.pingToDB()))
	h.Get("/api/user/urls", h.middlewareGzipper(h.returnUserURLs()))
	h.Delete("/api/user/urls", h.middlewareGzipper(h.deleteURLs()))

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

type processCookieResult struct {
	userID int
	cookie *http.Cookie
}

type BatchResponse []BatchResponseElem

type BatchAnswer []BatchAnswerElem

type BatchResponseElem struct {
	CorrelationID string `json:"correlation_id"`
	OriginalURL   string `json:"original_url"`
}

type BatchAnswerElem struct {
	CorrelationID string `json:"correlation_id"`
	ShortURL      string `json:"short_url"`
}

func (h *shortenerHandler) middlewareGzipper(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get(`Content-Encoding`) == `gzip` {
			gz, err := gzip.NewReader(r.Body)
			if err != nil {
				io.WriteString(w, err.Error())
				return
			}
			// Не уверен, что правильно использовать интерфейс с пустым вызовом Close(),
			// но пока не могу придумать других корректных вариантов
			r.Body = io.NopCloser(gz)
		}
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
		pcr, err := h.processCookies(r)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		body, err := io.ReadAll(r.Body)
		defer r.Body.Close()

		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		resultStatus := 201
		resultURL, errCreating := h.app.CreateShortURL(string(body), pcr.userID)
		if errCreating != nil {
			if errors.Is(errCreating, storage.ErrAlreadyExist) {
				resultURL, err = h.app.GetExistShortURL(string(body))
				if err != nil {
					http.Error(w, err.Error(), http.StatusBadRequest)
					return
				}
				resultStatus = 409
			} else {
				http.Error(w, errCreating.Error(), http.StatusBadRequest)
				return
			}
		}
		http.SetCookie(w, pcr.cookie)
		w.WriteHeader(resultStatus)
		_, errWrite := w.Write([]byte(resultURL))
		if errWrite != nil {
			log.Printf("Writting error")
			return
		}
	}
}

func (h *shortenerHandler) postURLByJSON() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		pcr, err := h.processCookies(r)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		body, err := io.ReadAll(r.Body)
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

		resultStatus := 201
		resultURL, errCreating := h.app.CreateShortURL(requestParsedBody.URL, pcr.userID)
		if errCreating != nil {
			if errors.Is(errCreating, storage.ErrAlreadyExist) {
				resultURL, err = h.app.GetExistShortURL(requestParsedBody.URL)
				if err != nil {
					http.Error(w, err.Error(), http.StatusBadRequest)
					return
				}
				resultStatus = 409
			} else {
				http.Error(w, errCreating.Error(), http.StatusBadRequest)
				return
			}
		}

		resultRespBody := struct {
			Result string `json:"result"`
		}{Result: resultURL}
		resp, err := json.Marshal(resultRespBody)
		if err != nil {
			http.Error(w, "Cannot marshal JSON response", http.StatusInternalServerError)
			return
		}

		http.SetCookie(w, pcr.cookie)
		w.Header().Set("content-type", "application/json")
		w.WriteHeader(resultStatus)
		_, errWrite := w.Write(resp)
		if errWrite != nil {
			log.Printf("Writting error")
			return
		}
	}
}

func (h *shortenerHandler) postURLBatch() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		pcr, err := h.processCookies(r)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		body, err := io.ReadAll(r.Body)
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

		var batchResp BatchResponse
		if err := json.Unmarshal(body, &batchResp); err != nil {
			http.Error(w, "Cannot unmarshal JSON request", http.StatusBadRequest)
			return
		}
		var urlsForShortener []string
		for _, respElem := range batchResp {
			urlsForShortener = append(urlsForShortener, respElem.OriginalURL)
		}
		shortURLs, errCreating := h.app.CreateShortURLs(urlsForShortener, pcr.userID)
		if errCreating != nil {
			http.Error(w, errCreating.Error(), http.StatusBadRequest)
			return
		}
		var batchAns BatchAnswer
		for i := 0; i < len(shortURLs); i++ {
			batchAns = append(batchAns, BatchAnswerElem{batchResp[i].CorrelationID, shortURLs[i]})
		}
		resp, err := json.Marshal(batchAns)
		if err != nil {
			http.Error(w, "Cannot marshal JSON response", http.StatusInternalServerError)
			return
		}

		http.SetCookie(w, pcr.cookie)
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
		// Нужно ли в этом обработчике создавать куки? На функционал они не повлияют, но юзера можно зафиксировать уже здесь

		paramURL := chi.URLParam(r, "shortURL")
		if strings.Contains(paramURL, "/") {
			http.Error(w, "URL contains invalid symbol", http.StatusBadRequest)
		}
		origURL, err := h.app.GetOrigURL(paramURL)
		if err != nil {
			if errors.Is(err, app.ErrURLDeleted) {
				w.WriteHeader(410)
				return
			} else if errors.Is(err, app.ErrCantFindURL) {
				http.Error(w, "Cannot find full URL for this short URL", http.StatusBadRequest)
				return
			} else {
				http.Error(w, fmt.Sprintf("Request error:%s", err.Error()), http.StatusBadRequest)
				return
			}
		}
		w.Header().Set("Location", origURL)
		w.WriteHeader(307)
	}
}

func (h *shortenerHandler) returnUserURLs() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		pcr, err := h.processCookies(r)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		userHaveHistoryURLs, err := h.app.UserHaveHistoryURLs(pcr.userID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if !userHaveHistoryURLs {
			http.SetCookie(w, pcr.cookie)
			w.WriteHeader(204)
			return
		}
		history, err := h.app.GetHistoryURLsForUser(pcr.userID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		http.SetCookie(w, pcr.cookie)
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
		// Нужно ли в этом обработчике создавать куки? На функционал они не повлияют, но юзера можно зафиксировать уже здесь
		w.WriteHeader(400)
	}
}

func (h *shortenerHandler) pingToDB() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Нужно ли в этом обработчике создавать куки? На функционал они не повлияют, но юзера можно зафиксировать уже здесь
		dbpool, err := pgxpool.Connect(context.Background(), h.app.DatabasePath)
		if err != nil {
			w.WriteHeader(500)
			return
		}
		defer dbpool.Close()
		w.WriteHeader(200)
	}
}

func (h *shortenerHandler) deleteURLs() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		pcr, err := h.processCookies(r)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		body, err := io.ReadAll(r.Body)
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

		var requestURLs []string
		if err := json.Unmarshal(body, &requestURLs); err != nil {
			http.Error(w, "Cannot unmarshal JSON request", http.StatusBadRequest)
			return
		}

		go h.processURLsForDelete(requestURLs, pcr.userID)
		w.WriteHeader(202)
	}
}

func (h *shortenerHandler) processURLsForDelete(URLs []string, userID int) {
	requestURLsNum := len(URLs)
	requestURLsChan := make(chan string, requestURLsNum)
	for _, URLForDel := range URLs {
		requestURLsChan <- URLForDel
	}
	delURLsChan := make(chan string)
	errChan := make(chan error)

	g := &errgroup.Group{}
	for i := 0; i < requestURLsNum; i++ {
		g.Go(func() error {
			URLForDel := <-requestURLsChan

			// Check if URL exist in DB and can be marked for delete by this user
			allowedForDel, err := h.app.UserHaveURLinHistory(userID, URLForDel)
			if err != nil {

				return fmt.Errorf("error in checking if URL belong to user: %w", err)
			}
			if !allowedForDel {
				return nil
			}
			URLExist, err := h.app.ShortURLExist(URLForDel)
			if err != nil {
				return fmt.Errorf("error in checking URL existing: %w", err)
			}
			if !URLExist {
				return nil
			}
			delURLsChan <- URLForDel
			return nil
		})
	}
	go func() {
		if err := g.Wait(); err != nil {
			log.Println(err)
			errChan <- err
			return
		}

		close(delURLsChan)
	}()

	var checkedURLsForDel []string
loop:
	for {
		select {
		case URLForDel, ok := <-delURLsChan:
			if !ok {
				break loop
			}
			checkedURLsForDel = append(checkedURLsForDel, URLForDel)
		case err := <-errChan:
			fmt.Println(err)
			return
		}
	}

	if len(checkedURLsForDel) > 0 {
		err := h.app.MarkDeleteBatchURLs(checkedURLsForDel)
		if err != nil {
			log.Printf("Cannot delete batch URLs. Error message:%s\n", err.Error())
		}
	}
}

func (h *shortenerHandler) createCookie(cookieName string, userID int) (*http.Cookie, error) {
	token := common.GetUserToken(userID)
	signedToken := common.SignMsg([]byte(token), h.secKey)

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

func (h *shortenerHandler) processCookies(r *http.Request) (processCookieResult, error) {
	userID := int(rand.Int31())

	// Process cookies
	cookieName := "token"
	userCookie, err := r.Cookie(cookieName)
	if err != nil {
		if !errors.Is(err, http.ErrNoCookie) {
			return processCookieResult{userID, nil}, err
		}
		userCookie, err = h.createCookie(cookieName, userID)
		if err != nil {
			return processCookieResult{userID, nil}, err
		}
	} else {
		// Checking sign of cookie
		curCookieValue := CookieData{}
		cookieValueUnescaped, err := url.QueryUnescape(userCookie.Value)
		if err != nil {
			return processCookieResult{userID, nil}, err
		}
		err = json.Unmarshal([]byte(cookieValueUnescaped), &curCookieValue)
		if err != nil {
			return processCookieResult{userID, nil}, err
		}
		expectedToken := common.GetUserToken(curCookieValue.UserID)
		signedExpectedToken := common.SignMsg([]byte(expectedToken), h.secKey)

		if hmac.Equal(curCookieValue.Token, signedExpectedToken) {
			userID = curCookieValue.UserID
		} else {
			userCookie, err = h.createCookie(cookieName, userID)
			if err != nil {
				return processCookieResult{userID, nil}, err
			}
		}
	}
	return processCookieResult{userID, userCookie}, nil
}
