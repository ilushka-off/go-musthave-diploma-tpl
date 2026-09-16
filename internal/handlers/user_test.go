package handlers

import (
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/ilushka-off/go-musthave-diploma-tpl/internal/auth"
	"github.com/ilushka-off/go-musthave-diploma-tpl/internal/models"
)

func TestRegister(t *testing.T) {
	tests := []struct {
		name       string
		body       string
		existing   string
		createErr  error
		wantStatus int
	}{
		{
			name:       "successful registration",
			body:       `{"login":"user","password":"secret"}`,
			wantStatus: http.StatusOK,
		},
		{
			name:       "malformed json",
			body:       `not json`,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "empty body",
			body:       ``,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "empty login",
			body:       `{"login":"","password":"secret"}`,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "empty password",
			body:       `{"login":"user","password":""}`,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "login longer than the column",
			body:       `{"login":"` + strings.Repeat("a", 33) + `","password":"secret"}`,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "login at the length limit",
			body:       `{"login":"` + strings.Repeat("a", 32) + `","password":"secret"}`,
			wantStatus: http.StatusOK,
		},
		{
			name:       "password longer than bcrypt allows",
			body:       `{"login":"user","password":"` + strings.Repeat("a", 100) + `"}`,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "login already taken",
			body:       `{"login":"taken","password":"secret"}`,
			existing:   "taken",
			wantStatus: http.StatusConflict,
		},
		{
			name:       "repository failure",
			body:       `{"login":"user","password":"secret"}`,
			createErr:  errors.New("db is down"),
			wantStatus: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env := newTestEnv(t)
			if tt.existing != "" {
				env.users.users[tt.existing] = models.User{ID: 1, Login: tt.existing}
			}
			env.users.createErr = tt.createErr

			resp := env.do(t, request{
				method: http.MethodPost,
				path:   "/api/user/register",
				body:   tt.body,
				header: map[string]string{"Content-Type": "application/json"},
			})

			if resp.status != tt.wantStatus {
				t.Fatalf("status = %d, want %d", resp.status, tt.wantStatus)
			}
			if tt.wantStatus == http.StatusOK {
				assertAuthenticated(t, env, resp)
			}
		})
	}
}

func TestLogin(t *testing.T) {
	tests := []struct {
		name       string
		body       string
		getErr     error
		wantStatus int
	}{
		{
			name:       "successful login",
			body:       `{"login":"user","password":"secret"}`,
			wantStatus: http.StatusOK,
		},
		{
			name:       "wrong password",
			body:       `{"login":"user","password":"wrong"}`,
			wantStatus: http.StatusUnauthorized,
		},
		{
			name:       "unknown login",
			body:       `{"login":"ghost","password":"secret"}`,
			wantStatus: http.StatusUnauthorized,
		},
		{
			name:       "malformed json",
			body:       `not json`,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "repository failure",
			body:       `{"login":"user","password":"secret"}`,
			getErr:     errors.New("db is down"),
			wantStatus: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env := newTestEnv(t)

			registered := env.do(t, request{
				method: http.MethodPost,
				path:   "/api/user/register",
				body:   `{"login":"user","password":"secret"}`,
			})
			if registered.status != http.StatusOK {
				t.Fatalf("setup registration failed with %d", registered.status)
			}

			env.users.getErr = tt.getErr

			resp := env.do(t, request{
				method: http.MethodPost,
				path:   "/api/user/login",
				body:   tt.body,
			})

			if resp.status != tt.wantStatus {
				t.Fatalf("status = %d, want %d", resp.status, tt.wantStatus)
			}
			if tt.wantStatus == http.StatusOK {
				assertAuthenticated(t, env, resp)
			}
		})
	}
}

func TestRegisterIssuesUsableToken(t *testing.T) {
	env := newTestEnv(t)

	resp := env.do(t, request{
		method: http.MethodPost,
		path:   "/api/user/register",
		body:   `{"login":"user","password":"secret"}`,
	})

	token := strings.TrimPrefix(resp.header.Get("Authorization"), "Bearer ")

	balance := env.do(t, request{method: http.MethodGet, path: "/api/user/balance", token: token})
	if balance.status != http.StatusOK {
		t.Fatalf("balance with fresh token = %d, want %d", balance.status, http.StatusOK)
	}
}

func assertAuthenticated(t *testing.T, env *testEnv, resp response) {
	t.Helper()

	header := resp.header.Get("Authorization")
	if !strings.HasPrefix(header, "Bearer ") {
		t.Fatalf("Authorization header = %q, want a Bearer token", header)
	}
	headerToken := strings.TrimPrefix(header, "Bearer ")

	var cookieToken string
	for _, cookie := range resp.cookies {
		if cookie.Name == auth.CookieName {
			cookieToken = cookie.Value
		}
	}
	if cookieToken == "" {
		t.Fatal("no auth cookie set")
	}
	if cookieToken != headerToken {
		t.Error("cookie and Authorization header carry different tokens")
	}

	if _, err := env.tokenManager.Parse(headerToken); err != nil {
		t.Errorf("issued token does not parse: %v", err)
	}
}
