package auth

import (
	"errors"
	"testing"
	"time"
)

func TestIssueAndParse(t *testing.T) {
	const userID = 42

	manager := NewTokenManager([]byte("test-secret"), time.Hour)

	token, err := manager.Issue(userID)
	if err != nil {
		t.Fatalf("Issue() unexpected error: %v", err)
	}
	if token == "" {
		t.Fatal("Issue() returned an empty token")
	}

	got, err := manager.Parse(token)
	if err != nil {
		t.Fatalf("Parse() unexpected error: %v", err)
	}
	if got != userID {
		t.Errorf("Parse() userID = %d, want %d", got, userID)
	}
}

func TestParseRejectsInvalidTokens(t *testing.T) {
	const secret = "test-secret"

	manager := NewTokenManager([]byte(secret), time.Hour)

	validToken, err := manager.Issue(1)
	if err != nil {
		t.Fatalf("Issue() unexpected error: %v", err)
	}

	expiredManager := NewTokenManager([]byte(secret), -time.Hour)
	expiredToken, err := expiredManager.Issue(1)
	if err != nil {
		t.Fatalf("Issue() unexpected error: %v", err)
	}

	tests := []struct {
		name    string
		manager *TokenManager
		token   string
	}{
		{
			name:    "empty token",
			manager: manager,
			token:   "",
		},
		{
			name:    "garbage instead of a token",
			manager: manager,
			token:   "not-a-token",
		},
		{
			name:    "token signed with another secret",
			manager: NewTokenManager([]byte("another-secret"), time.Hour),
			token:   validToken,
		},
		{
			name:    "expired token",
			manager: manager,
			token:   expiredToken,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			userID, err := tt.manager.Parse(tt.token)
			if !errors.Is(err, ErrInvalidToken) {
				t.Errorf("Parse() error = %v, want %v", err, ErrInvalidToken)
			}
			if userID != 0 {
				t.Errorf("Parse() userID = %d, want 0 on error", userID)
			}
		})
	}
}
