package main

import (
	"bytes"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"
)

func testRequest(t *testing.T, ts *httptest.Server, method, path string, body []byte) (*http.Response, string) {
	req, err := http.NewRequest(method, ts.URL+path, bytes.NewBuffer(body))
	require.NoError(t, err)

	client := http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	resp, errDoReq := client.Do(req)
	require.NoError(t, errDoReq)

	respBody, errRead := ioutil.ReadAll(resp.Body)
	require.NoError(t, errRead)

	defer func(Body io.ReadCloser) {
		err := Body.Close()
		require.NoError(t, err)
	}(resp.Body)

	return resp, string(respBody)
}

func TestRouter(t *testing.T) {
	type Want struct {
		code     int
		location string
		response string
	}
	Tests := []struct {
		name    string
		method  string
		target  string
		content string
		want    Want
	}{
		{
			name:    "POST test #1",
			method:  "POST",
			target:  "/",
			content: "yandex.com",
			want: Want{
				code:     201,
				location: "",
				response: `http://localhost:8080/1389853602`,
			},
		},
		{
			name:    "POST test #2",
			method:  "POST",
			target:  "/qwqwqqwqwqwqw",
			content: "yandex.com",
			want: Want{
				code:     400,
				location: "",
				response: "",
			},
		},
		{
			name:    "GET test #1",
			method:  "GET",
			target:  "/1389853602",
			content: "",
			want: Want{
				code:     307,
				location: "yandex.com",
				response: "",
			},
		},
		{
			name:    "GET test #2",
			method:  "GET",
			target:  "//%dfghdfkjghs/asadad",
			content: "",
			want: Want{
				code:     400,
				location: "",
				response: "",
			},
		},
	}

	r := NewRouter()
	ts := httptest.NewServer(r)
	defer ts.Close()

	for _, tt := range Tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.method != "GET" && tt.method != "POST" {
				t.Fatal("Error. Unknown test method")
			}

			resp, contentResp := testRequest(t, ts, tt.method, tt.target, []byte(tt.content))

			assert.Equal(t, tt.want.code, resp.StatusCode)
			if tt.method == "GET" {
				assert.Equal(t, tt.want.location, resp.Header.Get("location"))
			}
			assert.Equal(t, tt.want.response, contentResp)

		})
	}
}
