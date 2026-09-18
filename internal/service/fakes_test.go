package service

import (
	"context"

	"github.com/ilushka-off/go-musthave-diploma-tpl/internal/models"
	"github.com/ilushka-off/go-musthave-diploma-tpl/internal/storage"
	"github.com/shopspring/decimal"
)

var (
	_ storage.UserRepository       = (*fakeUserRepository)(nil)
	_ storage.OrderRepository      = (*fakeOrderRepository)(nil)
	_ storage.WithdrawalRepository = (*fakeWithdrawalRepository)(nil)
)

type fakeUserRepository struct {
	users     map[string]models.User
	createErr error
	getErr    error
	created   models.User
}

func newFakeUserRepository() *fakeUserRepository {
	return &fakeUserRepository{users: map[string]models.User{}}
}

func (f *fakeUserRepository) CreateUser(_ context.Context, user models.User) (int, error) {
	if f.createErr != nil {
		return 0, f.createErr
	}
	if _, ok := f.users[user.Login]; ok {
		return 0, storage.ErrLoginExists
	}

	f.created = user
	user.ID = len(f.users) + 1
	f.users[user.Login] = user

	return user.ID, nil
}

func (f *fakeUserRepository) GetUserByLogin(_ context.Context, login string) (models.User, error) {
	if f.getErr != nil {
		return models.User{}, f.getErr
	}
	user, ok := f.users[login]
	if !ok {
		return models.User{}, storage.ErrUserNotFound
	}
	return user, nil
}

func (f *fakeUserRepository) GetCurrentBalanceByUserID(_ context.Context, _ int) (decimal.Decimal, error) {
	if f.getErr != nil {
		return decimal.Decimal{}, f.getErr
	}
	return decimal.Decimal{}, nil
}

type fakeOrderRepository struct {
	orders    map[string]models.Order
	byUser    []models.Order
	createErr error
	listErr   error
}

func (f *fakeOrderRepository) CreateOrder(_ context.Context, userID int, number string, status models.OrderStatus) (int, error) {
	if f.createErr != nil {
		return 0, f.createErr
	}
	if _, ok := f.orders[number]; ok {
		return 0, storage.ErrOrderExists
	}

	order := models.Order{
		ID:     len(f.orders) + 1,
		UserID: userID,
		Number: number,
		Status: status,
	}
	f.orders[number] = order

	return order.ID, nil
}

func (f *fakeOrderRepository) GetOrderByNumber(_ context.Context, number string) (models.Order, error) {
	order, ok := f.orders[number]
	if !ok {
		return models.Order{}, storage.ErrOrderNotFound
	}
	return order, nil
}

func (f *fakeOrderRepository) GetOrdersByUserID(_ context.Context, _ int) ([]models.Order, error) {
	if f.listErr != nil {
		return nil, f.listErr
	}
	return f.byUser, nil
}

func (f *fakeOrderRepository) UpdateStatusByNumber(_ context.Context, _ string, _ *decimal.Decimal, _ models.OrderStatus) error {
	panic("not used in these tests")
}

func (f *fakeOrderRepository) GetPendingOrders(_ context.Context) ([]models.Order, error) {
	panic("not used in these tests")
}

type fakeWithdrawalRepository struct {
	withdrawals []models.Withdrawal
	withdrawn   decimal.Decimal
	balance     decimal.Decimal
	withdrawErr error
	getErr      error
	listErr     error
	balanceErr  error
}

func (f *fakeWithdrawalRepository) CreateWithdrawal(_ context.Context, _ int, _ string, _ decimal.Decimal) (int, error) {
	if f.withdrawErr != nil {
		return 0, f.withdrawErr
	}
	return 1, nil
}

func (f *fakeWithdrawalRepository) GetWithdrawalsByUserID(_ context.Context, _ int) ([]models.Withdrawal, error) {
	if f.listErr != nil {
		return nil, f.listErr
	}
	return f.withdrawals, nil
}

func (f *fakeWithdrawalRepository) GetWithdrawnByUserID(_ context.Context, _ int) (decimal.Decimal, error) {
	if f.getErr != nil {
		return decimal.Decimal{}, f.getErr
	}
	return f.withdrawn, nil
}

func (f *fakeWithdrawalRepository) GetBalance(_ context.Context, _ int) (decimal.Decimal, decimal.Decimal, error) {
	if f.balanceErr != nil {
		return decimal.Decimal{}, decimal.Decimal{}, f.balanceErr
	}
	return f.balance, f.withdrawn, nil
}
