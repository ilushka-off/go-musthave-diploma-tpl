package accrual

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/ilushka-off/go-musthave-diploma-tpl/internal/models"
	"github.com/shopspring/decimal"
)

func TestGetOrderAccrual(t *testing.T) {
	tests := []struct {
		name        string
		statusCode  int
		retryAfter  string
		body        string
		wantStatus  models.OrderStatus
		wantAccrual *decimal.Decimal
		wantErr     error
		wantRetry   time.Duration
		wantAnyErr  bool
	}{
		{
			name:        "processed with accrual",
			statusCode:  http.StatusOK,
			body:        `{"order":"12345678903","status":"PROCESSED","accrual":500.5}`,
			wantStatus:  models.Processed,
			wantAccrual: decimalPtr("500.5"),
		},
		{
			name:       "processed without accrual",
			statusCode: http.StatusOK,
			body:       `{"order":"12345678903","status":"PROCESSED"}`,
			wantStatus: models.Processed,
		},
		{
			name:       "registered maps to processing",
			statusCode: http.StatusOK,
			body:       `{"order":"12345678903","status":"REGISTERED"}`,
			wantStatus: models.Processing,
		},
		{
			name:       "processing",
			statusCode: http.StatusOK,
			body:       `{"order":"12345678903","status":"PROCESSING"}`,
			wantStatus: models.Processing,
		},
		{
			name:       "invalid",
			statusCode: http.StatusOK,
			body:       `{"order":"12345678903","status":"INVALID"}`,
			wantStatus: models.Invalid,
		},
		{
			name:       "unknown status",
			statusCode: http.StatusOK,
			body:       `{"order":"12345678903","status":"WAT"}`,
			wantAnyErr: true,
		},
		{
			name:       "malformed body",
			statusCode: http.StatusOK,
			body:       `not json`,
			wantAnyErr: true,
		},
		{
			name:       "not registered",
			statusCode: http.StatusNoContent,
			wantErr:    ErrOrderNotRegistered,
		},
		{
			name:       "rate limited with retry after",
			statusCode: http.StatusTooManyRequests,
			retryAfter: "5",
			wantRetry:  5 * time.Second,
		},
		{
			name:       "rate limited without retry after",
			statusCode: http.StatusTooManyRequests,
			wantRetry:  60 * time.Second,
		},
		{
			name:       "rate limited with garbage retry after",
			statusCode: http.StatusTooManyRequests,
			retryAfter: "soon",
			wantRetry:  60 * time.Second,
		},
		{
			name:       "rate limited with zero retry after",
			statusCode: http.StatusTooManyRequests,
			retryAfter: "0",
			wantRetry:  60 * time.Second,
		},
		{
			name:       "server error",
			statusCode: http.StatusInternalServerError,
			wantAnyErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var gotPath string

			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				gotPath = r.URL.Path
				if tt.retryAfter != "" {
					w.Header().Set("Retry-After", tt.retryAfter)
				}
				w.WriteHeader(tt.statusCode)
				w.Write([]byte(tt.body))
			}))
			defer server.Close()

			client := NewClient(server.URL)
			result, err := client.GetOrderAccrual(context.Background(), "12345678903")

			if want := "/api/orders/12345678903"; gotPath != want {
				t.Errorf("request path = %q, want %q", gotPath, want)
			}

			if tt.wantRetry != 0 {
				rlErr, ok := errors.AsType[*RateLimitError](err)
				if !ok {
					t.Fatalf("error = %v, want *RateLimitError", err)
				}
				if rlErr.RetryAfter != tt.wantRetry {
					t.Errorf("RetryAfter = %v, want %v", rlErr.RetryAfter, tt.wantRetry)
				}
				if rlErr.Error() == "" {
					t.Error("RateLimitError.Error() is empty")
				}
				return
			}

			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("error = %v, want %v", err, tt.wantErr)
				}
				return
			}

			if tt.wantAnyErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if result.Status != tt.wantStatus {
				t.Errorf("status = %q, want %q", result.Status, tt.wantStatus)
			}
			if !equalAccrual(result.Accrual, tt.wantAccrual) {
				t.Errorf("accrual = %v, want %v", result.Accrual, tt.wantAccrual)
			}
		})
	}
}

func TestGetOrderAccrualRequestFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	server.Close()

	client := NewClient(server.URL)
	if _, err := client.GetOrderAccrual(context.Background(), "12345678903"); err == nil {
		t.Fatal("expected error for unreachable accrual system")
	}
}

func TestGetOrderAccrualCancelledContext(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer server.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	client := NewClient(server.URL)
	if _, err := client.GetOrderAccrual(ctx, "12345678903"); !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, want context.Canceled", err)
	}
}

func decimalPtr(value string) *decimal.Decimal {
	parsed := decimal.RequireFromString(value)
	return &parsed
}
