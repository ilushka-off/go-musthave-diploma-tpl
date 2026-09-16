package handlers

import (
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/ilushka-off/go-musthave-diploma-tpl/internal/models"
	"github.com/ilushka-off/go-musthave-diploma-tpl/internal/storage"
	"github.com/shopspring/decimal"
)

func TestGetBalance(t *testing.T) {
	tests := []struct {
		name       string
		current    string
		withdrawn  string
		balanceEr  error
		sumErr     error
		noToken    bool
		wantStatus int
		wantBody   string
	}{
		{
			name:       "current and withdrawn are reported",
			current:    "500.5",
			withdrawn:  "42",
			wantStatus: http.StatusOK,
			wantBody:   `{"current":500.5,"withdrawn":42}`,
		},
		{
			name:       "zero balance",
			current:    "0",
			withdrawn:  "0",
			wantStatus: http.StatusOK,
			wantBody:   `{"current":0,"withdrawn":0}`,
		},
		{
			name:       "unauthenticated",
			noToken:    true,
			wantStatus: http.StatusUnauthorized,
		},
		{
			name:       "balance lookup fails",
			balanceEr:  errors.New("db is down"),
			wantStatus: http.StatusInternalServerError,
		},
		{
			name:       "withdrawn lookup fails",
			sumErr:     errors.New("db is down"),
			wantStatus: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env := newTestEnv(t)
			if tt.current != "" {
				env.users.balance = decimal.RequireFromString(tt.current)
			}
			if tt.withdrawn != "" {
				env.withdrawals.withdrawn = decimal.RequireFromString(tt.withdrawn)
			}
			env.users.balanceEr = tt.balanceEr
			env.withdrawals.sumErr = tt.sumErr

			req := request{method: http.MethodGet, path: "/api/user/balance"}
			if !tt.noToken {
				req.token = env.tokenFor(t, 1)
			}

			resp := env.do(t, req)
			if resp.StatusCode != tt.wantStatus {
				t.Fatalf("status = %d, want %d", resp.StatusCode, tt.wantStatus)
			}
			if tt.wantBody == "" {
				return
			}

			if got := resp.Header.Get("Content-Type"); got != "application/json" {
				t.Errorf("Content-Type = %q, want application/json", got)
			}
			assertJSONEqual(t, readBody(t, resp), tt.wantBody)
		})
	}
}

func TestWithdrawHandler(t *testing.T) {
	tests := []struct {
		name       string
		body       string
		createErr  error
		noToken    bool
		wantStatus int
		wantSum    string
	}{
		{
			name:       "successful withdrawal",
			body:       `{"order":"2377225624","sum":751}`,
			wantStatus: http.StatusOK,
			wantSum:    "751",
		},
		{
			name:       "fractional sum is rounded to cents",
			body:       `{"order":"2377225624","sum":0.005}`,
			wantStatus: http.StatusOK,
			wantSum:    "0.01",
		},
		{
			name:       "sub cent sum is rejected",
			body:       `{"order":"2377225624","sum":0.004}`,
			wantStatus: http.StatusUnprocessableEntity,
		},
		{
			name:       "zero sum",
			body:       `{"order":"2377225624","sum":0}`,
			wantStatus: http.StatusUnprocessableEntity,
		},
		{
			name:       "negative sum",
			body:       `{"order":"2377225624","sum":-5}`,
			wantStatus: http.StatusUnprocessableEntity,
		},
		{
			name:       "invalid order number",
			body:       `{"order":"12345678901","sum":10}`,
			wantStatus: http.StatusUnprocessableEntity,
		},
		{
			name:       "insufficient funds",
			body:       `{"order":"2377225624","sum":751}`,
			createErr:  storage.ErrUserInsufficientFunds,
			wantStatus: http.StatusPaymentRequired,
		},
		{
			name:       "malformed json",
			body:       `not json`,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "unauthenticated",
			body:       `{"order":"2377225624","sum":751}`,
			noToken:    true,
			wantStatus: http.StatusUnauthorized,
		},
		{
			name:       "repository failure",
			body:       `{"order":"2377225624","sum":751}`,
			createErr:  errors.New("db is down"),
			wantStatus: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env := newTestEnv(t)
			env.withdrawals.createErr = tt.createErr

			req := request{
				method: http.MethodPost,
				path:   "/api/user/balance/withdraw",
				body:   tt.body,
				header: map[string]string{"Content-Type": "application/json"},
			}
			if !tt.noToken {
				req.token = env.tokenFor(t, 1)
			}

			resp := env.do(t, req)
			if resp.StatusCode != tt.wantStatus {
				t.Fatalf("status = %d, want %d", resp.StatusCode, tt.wantStatus)
			}
			if tt.wantSum == "" {
				return
			}

			if len(env.withdrawals.created) != 1 {
				t.Fatalf("recorded %d withdrawals, want 1", len(env.withdrawals.created))
			}
			if got := env.withdrawals.created[0].Sum; !got.Equal(decimal.RequireFromString(tt.wantSum)) {
				t.Errorf("stored sum = %s, want %s", got, tt.wantSum)
			}
		})
	}
}

func TestListWithdrawals(t *testing.T) {
	processed := time.Date(2020, 12, 9, 16, 9, 57, 0, time.FixedZone("MSK", 3*60*60))

	tests := []struct {
		name        string
		withdrawals []models.Withdrawal
		listErr     error
		noToken     bool
		wantStatus  int
		wantBody    string
	}{
		{
			name:       "no withdrawals",
			wantStatus: http.StatusNoContent,
		},
		{
			name:       "unauthenticated",
			noToken:    true,
			wantStatus: http.StatusUnauthorized,
		},
		{
			name:       "repository failure",
			listErr:    errors.New("db is down"),
			wantStatus: http.StatusInternalServerError,
		},
		{
			name: "withdrawal is reported",
			withdrawals: []models.Withdrawal{
				{Order: "2377225624", Sum: decimal.NewFromInt(500), ProcessedAt: processed},
			},
			wantStatus: http.StatusOK,
			wantBody:   `[{"order":"2377225624","sum":500,"processed_at":"2020-12-09T16:09:57+03:00"}]`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env := newTestEnv(t)
			env.withdrawals.withdrawals = tt.withdrawals
			env.withdrawals.listErr = tt.listErr

			req := request{method: http.MethodGet, path: "/api/user/withdrawals"}
			if !tt.noToken {
				req.token = env.tokenFor(t, 1)
			}

			resp := env.do(t, req)
			if resp.StatusCode != tt.wantStatus {
				t.Fatalf("status = %d, want %d", resp.StatusCode, tt.wantStatus)
			}
			if tt.wantBody == "" {
				return
			}

			if got := resp.Header.Get("Content-Type"); got != "application/json" {
				t.Errorf("Content-Type = %q, want application/json", got)
			}
			assertJSONEqual(t, readBody(t, resp), tt.wantBody)
		})
	}
}
