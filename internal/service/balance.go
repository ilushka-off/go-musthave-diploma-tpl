package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/ilushka-off/go-musthave-diploma-tpl/internal/luhn"
	"github.com/ilushka-off/go-musthave-diploma-tpl/internal/models"
	"github.com/ilushka-off/go-musthave-diploma-tpl/internal/storage"
	"github.com/shopspring/decimal"
)

var (
	ErrInsufficientFunds    = errors.New("insufficient funds")
	ErrInvalidWithdrawalSum = errors.New("withdrawal sum must be positive")
)

type BalanceService struct {
	withdrawals storage.WithdrawalRepository
	users       storage.UserRepository
}

type Balance struct {
	Current   decimal.Decimal
	Withdrawn decimal.Decimal
}

func NewBalanceService(withdrawals storage.WithdrawalRepository, users storage.UserRepository) *BalanceService {
	return &BalanceService{
		withdrawals: withdrawals,
		users:       users,
	}
}

func (s *BalanceService) GetBalance(ctx context.Context, userID int) (Balance, error) {
	current, err := s.users.GetCurrentBalanceByUserID(ctx, userID)
	if err != nil {
		return Balance{}, fmt.Errorf("get current balance: %w", err)
	}
	withdrawn, err := s.withdrawals.GetWithdrawnByUserID(ctx, userID)
	if err != nil {
		return Balance{}, fmt.Errorf("get withdrawn sum: %w", err)
	}
	return Balance{
		Current:   current,
		Withdrawn: withdrawn,
	}, nil

}

func (s *BalanceService) Withdraw(ctx context.Context, userID int, order string, sum decimal.Decimal) error {
	if !luhn.Valid(order) {
		return ErrInvalidOrderNumber
	}
	if !sum.IsPositive() {
		return ErrInvalidWithdrawalSum
	}
	_, err := s.withdrawals.CreateWithdrawal(ctx, userID, order, sum)
	if errors.Is(err, storage.ErrUserInsufficientFunds) {
		return ErrInsufficientFunds
	}
	if err != nil {
		return fmt.Errorf("create withdrawal: %w", err)
	}
	return nil
}

func (s *BalanceService) ListWithdrawals(ctx context.Context, userID int) ([]models.Withdrawal, error) {
	withdrawals, err := s.withdrawals.GetWithdrawalsByUserID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("list withdrawals: %w", err)
	}
	return withdrawals, nil
}
