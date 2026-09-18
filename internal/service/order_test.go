package service

import (
	"context"
	"errors"
	"testing"

	"github.com/ilushka-off/go-musthave-diploma-tpl/internal/models"
)

func TestUploadOrder(t *testing.T) {
	errDB := errors.New("db is down")

	const (
		validNumber   = "12345678903"
		invalidNumber = "12345678900"
	)

	existingOrder := models.Order{ID: 1, UserID: 1, Number: validNumber, Status: models.New}

	tests := []struct {
		name        string
		existing    map[string]models.Order
		userID      int
		number      string
		createErr   error
		wantCreated bool
		wantErr     error
	}{
		{
			name:     "invalid order number",
			existing: map[string]models.Order{},
			userID:   1,
			number:   invalidNumber,
			wantErr:  ErrInvalidOrderNumber,
		},
		{
			name:        "new order accepted",
			existing:    map[string]models.Order{},
			userID:      1,
			number:      validNumber,
			wantCreated: true,
		},
		{
			name:     "same user uploads the number again",
			existing: map[string]models.Order{validNumber: existingOrder},
			userID:   1,
			number:   validNumber,
		},
		{
			name:     "number belongs to another user",
			existing: map[string]models.Order{validNumber: existingOrder},
			userID:   2,
			number:   validNumber,
			wantErr:  ErrOrderOwnedByAnotherUser,
		},
		{
			name:      "repository failure",
			existing:  map[string]models.Order{},
			userID:    1,
			number:    validNumber,
			createErr: errDB,
			wantErr:   errDB,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			orders := &fakeOrderRepository{orders: tt.existing, createErr: tt.createErr}
			service := NewOrderService(orders)

			created, err := service.UploadOrder(context.Background(), tt.userID, tt.number)
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("UploadOrder(%q) error = %v, want %v", tt.number, err, tt.wantErr)
			}
			if created != tt.wantCreated {
				t.Errorf("UploadOrder(%q) created = %v, want %v", tt.number, created, tt.wantCreated)
			}
		})
	}
}
