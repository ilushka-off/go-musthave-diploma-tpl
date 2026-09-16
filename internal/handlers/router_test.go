package handlers

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"io"
	"net/http"
	"testing"
)

func TestProtectedRoutesRequireAuth(t *testing.T) {
	routes := []struct {
		method string
		path   string
	}{
		{http.MethodPost, "/api/user/orders"},
		{http.MethodGet, "/api/user/orders"},
		{http.MethodPost, "/api/user/balance/withdraw"},
		{http.MethodGet, "/api/user/balance"},
		{http.MethodGet, "/api/user/withdrawals"},
	}

	env := newTestEnv(t)

	for _, route := range routes {
		t.Run(route.method+" "+route.path, func(t *testing.T) {
			resp := env.do(t, request{method: route.method, path: route.path, body: `{}`})
			if resp.status != http.StatusUnauthorized {
				t.Errorf("status = %d, want %d", resp.status, http.StatusUnauthorized)
			}
		})
	}
}

func TestProtectedRoutesRejectBadToken(t *testing.T) {
	env := newTestEnv(t)

	tokens := map[string]string{
		"garbage":        "not-a-token",
		"wrong signture": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJleHAiOjk5OTk5OTk5OTksIlVzZXJJRCI6MX0.bad",
	}

	for name, token := range tokens {
		t.Run(name, func(t *testing.T) {
			resp := env.do(t, request{method: http.MethodGet, path: "/api/user/balance", token: token})
			if resp.status != http.StatusUnauthorized {
				t.Errorf("status = %d, want %d", resp.status, http.StatusUnauthorized)
			}
		})
	}
}

func TestCookieAuthIsAccepted(t *testing.T) {
	env := newTestEnv(t)

	registered := env.do(t, request{
		method: http.MethodPost,
		path:   "/api/user/register",
		body:   `{"login":"user","password":"secret"}`,
	})

	req, err := http.NewRequest(http.MethodGet, env.server.URL+"/api/user/balance", nil)
	if err != nil {
		t.Fatalf("create request: %v", err)
	}
	for _, cookie := range registered.cookies {
		req.AddCookie(cookie)
	}

	resp, err := env.server.Client().Do(req)
	if err != nil {
		t.Fatalf("send request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
}

func TestGzipRequestBodyIsDecompressed(t *testing.T) {
	env := newTestEnv(t)

	var compressed bytes.Buffer
	writer := gzip.NewWriter(&compressed)
	if _, err := writer.Write([]byte(`{"login":"user","password":"secret"}`)); err != nil {
		t.Fatalf("compress body: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close gzip writer: %v", err)
	}

	resp := env.do(t, request{
		method: http.MethodPost,
		path:   "/api/user/register",
		raw:    compressed.Bytes(),
		header: map[string]string{
			"Content-Type":     "application/json",
			"Content-Encoding": "gzip",
		},
	})

	if resp.status != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.status, http.StatusOK)
	}
	if _, ok := env.users.users["user"]; !ok {
		t.Error("user was not created from the gzipped body")
	}
}

func TestBrokenGzipRequestBodyIsRejected(t *testing.T) {
	env := newTestEnv(t)

	resp := env.do(t, request{
		method: http.MethodPost,
		path:   "/api/user/register",
		raw:    []byte("this is not gzip"),
		header: map[string]string{"Content-Encoding": "gzip"},
	})

	if resp.status != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", resp.status, http.StatusBadRequest)
	}
}

func TestGzipResponseIsCompressed(t *testing.T) {
	env := newTestEnv(t)

	req, err := http.NewRequest(http.MethodGet, env.server.URL+"/api/user/balance", nil)
	if err != nil {
		t.Fatalf("create request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer "+env.tokenFor(t, 1))
	req.Header.Set("Accept-Encoding", "gzip")

	transport := &http.Transport{DisableCompression: true}
	defer transport.CloseIdleConnections()

	resp, err := (&http.Client{Transport: transport}).Do(req)
	if err != nil {
		t.Fatalf("send request: %v", err)
	}
	defer resp.Body.Close()

	if got := resp.Header.Get("Content-Encoding"); got != "gzip" {
		t.Fatalf("Content-Encoding = %q, want gzip", got)
	}

	reader, err := gzip.NewReader(resp.Body)
	if err != nil {
		t.Fatalf("read gzip response: %v", err)
	}
	defer reader.Close()

	data, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("decompress response: %v", err)
	}

	var balance BalanceResponseDTO
	if err := json.Unmarshal(data, &balance); err != nil {
		t.Fatalf("decode decompressed body %q: %v", data, err)
	}
}
