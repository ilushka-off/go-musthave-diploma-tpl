package service

import (
	"context"
	"errors"
	"testing"

	"github.com/ilushka-off/go-musthave-diploma-tpl/internal/models"
	"github.com/shopspring/decimal"
)

func TestListOrders(t *testing.T) {
	errDB := errors.New("db is down")

	tests := []struct {
		name      string
		stored    []models.Order
		listErr   error
		wantCount int
		wantErr   error
	}{
		{
			name:      "no orders",
			wantCount: 0,
		},
		{
			name: "orders are returned as stored",
			stored: []models.Order{
				{Number: "9278923470", Status: models.Processed},
				{Number: "12345678903", Status: models.New},
			},
			wantCount: 2,
		},
		{
			name:    "repository failure",
			listErr: errDB,
			wantErr: errDB,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			orders := &fakeOrderRepository{byUser: tt.stored, listErr: tt.listErr}
			got, err := NewOrderService(orders).ListOrders(context.Background(), 1)

			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("error = %v, want %v", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(got) != tt.wantCount {
				t.Fatalf("got %d orders, want %d", len(got), tt.wantCount)
			}
			for i := range got {
				if got[i].Number != tt.stored[i].Number {
					t.Errorf("order %d = %q, want %q", i, got[i].Number, tt.stored[i].Number)
				}
			}
		})
	}
}

func TestListWithdrawals(t *testing.T) {
	errDB := errors.New("db is down")

	tests := []struct {
		name      string
		stored    []models.Withdrawal
		listErr   error
		wantCount int
		wantErr   error
	}{
		{
			name:      "no withdrawals",
			wantCount: 0,
		},
		{
			name: "withdrawals are returned as stored",
			stored: []models.Withdrawal{
				{Order: "2377225624", Sum: decimal.NewFromInt(500)},
			},
			wantCount: 1,
		},
		{
			name:    "repository failure",
			listErr: errDB,
			wantErr: errDB,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			withdrawals := &fakeWithdrawalRepository{withdrawals: tt.stored, listErr: tt.listErr}
			service := NewBalanceService(withdrawals)

			got, err := service.ListWithdrawals(context.Background(), 1)

			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("error = %v, want %v", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(got) != tt.wantCount {
				t.Fatalf("got %d withdrawals, want %d", len(got), tt.wantCount)
			}
			for i := range got {
				if got[i].Order != tt.stored[i].Order {
					t.Errorf("withdrawal %d = %q, want %q", i, got[i].Order, tt.stored[i].Order)
				}
			}
		})
	}
}
