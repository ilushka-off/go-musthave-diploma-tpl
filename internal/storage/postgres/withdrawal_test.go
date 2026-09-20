package postgres

import (
	"context"
	"errors"
	"testing"

	"github.com/ilushka-off/go-musthave-diploma-tpl/internal/storage"
	"github.com/shopspring/decimal"
)

func TestWithdrawalRepositoryCreateWithdrawal(t *testing.T) {
	ctx := context.Background()
	pool := testPool(t)
	repository := NewWithdrawalRepository(pool)
	userID := seedUser(t, pool, "user")

	if _, err := pool.Exec(ctx, `UPDATE users SET balance = 1000 WHERE id = $1`, userID); err != nil {
		t.Fatalf("set balance: %v", err)
	}

	withdrawalID, err := repository.CreateWithdrawal(ctx, userID, "2377225624", decimal.RequireFromString("751"))
	if err != nil {
		t.Fatalf("create withdrawal: %v", err)
	}
	if withdrawalID == 0 {
		t.Error("returned withdrawal id is zero")
	}

	want := decimal.RequireFromString("249")
	if got := currentBalance(t, pool, userID); !got.Equal(want) {
		t.Errorf("balance = %s, want %s", got, want)
	}

	t.Run("insufficient funds leaves the balance intact", func(t *testing.T) {
		_, err := repository.CreateWithdrawal(ctx, userID, "12345678903", decimal.RequireFromString("1000"))
		if !errors.Is(err, storage.ErrUserInsufficientFunds) {
			t.Fatalf("error = %v, want %v", err, storage.ErrUserInsufficientFunds)
		}
		if got := currentBalance(t, pool, userID); !got.Equal(want) {
			t.Errorf("balance = %s, want %s", got, want)
		}
	})

	t.Run("withdrawing the whole balance is allowed", func(t *testing.T) {
		if _, err := repository.CreateWithdrawal(ctx, userID, "4561261212345467", want); err != nil {
			t.Fatalf("create withdrawal: %v", err)
		}
		if got := currentBalance(t, pool, userID); !got.IsZero() {
			t.Errorf("balance = %s, want 0", got)
		}
	})

	t.Run("unknown user", func(t *testing.T) {
		_, err := repository.CreateWithdrawal(ctx, userID+1000, "79927398713", decimal.NewFromInt(1))
		if !errors.Is(err, storage.ErrUserInsufficientFunds) {
			t.Fatalf("error = %v, want %v", err, storage.ErrUserInsufficientFunds)
		}
	})
}

func TestWithdrawalRepositoryBalanceMatchesHistory(t *testing.T) {
	ctx := context.Background()
	pool := testPool(t)
	repository := NewWithdrawalRepository(pool)
	userID := seedUser(t, pool, "user")

	credited := decimal.RequireFromString("1000.06")
	if _, err := pool.Exec(ctx, `UPDATE users SET balance = $1 WHERE id = $2`, credited, userID); err != nil {
		t.Fatalf("set balance: %v", err)
	}

	sums := []string{"0.01", "751", "10.50"}
	for i, sum := range sums {
		number := []string{"2377225624", "12345678903", "4561261212345467"}[i]
		if _, err := repository.CreateWithdrawal(ctx, userID, number, decimal.RequireFromString(sum)); err != nil {
			t.Fatalf("create withdrawal %s: %v", sum, err)
		}
	}

	withdrawn, err := repository.GetWithdrawnByUserID(ctx, userID)
	if err != nil {
		t.Fatalf("get withdrawn: %v", err)
	}

	total := currentBalance(t, pool, userID).Add(withdrawn)
	if !total.Equal(credited) {
		t.Errorf("current + withdrawn = %s, want %s", total, credited)
	}
}

func TestWithdrawalRepositoryGetWithdrawalsByUserID(t *testing.T) {
	ctx := context.Background()
	pool := testPool(t)
	repository := NewWithdrawalRepository(pool)
	userID := seedUser(t, pool, "user")
	other := seedUser(t, pool, "other")

	if _, err := pool.Exec(ctx, `UPDATE users SET balance = 1000`); err != nil {
		t.Fatalf("set balance: %v", err)
	}

	for _, number := range []string{"2377225624", "12345678903", "4561261212345467"} {
		if _, err := repository.CreateWithdrawal(ctx, userID, number, decimal.NewFromInt(10)); err != nil {
			t.Fatalf("create withdrawal %s: %v", number, err)
		}
	}
	if _, err := repository.CreateWithdrawal(ctx, other, "79927398713", decimal.NewFromInt(10)); err != nil {
		t.Fatalf("create foreign withdrawal: %v", err)
	}

	withdrawals, err := repository.GetWithdrawalsByUserID(ctx, userID)
	if err != nil {
		t.Fatalf("get withdrawals: %v", err)
	}
	if len(withdrawals) != 3 {
		t.Fatalf("got %d withdrawals, want 3", len(withdrawals))
	}
	if withdrawals[0].Order != "4561261212345467" {
		t.Errorf("first withdrawal = %q, want the newest one", withdrawals[0].Order)
	}
	if withdrawals[2].Order != "2377225624" {
		t.Errorf("last withdrawal = %q, want the oldest one", withdrawals[2].Order)
	}
	for _, withdrawal := range withdrawals {
		if withdrawal.UserID != userID {
			t.Fatalf("withdrawal %q belongs to user %d, want %d", withdrawal.Order, withdrawal.UserID, userID)
		}
		if withdrawal.ProcessedAt.IsZero() {
			t.Errorf("withdrawal %q has no processed_at", withdrawal.Order)
		}
	}

	t.Run("user without withdrawals", func(t *testing.T) {
		empty := seedUser(t, pool, "empty")
		withdrawals, err := repository.GetWithdrawalsByUserID(ctx, empty)
		if err != nil {
			t.Fatalf("get withdrawals: %v", err)
		}
		if len(withdrawals) != 0 {
			t.Errorf("got %d withdrawals, want none", len(withdrawals))
		}
	})
}

func TestWithdrawalRepositoryGetWithdrawnByUserID(t *testing.T) {
	ctx := context.Background()
	pool := testPool(t)
	repository := NewWithdrawalRepository(pool)
	userID := seedUser(t, pool, "user")

	withdrawn, err := repository.GetWithdrawnByUserID(ctx, userID)
	if err != nil {
		t.Fatalf("get withdrawn: %v", err)
	}
	if !withdrawn.IsZero() {
		t.Errorf("withdrawn = %s, want 0", withdrawn)
	}

	if _, err := pool.Exec(ctx, `UPDATE users SET balance = 1000 WHERE id = $1`, userID); err != nil {
		t.Fatalf("set balance: %v", err)
	}
	for _, number := range []string{"2377225624", "12345678903"} {
		if _, err := repository.CreateWithdrawal(ctx, userID, number, decimal.RequireFromString("10.55")); err != nil {
			t.Fatalf("create withdrawal: %v", err)
		}
	}

	withdrawn, err = repository.GetWithdrawnByUserID(ctx, userID)
	if err != nil {
		t.Fatalf("get withdrawn: %v", err)
	}

	want := decimal.RequireFromString("21.10")
	if !withdrawn.Equal(want) {
		t.Errorf("withdrawn = %s, want %s", withdrawn, want)
	}
}
