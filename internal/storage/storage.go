package storage

import (
	"context"
	"errors"

	"github.com/ilushka-off/go-musthave-diploma-tpl/internal/models"
	"github.com/shopspring/decimal"
)

var (
	ErrUserNotFound          = errors.New("user not found")
	ErrLoginExists           = errors.New("user with this login already exists")
	ErrUserInsufficientFunds = errors.New("insufficient funds in the account")
	ErrOrderNotFound         = errors.New("order not found")
	ErrOrderExists           = errors.New("order with this number already exists")
)

type UserRepository interface {
	CreateUser(ctx context.Context, user models.User) (int, error)
	GetUserByLogin(ctx context.Context, login string) (models.User, error)
	GetCurrentBalanceByUserID(ctx context.Context, userID int) (decimal.Decimal, error)
}

type OrderRepository interface {
	CreateOrder(ctx context.Context, userID int, number string, status models.OrderStatus) (int, error)
	GetOrderByNumber(ctx context.Context, number string) (models.Order, error)
	GetOrdersByUserID(ctx context.Context, userID int) ([]models.Order, error)
	UpdateStatusByNumber(ctx context.Context, number string, accrual *decimal.Decimal, status models.OrderStatus) error
	GetPendingOrders(ctx context.Context) ([]models.Order, error)
}

type WithdrawalRepository interface {
	CreateWithdrawal(ctx context.Context, userID int, order string, sum decimal.Decimal) (int, error)
	GetWithdrawalsByUserID(ctx context.Context, userID int) ([]models.Withdrawal, error)
	GetWithdrawnByUserID(ctx context.Context, userID int) (decimal.Decimal, error)
}
