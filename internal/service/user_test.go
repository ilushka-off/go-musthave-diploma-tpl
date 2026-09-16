package service

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/ilushka-off/go-musthave-diploma-tpl/internal/auth"
	"github.com/ilushka-off/go-musthave-diploma-tpl/internal/models"
	"github.com/ilushka-off/go-musthave-diploma-tpl/internal/storage"
)

func newTestTokenManager() *auth.TokenManager {
	return auth.NewTokenManager([]byte("test-secret"), time.Hour)
}

func TestRegister(t *testing.T) {
	errDB := errors.New("db is down")

	tests := []struct {
		name      string
		login     string
		password  string
		createErr error
		wantErr   error
	}{
		{
			name:     "empty login",
			login:    "",
			password: "qwerty",
			wantErr:  ErrInvalidInput,
		},
		{
			name:     "empty password",
			login:    "ivan",
			password: "",
			wantErr:  ErrInvalidInput,
		},
		{
			name:     "password longer than bcrypt limit",
			login:    "ivan",
			password: strings.Repeat("a", 100),
			wantErr:  ErrInvalidInput,
		},
		{
			name:      "login already taken",
			login:     "ivan",
			password:  "qwerty",
			createErr: storage.ErrLoginExists,
			wantErr:   ErrLoginTaken,
		},
		{
			name:      "repository failure",
			login:     "ivan",
			password:  "qwerty",
			createErr: errDB,
			wantErr:   errDB,
		},
		{
			name:     "successful registration",
			login:    "ivan",
			password: "qwerty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			users := newFakeUserRepository()
			users.createErr = tt.createErr
			service := NewUserService(users, newTestTokenManager())

			token, err := service.Register(context.Background(), tt.login, tt.password)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("Register() error = %v, want %v", err, tt.wantErr)
			}
			if tt.wantErr != nil {
				return
			}

			if token == "" {
				t.Error("Register() returned an empty token")
			}
			if users.created.PasswordHash == tt.password {
				t.Error("Register() stored the raw password instead of a hash")
			}
		})
	}
}

func TestLogin(t *testing.T) {
	errDB := errors.New("db is down")

	const (
		login    = "ivan"
		password = "qwerty"
	)

	passwordHash, err := auth.HashPassword(password)
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}
	storedUser := models.User{ID: 1, Login: login, PasswordHash: passwordHash}

	tests := []struct {
		name     string
		users    map[string]models.User
		getErr   error
		password string
		wantErr  error
	}{
		{
			name:     "user not found",
			users:    map[string]models.User{},
			password: password,
			wantErr:  ErrInvalidCredentials,
		},
		{
			name:     "wrong password",
			users:    map[string]models.User{login: storedUser},
			password: "wrong",
			wantErr:  ErrInvalidCredentials,
		},
		{
			name:     "repository failure",
			users:    map[string]models.User{},
			getErr:   errDB,
			password: password,
			wantErr:  errDB,
		},
		{
			name:     "successful login",
			users:    map[string]models.User{login: storedUser},
			password: password,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			users := &fakeUserRepository{users: tt.users, getErr: tt.getErr}
			service := NewUserService(users, newTestTokenManager())

			token, err := service.Login(context.Background(), login, tt.password)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("Login() error = %v, want %v", err, tt.wantErr)
			}
			if tt.wantErr == nil && token == "" {
				t.Error("Login() returned an empty token")
			}
		})
	}
}
