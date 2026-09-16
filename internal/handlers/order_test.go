package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/ilushka-off/go-musthave-diploma-tpl/internal/models"
	"github.com/shopspring/decimal"
)

func TestUploadOrder(t *testing.T) {
	const (
		validNumber   = "12345678903"
		otherNumber   = "2377225624"
		invalidNumber = "12345678901"
	)

	tests := []struct {
		name       string
		number     string
		owner      int
		createEr   error
		getErr     error
		noToken    bool
		wantStatus int
	}{
		{
			name:       "new order accepted",
			number:     validNumber,
			wantStatus: http.StatusAccepted,
		},
		{
			name:       "same user uploads twice",
			number:     validNumber,
			owner:      1,
			wantStatus: http.StatusOK,
		},
		{
			name:       "order owned by another user",
			number:     validNumber,
			owner:      2,
			wantStatus: http.StatusConflict,
		},
		{
			name:       "invalid luhn number",
			number:     invalidNumber,
			wantStatus: http.StatusUnprocessableEntity,
		},
		{
			name:       "non numeric body",
			number:     "abc",
			wantStatus: http.StatusUnprocessableEntity,
		},
		{
			name:       "empty body",
			number:     "",
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "whitespace body",
			number:     "   \n",
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "unauthenticated",
			number:     validNumber,
			noToken:    true,
			wantStatus: http.StatusUnauthorized,
		},
		{
			name:       "create failure",
			number:     otherNumber,
			createEr:   errors.New("db is down"),
			wantStatus: http.StatusInternalServerError,
		},
		{
			name:       "lookup failure after conflict",
			number:     validNumber,
			owner:      1,
			getErr:     errors.New("db is down"),
			wantStatus: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env := newTestEnv(t)
			if tt.owner != 0 {
				env.orders.orders[tt.number] = models.Order{
					ID:     1,
					UserID: tt.owner,
					Number: tt.number,
					Status: models.New,
				}
			}
			env.orders.createEr = tt.createEr
			env.orders.getErr = tt.getErr

			req := request{
				method: http.MethodPost,
				path:   "/api/user/orders",
				body:   tt.number,
				header: map[string]string{"Content-Type": "text/plain"},
			}
			if !tt.noToken {
				req.token = env.tokenFor(t, 1)
			}

			resp := env.do(t, req)
			if resp.status != tt.wantStatus {
				t.Fatalf("status = %d, want %d", resp.status, tt.wantStatus)
			}
		})
	}
}

func TestUploadOrderTrimsWhitespace(t *testing.T) {
	env := newTestEnv(t)

	resp := env.do(t, request{
		method: http.MethodPost,
		path:   "/api/user/orders",
		body:   "  12345678903\n",
		token:  env.tokenFor(t, 1),
	})

	if resp.status != http.StatusAccepted {
		t.Fatalf("status = %d, want %d", resp.status, http.StatusAccepted)
	}
	if _, ok := env.orders.orders["12345678903"]; !ok {
		t.Error("order number was not trimmed before storing")
	}
}

func TestListOrders(t *testing.T) {
	uploaded := time.Date(2020, 12, 10, 15, 15, 45, 0, time.FixedZone("MSK", 3*60*60))
	accrual := decimal.RequireFromString("500.5")

	tests := []struct {
		name       string
		orders     []models.Order
		listErr    error
		noToken    bool
		wantStatus int
		wantBody   string
	}{
		{
			name:       "no orders",
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
			name: "processed order exposes accrual",
			orders: []models.Order{
				{Number: "9278923470", Status: models.Processed, Accrual: &accrual, UploadedAt: uploaded},
			},
			wantStatus: http.StatusOK,
			wantBody:   `[{"number":"9278923470","status":"PROCESSED","accrual":500.5,"uploaded_at":"2020-12-10T15:15:45+03:00"}]`,
		},
		{
			name: "order without accrual omits the field",
			orders: []models.Order{
				{Number: "12345678903", Status: models.Processing, UploadedAt: uploaded},
			},
			wantStatus: http.StatusOK,
			wantBody:   `[{"number":"12345678903","status":"PROCESSING","uploaded_at":"2020-12-10T15:15:45+03:00"}]`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env := newTestEnv(t)
			env.orders.byUser = tt.orders
			env.orders.listErr = tt.listErr

			req := request{method: http.MethodGet, path: "/api/user/orders"}
			if !tt.noToken {
				req.token = env.tokenFor(t, 1)
			}

			resp := env.do(t, req)
			if resp.status != tt.wantStatus {
				t.Fatalf("status = %d, want %d", resp.status, tt.wantStatus)
			}
			if tt.wantBody == "" {
				return
			}

			if got := resp.header.Get("Content-Type"); got != "application/json" {
				t.Errorf("Content-Type = %q, want application/json", got)
			}
			assertJSONEqual(t, resp.body, tt.wantBody)
		})
	}
}

func TestListOrdersPreservesRepositoryOrder(t *testing.T) {
	env := newTestEnv(t)

	newest := time.Date(2020, 12, 10, 15, 15, 45, 0, time.UTC)
	oldest := time.Date(2020, 12, 9, 16, 9, 53, 0, time.UTC)

	env.orders.byUser = []models.Order{
		{Number: "9278923470", Status: models.Processed, UploadedAt: newest},
		{Number: "346436439", Status: models.Invalid, UploadedAt: oldest},
	}

	resp := env.do(t, request{method: http.MethodGet, path: "/api/user/orders", token: env.tokenFor(t, 1)})

	var got []OrderDTO
	if err := json.Unmarshal([]byte(resp.body), &got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d orders, want 2", len(got))
	}
	if got[0].Number != "9278923470" || got[1].Number != "346436439" {
		t.Errorf("order sequence = %q, %q, want newest first", got[0].Number, got[1].Number)
	}
}

func assertJSONEqual(t *testing.T, got, want string) {
	t.Helper()

	var gotValue, wantValue any
	if err := json.Unmarshal([]byte(got), &gotValue); err != nil {
		t.Fatalf("decode got %q: %v", got, err)
	}
	if err := json.Unmarshal([]byte(want), &wantValue); err != nil {
		t.Fatalf("decode want %q: %v", want, err)
	}

	gotNormalized, _ := json.Marshal(gotValue)
	wantNormalized, _ := json.Marshal(wantValue)

	if string(gotNormalized) != string(wantNormalized) {
		t.Errorf("body = %s, want %s", gotNormalized, wantNormalized)
	}
}
