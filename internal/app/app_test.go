package app_test

import (
	"bytes"
	"fmt"
	"github.com/ffrxp/go-practicum/internal/app"
	"github.com/ffrxp/go-practicum/internal/common"
	"github.com/ffrxp/go-practicum/internal/handlers"
	"github.com/ffrxp/go-practicum/internal/storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"
)

func testRequest(t *testing.T, ts *httptest.Server, method, contentType, path string, content []byte) (*http.Response, string) {
	req, err := http.NewRequest(method, ts.URL+path, bytes.NewBuffer(content))
	require.NoError(t, err)
	if contentType != "" {
		req.Header.Set("content-type", contentType)
	}

	client := http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	resp, errDoReq := client.Do(req)
	require.NoError(t, errDoReq)

	respBody, errRead := ioutil.ReadAll(resp.Body)

	require.NoError(t, errRead)

	return resp, string(respBody)
}

func TestRouter(t *testing.T) {
	config := common.InitConfig()

	type Want struct {
		code        int
		location    string
		contentType string
		response    string
	}
	Tests := []struct {
		name        string
		method      string
		target      string
		content     string
		contentType string
		want        Want
	}{
		{
			name:        "GET test #1",
			method:      "GET",
			target:      "/api/user/urls",
			content:     "",
			contentType: "",
			want: Want{
				code:        204,
				location:    "",
				contentType: "",
				response:    "",
			},
		},
		{
			name:        "POST test #1",
			method:      "POST",
			target:      "/",
			content:     "yandex.com",
			contentType: "",
			want: Want{
				code:        201,
				location:    "",
				contentType: "",
				response:    fmt.Sprintf("%s/1389853602", config.BaseAddress),
			},
		},
		{
			name:        "POST test #2",
			method:      "POST",
			target:      "/qwqwqqwqwqwqw",
			content:     "yandex.com",
			contentType: "",
			want: Want{
				code:        400,
				location:    "",
				contentType: "",
				response:    "",
			},
		},
		{
			name:        "GET test #2",
			method:      "GET",
			target:      "/1389853602",
			content:     "",
			contentType: "",
			want: Want{
				code:        307,
				location:    "yandex.com",
				contentType: "",
				response:    "",
			},
		},
		{
			name:        "GET test #3",
			method:      "GET",
			target:      "//%dfghdfkjghs/asadad",
			content:     "",
			contentType: "",
			want: Want{
				code:        400,
				location:    "",
				contentType: "",
				response:    "",
			},
		},
		{
			name:        "POST test #3 (JSON)",
			method:      "POST",
			target:      "/api/shorten",
			content:     "{\"url\":\"ya.ru\"}",
			contentType: "application/json",
			want: Want{
				code:        201,
				location:    "",
				contentType: "application/json",
				response:    fmt.Sprintf("{\"result\":\"%s/3201241320\"}", config.BaseAddress),
			},
		},
		{
			name:        "POST test #4 (batch)",
			method:      "POST",
			target:      "/api/shorten/batch",
			content:     "[{\"correlation_id\":\"url1\",\"original_url\":\"stackoverflow.com\"},{\"correlation_id\":\"url2\",\"original_url\":\"go.dev\"}]",
			contentType: "application/json",
			want: Want{
				code:        201,
				location:    "",
				contentType: "application/json",
				response:    fmt.Sprintf("[{\"correlation_id\":\"url1\",\"short_url\":\"%s/2177322106\"},{\"correlation_id\":\"url2\",\"short_url\":\"%s/294555335\"}]", config.BaseAddress, config.BaseAddress),
			},
		},
		{
			name:        "POST test #5 (request with existing data)",
			method:      "POST",
			target:      "/",
			content:     "yandex.com",
			contentType: "",
			want: Want{
				code:        409,
				location:    "",
				contentType: "",
				response:    fmt.Sprintf("%s/1389853602", config.BaseAddress),
			},
		},
		{
			name:        "POST test #6 (request JSON with existing data)",
			method:      "POST",
			target:      "/api/shorten",
			content:     "{\"url\":\"ya.ru\"}",
			contentType: "application/json",
			want: Want{
				code:        409,
				location:    "",
				contentType: "application/json",
				response:    fmt.Sprintf("{\"result\":\"%s/3201241320\"}", config.BaseAddress),
			},
		},
	}

	appStorage := storage.NewDataStorage(config.StoragePath)
	defer appStorage.Close()
	sa := app.ShortenerApp{Storage: appStorage, BaseAddress: config.BaseAddress}

	h := handlers.NewShortenerHandler(&sa)
	ts := httptest.NewServer(h)
	defer ts.Close()

	for _, tt := range Tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Contains(t, []string{"GET", "POST"}, tt.method)

			resp, respContent := testRequest(t, ts, tt.method, tt.contentType, tt.target, []byte(tt.content))
			defer resp.Body.Close()

			assert.Equal(t, tt.want.code, resp.StatusCode)
			if tt.want.contentType != "" {
				assert.Equal(t, tt.want.contentType, resp.Header.Get("content-type"))
			}
			if tt.method == "GET" {
				assert.Equal(t, tt.want.location, resp.Header.Get("location"))
			}
			assert.Equal(t, tt.want.response, respContent)

		})
	}
}
