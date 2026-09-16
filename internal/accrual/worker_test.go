package accrual

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ilushka-off/go-musthave-diploma-tpl/internal/models"
	"github.com/ilushka-off/go-musthave-diploma-tpl/internal/storage"
	"github.com/shopspring/decimal"
	"go.uber.org/zap"
)

var _ storage.OrderRepository = (*fakeOrderRepository)(nil)

type statusUpdate struct {
	number  string
	accrual *decimal.Decimal
	status  models.OrderStatus
}

type fakeOrderRepository struct {
	mu         sync.Mutex
	pending    []models.Order
	updates    []statusUpdate
	pendingErr error
	updateErr  error
}

func (f *fakeOrderRepository) GetPendingOrders(_ context.Context) ([]models.Order, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.pendingErr != nil {
		return nil, f.pendingErr
	}
	return f.pending, nil
}

func (f *fakeOrderRepository) UpdateStatusByNumber(_ context.Context, number string, accrual *decimal.Decimal, status models.OrderStatus) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.updateErr != nil {
		return f.updateErr
	}
	f.updates = append(f.updates, statusUpdate{number: number, accrual: accrual, status: status})
	return nil
}

func (f *fakeOrderRepository) recordedUpdates() []statusUpdate {
	f.mu.Lock()
	defer f.mu.Unlock()

	return append([]statusUpdate(nil), f.updates...)
}

func (f *fakeOrderRepository) CreateOrder(_ context.Context, _ int, _ string, _ models.OrderStatus) (int, error) {
	panic("not used in these tests")
}

func (f *fakeOrderRepository) GetOrderByNumber(_ context.Context, _ string) (models.Order, error) {
	panic("not used in these tests")
}

func (f *fakeOrderRepository) GetOrdersByUserID(_ context.Context, _ int) ([]models.Order, error) {
	panic("not used in these tests")
}

func newTestWorker(repository storage.OrderRepository, baseURL string) *Worker {
	return NewWorker(repository, zap.NewNop(), NewClient(baseURL), time.Millisecond)
}

func TestProcessPending(t *testing.T) {
	tests := []struct {
		name        string
		pending     []models.Order
		responses   map[string]string
		statusCodes map[string]int
		wantUpdates []statusUpdate
	}{
		{
			name:    "processed order is credited",
			pending: []models.Order{{Number: "1", Status: models.New}},
			responses: map[string]string{
				"1": `{"order":"1","status":"PROCESSED","accrual":500.5}`,
			},
			wantUpdates: []statusUpdate{
				{number: "1", accrual: decimalPtr("500.5"), status: models.Processed},
			},
		},
		{
			name:    "new order moves to processing",
			pending: []models.Order{{Number: "1", Status: models.New}},
			responses: map[string]string{
				"1": `{"order":"1","status":"REGISTERED"}`,
			},
			wantUpdates: []statusUpdate{
				{number: "1", status: models.Processing},
			},
		},
		{
			name:    "invalid order is stored",
			pending: []models.Order{{Number: "1", Status: models.New}},
			responses: map[string]string{
				"1": `{"order":"1","status":"INVALID"}`,
			},
			wantUpdates: []statusUpdate{
				{number: "1", status: models.Invalid},
			},
		},
		{
			name:        "unregistered order is skipped",
			pending:     []models.Order{{Number: "1", Status: models.New}},
			statusCodes: map[string]int{"1": http.StatusNoContent},
		},
		{
			name:        "server error is skipped",
			pending:     []models.Order{{Number: "1", Status: models.New}},
			statusCodes: map[string]int{"1": http.StatusInternalServerError},
		},
		{
			name:    "unknown status is skipped",
			pending: []models.Order{{Number: "1", Status: models.New}},
			responses: map[string]string{
				"1": `{"order":"1","status":"WAT"}`,
			},
		},
		{
			name:    "processing order stays in processing",
			pending: []models.Order{{Number: "1", Status: models.Processing}},
			responses: map[string]string{
				"1": `{"order":"1","status":"PROCESSING"}`,
			},
			wantUpdates: []statusUpdate{
				{number: "1", status: models.Processing},
			},
		},
		{
			name:    "changed accrual is stored",
			pending: []models.Order{{Number: "1", Status: models.Processing, Accrual: decimalPtr("10")}},
			responses: map[string]string{
				"1": `{"order":"1","status":"PROCESSING","accrual":20}`,
			},
			wantUpdates: []statusUpdate{
				{number: "1", accrual: decimalPtr("20"), status: models.Processing},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				number := r.URL.Path[len("/api/orders/"):]
				if code, ok := tt.statusCodes[number]; ok {
					w.WriteHeader(code)
					return
				}
				w.Write([]byte(tt.responses[number]))
			}))
			defer server.Close()

			repository := &fakeOrderRepository{pending: tt.pending}
			newTestWorker(repository, server.URL).processPending(context.Background())

			assertUpdates(t, repository.recordedUpdates(), tt.wantUpdates)
		})
	}
}

func TestProcessPendingRetriesAfterRateLimit(t *testing.T) {
	var calls atomic.Int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if calls.Add(1) == 1 {
			w.Header().Set("Retry-After", "1")
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		w.Write([]byte(`{"order":"1","status":"PROCESSED","accrual":100}`))
	}))
	defer server.Close()

	repository := &fakeOrderRepository{pending: []models.Order{{Number: "1", Status: models.New}}}
	newTestWorker(repository, server.URL).processPending(context.Background())

	assertUpdates(t, repository.recordedUpdates(), []statusUpdate{
		{number: "1", accrual: decimalPtr("100"), status: models.Processed},
	})
	if got := calls.Load(); got != 2 {
		t.Errorf("accrual calls = %d, want 2", got)
	}
}

func TestProcessPendingRateLimitDoesNotDropBatch(t *testing.T) {
	var limited atomic.Bool

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !limited.Swap(true) {
			w.Header().Set("Retry-After", "1")
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		number := r.URL.Path[len("/api/orders/"):]
		w.Write([]byte(`{"order":"` + number + `","status":"PROCESSED","accrual":1}`))
	}))
	defer server.Close()

	pending := []models.Order{
		{Number: "1", Status: models.New},
		{Number: "2", Status: models.New},
		{Number: "3", Status: models.New},
		{Number: "4", Status: models.New},
		{Number: "5", Status: models.New},
	}

	repository := &fakeOrderRepository{pending: pending}
	newTestWorker(repository, server.URL).processPending(context.Background())

	if got := len(repository.recordedUpdates()); got != len(pending) {
		t.Fatalf("updated %d orders, want %d", got, len(pending))
	}
}

func TestProcessPendingRepositoryErrors(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"order":"1","status":"PROCESSED","accrual":1}`))
	}))
	defer server.Close()

	t.Run("pending lookup fails", func(t *testing.T) {
		repository := &fakeOrderRepository{pendingErr: errors.New("db is down")}
		newTestWorker(repository, server.URL).processPending(context.Background())

		assertUpdates(t, repository.recordedUpdates(), nil)
	})

	t.Run("update fails", func(t *testing.T) {
		repository := &fakeOrderRepository{
			pending:   []models.Order{{Number: "1", Status: models.New}},
			updateErr: errors.New("db is down"),
		}
		newTestWorker(repository, server.URL).processPending(context.Background())

		assertUpdates(t, repository.recordedUpdates(), nil)
	})

	t.Run("no pending orders", func(t *testing.T) {
		repository := &fakeOrderRepository{}
		newTestWorker(repository, server.URL).processPending(context.Background())

		assertUpdates(t, repository.recordedUpdates(), nil)
	})
}

func TestProcessPendingStopsOnCancelledContext(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"order":"1","status":"PROCESSED","accrual":1}`))
	}))
	defer server.Close()

	pending := make([]models.Order, 0, 100)
	for i := range 100 {
		pending = append(pending, models.Order{Number: string(rune('a' + i%26)), Status: models.New})
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	repository := &fakeOrderRepository{pending: pending}

	done := make(chan struct{})
	go func() {
		newTestWorker(repository, server.URL).processPending(ctx)
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("processPending did not stop on cancelled context")
	}
}

func TestRunProcessesOrdersUntilContextIsDone(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"order":"1","status":"PROCESSED","accrual":1}`))
	}))
	defer server.Close()

	repository := &fakeOrderRepository{pending: []models.Order{{Number: "1", Status: models.New}}}
	worker := newTestWorker(repository, server.URL)

	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan struct{})
	go func() {
		worker.Run(ctx)
		close(done)
	}()

	deadline := time.After(5 * time.Second)
	for len(repository.recordedUpdates()) == 0 {
		select {
		case <-deadline:
			cancel()
			t.Fatal("Run did not process pending orders")
		case <-time.After(time.Millisecond):
		}
	}
	cancel()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Run did not stop on cancelled context")
	}
}

func equalAccrual(a, b *decimal.Decimal) bool {
	if a == nil || b == nil {
		return a == b
	}
	return a.Equal(*b)
}

func assertUpdates(t *testing.T, got, want []statusUpdate) {
	t.Helper()

	if len(got) != len(want) {
		t.Fatalf("got %d updates, want %d: %+v", len(got), len(want), got)
	}
	for i := range want {
		if got[i].number != want[i].number {
			t.Errorf("update %d number = %q, want %q", i, got[i].number, want[i].number)
		}
		if got[i].status != want[i].status {
			t.Errorf("update %d status = %q, want %q", i, got[i].status, want[i].status)
		}
		if !equalAccrual(got[i].accrual, want[i].accrual) {
			t.Errorf("update %d accrual = %v, want %v", i, got[i].accrual, want[i].accrual)
		}
	}
}
