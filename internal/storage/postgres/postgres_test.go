package postgres

import (
	"context"
	"os"
	"sync"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

const databaseURIEnv = "TEST_DATABASE_URI"

var migrateOnce sync.Once

func testPool(t *testing.T) *pgxpool.Pool {
	t.Helper()

	databaseURI := os.Getenv(databaseURIEnv)
	if databaseURI == "" {
		t.Skipf("%s is not set", databaseURIEnv)
	}

	var migrateErr error
	migrateOnce.Do(func() {
		migrateErr = RunMigrations(databaseURI)
	})
	if migrateErr != nil {
		t.Fatalf("run migrations: %v", migrateErr)
	}

	pool, err := OpenPool(context.Background(), databaseURI)
	if err != nil {
		t.Fatalf("open pool: %v", err)
	}
	t.Cleanup(pool.Close)

	_, err = pool.Exec(context.Background(),
		`TRUNCATE users, orders, withdrawals RESTART IDENTITY CASCADE`)
	if err != nil {
		t.Fatalf("truncate tables: %v", err)
	}

	return pool
}

func TestOpenPool(t *testing.T) {
	t.Run("malformed uri", func(t *testing.T) {
		if _, err := OpenPool(context.Background(), "://nonsense"); err == nil {
			t.Fatal("expected error for a malformed database uri")
		}
	})

	t.Run("unreachable database", func(t *testing.T) {
		_, err := OpenPool(context.Background(), "postgres://user:pass@127.0.0.1:1/db?sslmode=disable&connect_timeout=1")
		if err == nil {
			t.Fatal("expected error for an unreachable database")
		}
	})
}

func TestRunMigrations(t *testing.T) {
	t.Run("malformed uri", func(t *testing.T) {
		if err := RunMigrations("://nonsense"); err == nil {
			t.Fatal("expected error for a malformed database uri")
		}
	})

	t.Run("is idempotent", func(t *testing.T) {
		databaseURI := os.Getenv(databaseURIEnv)
		if databaseURI == "" {
			t.Skipf("%s is not set", databaseURIEnv)
		}
		if err := RunMigrations(databaseURI); err != nil {
			t.Fatalf("first run: %v", err)
		}
		if err := RunMigrations(databaseURI); err != nil {
			t.Fatalf("second run: %v", err)
		}
	})
}
