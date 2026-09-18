// Package storage описывает контракты хранилищ и общие ошибки доступа к данным.
package storage

import (
	"context"
	"errors"

	"github.com/ilushka-off/go-musthave-diploma-tpl/internal/models"
	"github.com/shopspring/decimal"
)

// Ошибки хранилища, на которые реагирует слой сервисов.
var (
	ErrUserNotFound          = errors.New("user not found")
	ErrLoginExists           = errors.New("user with this login already exists")
	ErrUserInsufficientFunds = errors.New("insufficient funds in the account")
	ErrOrderNotFound         = errors.New("order not found")
	ErrOrderExists           = errors.New("order with this number already exists")
)

// UserRepository хранит пользователей и их текущий баланс.
type UserRepository interface {
	CreateUser(ctx context.Context, user models.User) (int, error)
	GetUserByLogin(ctx context.Context, login string) (models.User, error)
	GetCurrentBalanceByUserID(ctx context.Context, userID int) (decimal.Decimal, error)
}

// OrderRepository хранит заказы пользователей и статусы их расчёта.
type OrderRepository interface {
	CreateOrder(ctx context.Context, userID int, number string, status models.OrderStatus) (int, error)
	GetOrderByNumber(ctx context.Context, number string) (models.Order, error)
	GetOrdersByUserID(ctx context.Context, userID int) ([]models.Order, error)
	UpdateStatusByNumber(ctx context.Context, number string, accrual *decimal.Decimal, status models.OrderStatus) error
	GetPendingOrders(ctx context.Context) ([]models.Order, error)
}

// WithdrawalRepository хранит списания баллов с накопительных счетов.
type WithdrawalRepository interface {
	CreateWithdrawal(ctx context.Context, userID int, order string, sum decimal.Decimal) (int, error)
	GetWithdrawalsByUserID(ctx context.Context, userID int) ([]models.Withdrawal, error)
	GetWithdrawnByUserID(ctx context.Context, userID int) (decimal.Decimal, error)
	// GetBalance возвращает текущий остаток и сумму списаний одной консистентной
	// операцией: обе величины читаются из одного снапшота, чтобы конкурентное
	// списание не могло разъехаться между ними.
	GetBalance(ctx context.Context, userID int) (current, withdrawn decimal.Decimal, err error)
}
