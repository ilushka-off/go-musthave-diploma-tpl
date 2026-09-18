package postgres

import (
	"context"
	"errors"
	"testing"

	"github.com/ilushka-off/go-musthave-diploma-tpl/internal/models"
	"github.com/ilushka-off/go-musthave-diploma-tpl/internal/storage"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"
)

func seedUser(t *testing.T, pool *pgxpool.Pool, login string) int {
	t.Helper()

	userID, err := NewUserRepository(pool).CreateUser(context.Background(), models.User{
		Login:        login,
		PasswordHash: "hash",
	})
	if err != nil {
		t.Fatalf("seed user %q: %v", login, err)
	}
	return userID
}

func currentBalance(t *testing.T, pool *pgxpool.Pool, userID int) decimal.Decimal {
	t.Helper()

	balance, err := NewUserRepository(pool).GetCurrentBalanceByUserID(context.Background(), userID)
	if err != nil {
		t.Fatalf("get balance: %v", err)
	}
	return balance
}

func TestOrderRepositoryCreateOrder(t *testing.T) {
	ctx := context.Background()
	pool := testPool(t)
	repository := NewOrderRepository(pool)
	userID := seedUser(t, pool, "user")

	orderID, err := repository.CreateOrder(ctx, userID, "12345678903", models.New)
	if err != nil {
		t.Fatalf("create order: %v", err)
	}
	if orderID == 0 {
		t.Error("returned order id is zero")
	}

	t.Run("duplicate number", func(t *testing.T) {
		_, err := repository.CreateOrder(ctx, userID, "12345678903", models.New)
		if !errors.Is(err, storage.ErrOrderExists) {
			t.Fatalf("error = %v, want %v", err, storage.ErrOrderExists)
		}
	})

	t.Run("duplicate number from another user", func(t *testing.T) {
		other := seedUser(t, pool, "other")
		_, err := repository.CreateOrder(ctx, other, "12345678903", models.New)
		if !errors.Is(err, storage.ErrOrderExists) {
			t.Fatalf("error = %v, want %v", err, storage.ErrOrderExists)
		}
	})

	t.Run("unknown user", func(t *testing.T) {
		_, err := repository.CreateOrder(ctx, userID+1000, "2377225624", models.New)
		if err == nil {
			t.Fatal("expected foreign key violation")
		}
	})
}

func TestOrderRepositoryGetOrderByNumber(t *testing.T) {
	ctx := context.Background()
	pool := testPool(t)
	repository := NewOrderRepository(pool)
	userID := seedUser(t, pool, "user")

	if _, err := repository.CreateOrder(ctx, userID, "12345678903", models.New); err != nil {
		t.Fatalf("create order: %v", err)
	}

	order, err := repository.GetOrderByNumber(ctx, "12345678903")
	if err != nil {
		t.Fatalf("get order: %v", err)
	}
	if order.UserID != userID {
		t.Errorf("user id = %d, want %d", order.UserID, userID)
	}
	if order.Status != models.New {
		t.Errorf("status = %q, want %q", order.Status, models.New)
	}
	if order.Accrual != nil {
		t.Errorf("accrual = %v, want nil", order.Accrual)
	}
	if order.UploadedAt.IsZero() {
		t.Error("uploaded_at was not set")
	}

	t.Run("unknown number", func(t *testing.T) {
		_, err := repository.GetOrderByNumber(ctx, "2377225624")
		if !errors.Is(err, storage.ErrOrderNotFound) {
			t.Fatalf("error = %v, want %v", err, storage.ErrOrderNotFound)
		}
	})
}

func TestOrderRepositoryGetOrdersByUserID(t *testing.T) {
	ctx := context.Background()
	pool := testPool(t)
	repository := NewOrderRepository(pool)
	userID := seedUser(t, pool, "user")
	other := seedUser(t, pool, "other")

	for _, number := range []string{"12345678903", "2377225624", "4561261212345467"} {
		if _, err := repository.CreateOrder(ctx, userID, number, models.New); err != nil {
			t.Fatalf("create order %s: %v", number, err)
		}
	}
	if _, err := repository.CreateOrder(ctx, other, "79927398713", models.New); err != nil {
		t.Fatalf("create foreign order: %v", err)
	}

	orders, err := repository.GetOrdersByUserID(ctx, userID)
	if err != nil {
		t.Fatalf("get orders: %v", err)
	}
	if len(orders) != 3 {
		t.Fatalf("got %d orders, want 3", len(orders))
	}
	if orders[0].Number != "4561261212345467" {
		t.Errorf("first order = %q, want the newest one", orders[0].Number)
	}
	if orders[2].Number != "12345678903" {
		t.Errorf("last order = %q, want the oldest one", orders[2].Number)
	}
	for _, order := range orders {
		if order.UserID != userID {
			t.Fatalf("order %q belongs to user %d, want %d", order.Number, order.UserID, userID)
		}
	}

	t.Run("user without orders", func(t *testing.T) {
		empty := seedUser(t, pool, "empty")
		orders, err := repository.GetOrdersByUserID(ctx, empty)
		if err != nil {
			t.Fatalf("get orders: %v", err)
		}
		if len(orders) != 0 {
			t.Errorf("got %d orders, want none", len(orders))
		}
	})
}

func TestOrderRepositoryUpdateStatusByNumber(t *testing.T) {
	ctx := context.Background()

	accrual := decimal.RequireFromString("500.50")

	tests := []struct {
		name        string
		initial     models.OrderStatus
		status      models.OrderStatus
		accrual     *decimal.Decimal
		wantStatus  models.OrderStatus
		wantBalance string
	}{
		{
			name:        "processed order credits the balance",
			initial:     models.New,
			status:      models.Processed,
			accrual:     &accrual,
			wantStatus:  models.Processed,
			wantBalance: "500.50",
		},
		{
			name:        "processed order without accrual credits nothing",
			initial:     models.New,
			status:      models.Processed,
			wantStatus:  models.Processed,
			wantBalance: "0",
		},
		{
			name:        "invalid order credits nothing",
			initial:     models.Processing,
			status:      models.Invalid,
			wantStatus:  models.Invalid,
			wantBalance: "0",
		},
		{
			name:        "new order moves to processing",
			initial:     models.New,
			status:      models.Processing,
			wantStatus:  models.Processing,
			wantBalance: "0",
		},
		{
			name:        "already processed order is not touched",
			initial:     models.Processed,
			status:      models.Processed,
			accrual:     &accrual,
			wantStatus:  models.Processed,
			wantBalance: "0",
		},
		{
			name:        "already invalid order is not touched",
			initial:     models.Invalid,
			status:      models.Processed,
			accrual:     &accrual,
			wantStatus:  models.Invalid,
			wantBalance: "0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pool := testPool(t)
			repository := NewOrderRepository(pool)
			userID := seedUser(t, pool, "user")

			if _, err := repository.CreateOrder(ctx, userID, "12345678903", tt.initial); err != nil {
				t.Fatalf("create order: %v", err)
			}

			if err := repository.UpdateStatusByNumber(ctx, "12345678903", tt.accrual, tt.status); err != nil {
				t.Fatalf("update status: %v", err)
			}

			order, err := repository.GetOrderByNumber(ctx, "12345678903")
			if err != nil {
				t.Fatalf("get order: %v", err)
			}
			if order.Status != tt.wantStatus {
				t.Errorf("status = %q, want %q", order.Status, tt.wantStatus)
			}

			want := decimal.RequireFromString(tt.wantBalance)
			if got := currentBalance(t, pool, userID); !got.Equal(want) {
				t.Errorf("balance = %s, want %s", got, want)
			}
		})
	}

	t.Run("unknown number is a no-op", func(t *testing.T) {
		pool := testPool(t)
		repository := NewOrderRepository(pool)

		if err := repository.UpdateStatusByNumber(ctx, "2377225624", &accrual, models.Processed); err != nil {
			t.Fatalf("update status: %v", err)
		}
	})

	t.Run("accrual is credited only once", func(t *testing.T) {
		pool := testPool(t)
		repository := NewOrderRepository(pool)
		userID := seedUser(t, pool, "user")

		if _, err := repository.CreateOrder(ctx, userID, "12345678903", models.New); err != nil {
			t.Fatalf("create order: %v", err)
		}
		for range 3 {
			if err := repository.UpdateStatusByNumber(ctx, "12345678903", &accrual, models.Processed); err != nil {
				t.Fatalf("update status: %v", err)
			}
		}

		if got := currentBalance(t, pool, userID); !got.Equal(accrual) {
			t.Errorf("balance = %s, want %s", got, accrual)
		}
	})

	t.Run("large accrual does not overflow the column", func(t *testing.T) {
		pool := testPool(t)
		repository := NewOrderRepository(pool)
		userID := seedUser(t, pool, "user")

		large := decimal.RequireFromString("10000000000")
		if _, err := repository.CreateOrder(ctx, userID, "12345678903", models.New); err != nil {
			t.Fatalf("create order: %v", err)
		}
		if err := repository.UpdateStatusByNumber(ctx, "12345678903", &large, models.Processed); err != nil {
			t.Fatalf("update status: %v", err)
		}

		order, err := repository.GetOrderByNumber(ctx, "12345678903")
		if err != nil {
			t.Fatalf("get order: %v", err)
		}
		if order.Status != models.Processed {
			t.Errorf("status = %q, want %q", order.Status, models.Processed)
		}
		if got := currentBalance(t, pool, userID); !got.Equal(large) {
			t.Errorf("balance = %s, want %s", got, large)
		}
	})
}

func TestOrderRepositoryGetPendingOrders(t *testing.T) {
	ctx := context.Background()
	pool := testPool(t)
	repository := NewOrderRepository(pool)
	userID := seedUser(t, pool, "user")

	seed := map[string]models.OrderStatus{
		"12345678903":      models.New,
		"2377225624":       models.Processing,
		"4561261212345467": models.Processed,
		"79927398713":      models.Invalid,
	}
	for number, status := range seed {
		if _, err := repository.CreateOrder(ctx, userID, number, status); err != nil {
			t.Fatalf("create order %s: %v", number, err)
		}
	}

	pending, err := repository.GetPendingOrders(ctx)
	if err != nil {
		t.Fatalf("get pending orders: %v", err)
	}
	if len(pending) != 2 {
		t.Fatalf("got %d pending orders, want 2", len(pending))
	}
	for _, order := range pending {
		if order.Status != models.New && order.Status != models.Processing {
			t.Errorf("order %q has final status %q", order.Number, order.Status)
		}
	}
}
