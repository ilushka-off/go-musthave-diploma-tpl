package auth

import (
	"errors"
	"strings"
	"testing"
)

func TestHashPassword(t *testing.T) {
	t.Run("hash differs from the password", func(t *testing.T) {
		const password = "qwerty"

		hash, err := HashPassword(password)
		if err != nil {
			t.Fatalf("HashPassword() unexpected error: %v", err)
		}
		if hash == password {
			t.Error("HashPassword() returned the password itself")
		}
	})

	t.Run("same password hashed twice gives different hashes", func(t *testing.T) {
		const password = "qwerty"

		first, err := HashPassword(password)
		if err != nil {
			t.Fatalf("HashPassword() unexpected error: %v", err)
		}
		second, err := HashPassword(password)
		if err != nil {
			t.Fatalf("HashPassword() unexpected error: %v", err)
		}

		if first == second {
			t.Error("HashPassword() produced equal hashes, salt is not applied")
		}
	})

	t.Run("password longer than bcrypt limit", func(t *testing.T) {
		_, err := HashPassword(strings.Repeat("a", 100))
		if !errors.Is(err, ErrPasswordTooLong) {
			t.Errorf("HashPassword() error = %v, want %v", err, ErrPasswordTooLong)
		}
	})
}

func TestCheckPassword(t *testing.T) {
	const password = "qwerty"

	hash, err := HashPassword(password)
	if err != nil {
		t.Fatalf("HashPassword() unexpected error: %v", err)
	}

	t.Run("correct password", func(t *testing.T) {
		if err := CheckPassword(hash, password); err != nil {
			t.Errorf("CheckPassword() error = %v, want nil", err)
		}
	})

	t.Run("wrong password", func(t *testing.T) {
		err := CheckPassword(hash, "wrong")
		if !errors.Is(err, ErrInvalidPassword) {
			t.Errorf("CheckPassword() error = %v, want %v", err, ErrInvalidPassword)
		}
	})

	t.Run("broken hash is not reported as a wrong password", func(t *testing.T) {
		err := CheckPassword("not-a-hash", password)
		if err == nil {
			t.Fatal("CheckPassword() error = nil, want an error")
		}
		if errors.Is(err, ErrInvalidPassword) {
			t.Error("CheckPassword() reported a broken hash as a wrong password")
		}
	})
}
