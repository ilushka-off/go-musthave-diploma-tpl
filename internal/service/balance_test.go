package service

import (
	"context"
	"errors"
	"testing"

	"github.com/ilushka-off/go-musthave-diploma-tpl/internal/storage"
	"github.com/shopspring/decimal"
)

func TestWithdraw(t *testing.T) {
	errDB := errors.New("db is down")

	const (
		validNumber   = "2377225624"
		invalidNumber = "12345678900"
	)

	tests := []struct {
		name        string
		number      string
		sum         decimal.Decimal
		withdrawErr error
		wantErr     error
	}{
		{
			name:    "invalid order number",
			number:  invalidNumber,
			sum:     decimal.NewFromInt(100),
			wantErr: ErrInvalidOrderNumber,
		},
		{
			name:    "zero sum",
			number:  validNumber,
			sum:     decimal.Zero,
			wantErr: ErrInvalidWithdrawalSum,
		},
		{
			name:    "negative sum",
			number:  validNumber,
			sum:     decimal.NewFromInt(-5),
			wantErr: ErrInvalidWithdrawalSum,
		},
		{
			name:        "insufficient funds",
			number:      validNumber,
			sum:         decimal.NewFromInt(100),
			withdrawErr: storage.ErrUserInsufficientFunds,
			wantErr:     ErrInsufficientFunds,
		},
		{
			name:        "repository failure",
			number:      validNumber,
			sum:         decimal.NewFromInt(100),
			withdrawErr: errDB,
			wantErr:     errDB,
		},
		{
			name:   "successful withdrawal",
			number: validNumber,
			sum:    decimal.NewFromInt(100),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			withdrawals := &fakeWithdrawalRepository{withdrawErr: tt.withdrawErr}
			service := NewBalanceService(withdrawals)

			err := service.Withdraw(context.Background(), 1, tt.number, tt.sum)
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("Withdraw(%q, %s) error = %v, want %v", tt.number, tt.sum, err, tt.wantErr)
			}
		})
	}
}

func TestGetBalance(t *testing.T) {
	wantCurrent := decimal.NewFromFloat(900.5)
	wantWithdrawn := decimal.NewFromInt(100)

	withdrawals := &fakeWithdrawalRepository{balance: wantCurrent, withdrawn: wantWithdrawn}

	service := NewBalanceService(withdrawals)

	balance, err := service.GetBalance(context.Background(), 1)
	if err != nil {
		t.Fatalf("GetBalance() unexpected error: %v", err)
	}
	if !balance.Current.Equal(wantCurrent) {
		t.Errorf("GetBalance() current = %s, want %s", balance.Current, wantCurrent)
	}
	if !balance.Withdrawn.Equal(wantWithdrawn) {
		t.Errorf("GetBalance() withdrawn = %s, want %s", balance.Withdrawn, wantWithdrawn)
	}
}

func TestGetBalanceRepositoryFailure(t *testing.T) {
	errDB := errors.New("db is down")

	service := NewBalanceService(&fakeWithdrawalRepository{balanceErr: errDB})

	if _, err := service.GetBalance(context.Background(), 1); !errors.Is(err, errDB) {
		t.Errorf("GetBalance() error = %v, want %v", err, errDB)
	}
}
