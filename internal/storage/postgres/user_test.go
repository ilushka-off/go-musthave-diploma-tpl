package postgres

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/ilushka-off/go-musthave-diploma-tpl/internal/models"
	"github.com/ilushka-off/go-musthave-diploma-tpl/internal/storage"
	"github.com/shopspring/decimal"
)

func TestUserRepositoryCreateUser(t *testing.T) {
	ctx := context.Background()
	repository := NewUserRepository(testPool(t))

	userID, err := repository.CreateUser(ctx, models.User{Login: "user", PasswordHash: "hash"})
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	if userID == 0 {
		t.Error("returned user id is zero")
	}

	t.Run("duplicate login", func(t *testing.T) {
		_, err := repository.CreateUser(ctx, models.User{Login: "user", PasswordHash: "hash"})
		if !errors.Is(err, storage.ErrLoginExists) {
			t.Fatalf("error = %v, want %v", err, storage.ErrLoginExists)
		}
	})

	t.Run("login longer than the column", func(t *testing.T) {
		_, err := repository.CreateUser(ctx, models.User{
			Login:        strings.Repeat("a", 33),
			PasswordHash: "hash",
		})
		if err == nil {
			t.Fatal("expected error for an oversized login")
		}
	})
}

func TestUserRepositoryGetUserByLogin(t *testing.T) {
	ctx := context.Background()
	repository := NewUserRepository(testPool(t))

	userID, err := repository.CreateUser(ctx, models.User{Login: "user", PasswordHash: "hash"})
	if err != nil {
		t.Fatalf("create user: %v", err)
	}

	user, err := repository.GetUserByLogin(ctx, "user")
	if err != nil {
		t.Fatalf("get user: %v", err)
	}
	if user.ID != userID {
		t.Errorf("id = %d, want %d", user.ID, userID)
	}
	if user.Login != "user" {
		t.Errorf("login = %q, want %q", user.Login, "user")
	}
	if user.PasswordHash != "hash" {
		t.Errorf("password hash = %q, want %q", user.PasswordHash, "hash")
	}
	if !user.Balance.IsZero() {
		t.Errorf("balance = %s, want 0", user.Balance)
	}

	t.Run("unknown login", func(t *testing.T) {
		_, err := repository.GetUserByLogin(ctx, "ghost")
		if !errors.Is(err, storage.ErrUserNotFound) {
			t.Fatalf("error = %v, want %v", err, storage.ErrUserNotFound)
		}
	})
}

func TestUserRepositoryGetCurrentBalanceByUserID(t *testing.T) {
	ctx := context.Background()
	pool := testPool(t)
	repository := NewUserRepository(pool)

	userID, err := repository.CreateUser(ctx, models.User{Login: "user", PasswordHash: "hash"})
	if err != nil {
		t.Fatalf("create user: %v", err)
	}

	balance, err := repository.GetCurrentBalanceByUserID(ctx, userID)
	if err != nil {
		t.Fatalf("get balance: %v", err)
	}
	if !balance.IsZero() {
		t.Errorf("balance = %s, want 0", balance)
	}

	want := decimal.RequireFromString("500.50")
	if _, err := pool.Exec(ctx, `UPDATE users SET balance = $1 WHERE id = $2`, want, userID); err != nil {
		t.Fatalf("set balance: %v", err)
	}

	balance, err = repository.GetCurrentBalanceByUserID(ctx, userID)
	if err != nil {
		t.Fatalf("get balance: %v", err)
	}
	if !balance.Equal(want) {
		t.Errorf("balance = %s, want %s", balance, want)
	}

	t.Run("unknown user", func(t *testing.T) {
		_, err := repository.GetCurrentBalanceByUserID(ctx, userID+1000)
		if !errors.Is(err, storage.ErrUserNotFound) {
			t.Fatalf("error = %v, want %v", err, storage.ErrUserNotFound)
		}
	})
}
