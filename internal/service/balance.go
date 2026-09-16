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

// Ошибки операций с накопительным счётом.
var (
	ErrInsufficientFunds    = errors.New("insufficient funds")
	ErrInvalidWithdrawalSum = errors.New("withdrawal sum must be positive")
)

// balanceScale — число знаков после запятой, с которым баллы хранятся в БД.
const balanceScale = 2

// BalanceService реализует работу с накопительным счётом: баланс, списания
// и историю списаний.
type BalanceService struct {
	withdrawals storage.WithdrawalRepository
	users       storage.UserRepository
}

// Balance — текущий остаток баллов и сумма, списанная за всё время.
type Balance struct {
	Current   decimal.Decimal
	Withdrawn decimal.Decimal
}

// NewBalanceService создаёт BalanceService поверх хранилищ списаний и пользователей.
func NewBalanceService(withdrawals storage.WithdrawalRepository, users storage.UserRepository) *BalanceService {
	return &BalanceService{
		withdrawals: withdrawals,
		users:       users,
	}
}

// GetBalance возвращает текущий остаток баллов и сумму всех списаний пользователя.
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

// Withdraw списывает баллы в счёт оплаты заказа order. Сумма округляется до
// balanceScale знаков, чтобы остаток и история списаний не разошлись.
// Возвращает ErrInvalidOrderNumber для номера, не проходящего проверку по
// алгоритму Луна, ErrInvalidWithdrawalSum для неположительной суммы
// и ErrInsufficientFunds, если баллов на счету не хватает.
func (s *BalanceService) Withdraw(ctx context.Context, userID int, order string, sum decimal.Decimal) error {
	if !luhn.Valid(order) {
		return ErrInvalidOrderNumber
	}
	sum = sum.Round(balanceScale)
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

// ListWithdrawals возвращает списания пользователя от самых новых к самым старым.
func (s *BalanceService) ListWithdrawals(ctx context.Context, userID int) ([]models.Withdrawal, error) {
	withdrawals, err := s.withdrawals.GetWithdrawalsByUserID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("list withdrawals: %w", err)
	}
	return withdrawals, nil
}
