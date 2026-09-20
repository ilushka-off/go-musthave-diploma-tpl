package handlers

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/ilushka-off/go-musthave-diploma-tpl/internal/auth"
	"github.com/ilushka-off/go-musthave-diploma-tpl/internal/models"
	"github.com/ilushka-off/go-musthave-diploma-tpl/internal/service"
	"github.com/ilushka-off/go-musthave-diploma-tpl/internal/storage"
	"github.com/shopspring/decimal"
	"go.uber.org/zap"
)

var (
	_ storage.UserRepository       = (*fakeUserRepository)(nil)
	_ storage.OrderRepository      = (*fakeOrderRepository)(nil)
	_ storage.WithdrawalRepository = (*fakeWithdrawalRepository)(nil)
)

type fakeUserRepository struct {
	mu        sync.Mutex
	users     map[string]models.User
	createErr error
	getErr    error
}

func newFakeUserRepository() *fakeUserRepository {
	return &fakeUserRepository{users: map[string]models.User{}}
}

func (f *fakeUserRepository) CreateUser(_ context.Context, user models.User) (int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.createErr != nil {
		return 0, f.createErr
	}
	if _, ok := f.users[user.Login]; ok {
		return 0, storage.ErrLoginExists
	}

	user.ID = len(f.users) + 1
	f.users[user.Login] = user

	return user.ID, nil
}

func (f *fakeUserRepository) GetUserByLogin(_ context.Context, login string) (models.User, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.getErr != nil {
		return models.User{}, f.getErr
	}
	user, ok := f.users[login]
	if !ok {
		return models.User{}, storage.ErrUserNotFound
	}
	return user, nil
}

func (f *fakeUserRepository) GetCurrentBalanceByUserID(_ context.Context, _ int) (decimal.Decimal, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.getErr != nil {
		return decimal.Decimal{}, f.getErr
	}
	return decimal.Decimal{}, nil
}

type fakeOrderRepository struct {
	mu       sync.Mutex
	orders   map[string]models.Order
	byUser   []models.Order
	createEr error
	getErr   error
	listErr  error
}

func newFakeOrderRepository() *fakeOrderRepository {
	return &fakeOrderRepository{orders: map[string]models.Order{}}
}

func (f *fakeOrderRepository) CreateOrder(_ context.Context, userID int, number string, status models.OrderStatus) (int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.createEr != nil {
		return 0, f.createEr
	}
	if _, ok := f.orders[number]; ok {
		return 0, storage.ErrOrderExists
	}

	order := models.Order{ID: len(f.orders) + 1, UserID: userID, Number: number, Status: status}
	f.orders[number] = order

	return order.ID, nil
}

func (f *fakeOrderRepository) GetOrderByNumber(_ context.Context, number string) (models.Order, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.getErr != nil {
		return models.Order{}, f.getErr
	}
	order, ok := f.orders[number]
	if !ok {
		return models.Order{}, storage.ErrOrderNotFound
	}
	return order, nil
}

func (f *fakeOrderRepository) GetOrdersByUserID(_ context.Context, _ int) ([]models.Order, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.listErr != nil {
		return nil, f.listErr
	}
	return f.byUser, nil
}

func (f *fakeOrderRepository) UpdateStatusByNumber(_ context.Context, _ string, _ *decimal.Decimal, _ models.OrderStatus) error {
	panic("not used in these tests")
}

func (f *fakeOrderRepository) GetPendingOrders(_ context.Context) ([]models.Order, error) {
	panic("not used in these tests")
}

type fakeWithdrawalRepository struct {
	mu          sync.Mutex
	withdrawals []models.Withdrawal
	withdrawn   decimal.Decimal
	balance     decimal.Decimal
	created     []models.Withdrawal
	createErr   error
	listErr     error
	balanceErr  error
}

func (f *fakeWithdrawalRepository) CreateWithdrawal(_ context.Context, userID int, order string, sum decimal.Decimal) (int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.createErr != nil {
		return 0, f.createErr
	}
	f.created = append(f.created, models.Withdrawal{UserID: userID, Order: order, Sum: sum})

	return len(f.created), nil
}

func (f *fakeWithdrawalRepository) GetWithdrawalsByUserID(_ context.Context, _ int) ([]models.Withdrawal, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.listErr != nil {
		return nil, f.listErr
	}
	return f.withdrawals, nil
}

func (f *fakeWithdrawalRepository) GetBalance(_ context.Context, _ int) (decimal.Decimal, decimal.Decimal, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.balanceErr != nil {
		return decimal.Decimal{}, decimal.Decimal{}, f.balanceErr
	}
	return f.balance, f.withdrawn, nil
}

type testEnv struct {
	server       *httptest.Server
	users        *fakeUserRepository
	orders       *fakeOrderRepository
	withdrawals  *fakeWithdrawalRepository
	tokenManager *auth.TokenManager
}

func newTestEnv(t *testing.T) *testEnv {
	t.Helper()

	decimal.MarshalJSONWithoutQuotes = true

	users := newFakeUserRepository()
	orders := newFakeOrderRepository()
	withdrawals := &fakeWithdrawalRepository{}
	tokenManager := auth.NewTokenManager([]byte("test-secret"), time.Hour)
	logger := zap.NewNop()

	router := NewRouter(
		NewUserHandler(service.NewUserService(users, tokenManager), logger),
		NewOrderHandler(service.NewOrderService(orders), logger),
		NewBalanceHandler(service.NewBalanceService(withdrawals), logger),
		logger,
		tokenManager,
	)

	server := httptest.NewServer(router)
	t.Cleanup(server.Close)

	return &testEnv{
		server:       server,
		users:        users,
		orders:       orders,
		withdrawals:  withdrawals,
		tokenManager: tokenManager,
	}
}

func (e *testEnv) tokenFor(t *testing.T, userID int) string {
	t.Helper()

	token, err := e.tokenManager.Issue(userID)
	if err != nil {
		t.Fatalf("issue token: %v", err)
	}
	return token
}

type request struct {
	method string
	path   string
	body   string
	token  string
	header map[string]string
	raw    []byte
}

type response struct {
	status  int
	header  http.Header
	cookies []*http.Cookie
	body    string
}

func (e *testEnv) do(t *testing.T, req request) response {
	t.Helper()

	body := io.Reader(nil)
	if req.raw != nil {
		body = bytes.NewReader(req.raw)
	} else if req.body != "" {
		body = strings.NewReader(req.body)
	}

	httpReq, err := http.NewRequest(req.method, e.server.URL+req.path, body)
	if err != nil {
		t.Fatalf("create request: %v", err)
	}
	if req.token != "" {
		httpReq.Header.Set("Authorization", "Bearer "+req.token)
	}
	for key, value := range req.header {
		httpReq.Header.Set(key, value)
	}

	resp, err := e.server.Client().Do(httpReq)
	if err != nil {
		t.Fatalf("send request: %v", err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}

	return response{
		status:  resp.StatusCode,
		header:  resp.Header.Clone(),
		cookies: resp.Cookies(),
		body:    string(data),
	}
}
