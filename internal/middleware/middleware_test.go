package middleware

import (
	"bytes"
	"compress/gzip"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/ilushka-off/go-musthave-diploma-tpl/internal/auth"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
)

func TestAuth(t *testing.T) {
	tokens := auth.NewTokenManager([]byte("test-secret"), time.Hour)
	expired := auth.NewTokenManager([]byte("test-secret"), -time.Hour)
	foreign := auth.NewTokenManager([]byte("other-secret"), time.Hour)

	valid, err := tokens.Issue(42)
	if err != nil {
		t.Fatalf("issue token: %v", err)
	}
	expiredToken, err := expired.Issue(42)
	if err != nil {
		t.Fatalf("issue expired token: %v", err)
	}
	foreignToken, err := foreign.Issue(42)
	if err != nil {
		t.Fatalf("issue foreign token: %v", err)
	}

	tests := []struct {
		name       string
		header     string
		cookie     string
		wantStatus int
		wantUserID int
	}{
		{
			name:       "valid bearer token",
			header:     "Bearer " + valid,
			wantStatus: http.StatusOK,
			wantUserID: 42,
		},
		{
			name:       "valid cookie",
			cookie:     valid,
			wantStatus: http.StatusOK,
			wantUserID: 42,
		},
		{
			name:       "cookie wins over header",
			header:     "Bearer " + foreignToken,
			cookie:     valid,
			wantStatus: http.StatusOK,
			wantUserID: 42,
		},
		{
			name:       "no credentials",
			wantStatus: http.StatusUnauthorized,
		},
		{
			name:       "header without bearer prefix",
			header:     valid,
			wantStatus: http.StatusUnauthorized,
		},
		{
			name:       "garbage token",
			header:     "Bearer nonsense",
			wantStatus: http.StatusUnauthorized,
		},
		{
			name:       "expired token",
			header:     "Bearer " + expiredToken,
			wantStatus: http.StatusUnauthorized,
		},
		{
			name:       "token signed with another secret",
			header:     "Bearer " + foreignToken,
			wantStatus: http.StatusUnauthorized,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var gotUserID int
			var called bool

			handler := Auth(tokens)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				called = true
				gotUserID, _ = UserIDFromContext(r.Context())
			}))

			req := httptest.NewRequest(http.MethodGet, "/", nil)
			if tt.header != "" {
				req.Header.Set("Authorization", tt.header)
			}
			if tt.cookie != "" {
				req.AddCookie(&http.Cookie{Name: auth.CookieName, Value: tt.cookie})
			}

			recorder := httptest.NewRecorder()
			handler.ServeHTTP(recorder, req)

			if recorder.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d", recorder.Code, tt.wantStatus)
			}
			if tt.wantStatus != http.StatusOK {
				if called {
					t.Error("next handler was called for a rejected request")
				}
				return
			}
			if gotUserID != tt.wantUserID {
				t.Errorf("user id = %d, want %d", gotUserID, tt.wantUserID)
			}
		})
	}
}

func TestUserIDFromContextWithoutValue(t *testing.T) {
	if _, ok := UserIDFromContext(context.Background()); ok {
		t.Error("UserIDFromContext reported a user id on a bare context")
	}
}

func TestDecompress(t *testing.T) {
	var compressed bytes.Buffer
	writer := gzip.NewWriter(&compressed)
	if _, err := writer.Write([]byte("payload")); err != nil {
		t.Fatalf("compress: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close gzip writer: %v", err)
	}

	tests := []struct {
		name       string
		body       []byte
		encoding   string
		wantStatus int
		wantBody   string
	}{
		{
			name:       "gzipped body is decompressed",
			body:       compressed.Bytes(),
			encoding:   "gzip",
			wantStatus: http.StatusOK,
			wantBody:   "payload",
		},
		{
			name:       "plain body passes through",
			body:       []byte("payload"),
			wantStatus: http.StatusOK,
			wantBody:   "payload",
		},
		{
			name:       "broken gzip is rejected",
			body:       []byte("not gzip"),
			encoding:   "gzip",
			wantStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got string
			var gotEncoding string

			handler := Decompress(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				gotEncoding = r.Header.Get("Content-Encoding")
				data, err := io.ReadAll(r.Body)
				if err != nil {
					t.Errorf("read body: %v", err)
					return
				}
				got = string(data)
			}))

			req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(tt.body))
			if tt.encoding != "" {
				req.Header.Set("Content-Encoding", tt.encoding)
			}

			recorder := httptest.NewRecorder()
			handler.ServeHTTP(recorder, req)

			if recorder.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d", recorder.Code, tt.wantStatus)
			}
			if tt.wantStatus != http.StatusOK {
				return
			}
			if got != tt.wantBody {
				t.Errorf("body = %q, want %q", got, tt.wantBody)
			}
			if tt.encoding == "gzip" && gotEncoding != "" {
				t.Errorf("Content-Encoding = %q, want it stripped", gotEncoding)
			}
		})
	}
}

func TestLogger(t *testing.T) {
	tests := []struct {
		name       string
		handler    http.HandlerFunc
		wantStatus int64
	}{
		{
			name:       "explicit status is logged",
			handler:    func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusAccepted) },
			wantStatus: http.StatusAccepted,
		},
		{
			name:       "implicit 200 is logged",
			handler:    func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("ok")) },
			wantStatus: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			core, logs := observer.New(zapcore.InfoLevel)

			handler := Logger(zap.New(core))(tt.handler)
			handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/api/user/orders", nil))

			entries := logs.All()
			if len(entries) != 1 {
				t.Fatalf("logged %d entries, want 1", len(entries))
			}

			fields := entries[0].ContextMap()
			if got := fields["status"]; got != tt.wantStatus {
				t.Errorf("status field = %v, want %d", got, tt.wantStatus)
			}
			if got := fields["method"]; got != http.MethodPost {
				t.Errorf("method field = %v, want %s", got, http.MethodPost)
			}
			if got := fields["uri"]; got != "/api/user/orders" {
				t.Errorf("uri field = %v, want /api/user/orders", got)
			}
			if _, ok := fields["duration"]; !ok {
				t.Error("duration field is missing")
			}
		})
	}
}
