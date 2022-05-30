package main

import (
	"bytes"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestShorterHandler_ServeHTTP(t *testing.T) {

	type postWant struct {
		code     int
		response string
	}
	postTests := []struct {
		name   string
		target string
		body   string
		want   postWant
	}{
		{
			name:   "POST test #1",
			target: "/",
			body:   "yandex.com",
			want: postWant{
				code:     201,
				response: `http://localhost:8080/1389853602`,
			},
		},
		{
			name:   "POST test #2",
			target: "/qwqwqqwqwqwqw",
			body:   "yandex.com",
			want: postWant{
				code:     400,
				response: "invalid URL\n",
			},
		},
	}

	type getWant struct {
		code     int
		location string
		response string
	}
	getTests := []struct {
		name   string
		target string
		body   string
		want   getWant
	}{
		{
			name:   "GET test #1",
			target: "/1389853602",
			body:   "",
			want: getWant{
				code:     307,
				location: `yandex.com`,
				response: "",
			},
		},
	}

	sh := ShorterHandler{make(map[string]string)}
	for _, tt := range postTests {
		t.Run(tt.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodPost, tt.target, bytes.NewBuffer([]byte(tt.body)))
			w := httptest.NewRecorder()
			sh.ServeHTTP(w, request)
			result := w.Result()

			assert.Equal(t, tt.want.code, result.StatusCode)

			bodyContent, err := ioutil.ReadAll(result.Body)
			require.NoError(t, err)
			err = result.Body.Close()
			require.NoError(t, err)

			assert.Equal(t, tt.want.response, string(bodyContent))
		})
	}
	for _, tt := range getTests {
		t.Run(tt.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, tt.target, bytes.NewBuffer([]byte(tt.body)))
			w := httptest.NewRecorder()
			sh.ServeHTTP(w, request)
			result := w.Result()

			assert.Equal(t, tt.want.code, result.StatusCode)
			assert.Equal(t, tt.want.location, result.Header.Get("location"))

			bodyContent, err := ioutil.ReadAll(result.Body)
			require.NoError(t, err)
			err = result.Body.Close()
			require.NoError(t, err)

			assert.Equal(t, tt.want.response, string(bodyContent))
		})
	}
}
