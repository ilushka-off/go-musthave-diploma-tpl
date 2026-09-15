package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/ilushka-off/go-musthave-diploma-tpl/internal/luhn"
	"github.com/ilushka-off/go-musthave-diploma-tpl/internal/models"
	"github.com/ilushka-off/go-musthave-diploma-tpl/internal/storage"
)

var (
	ErrInvalidOrderNumber      = errors.New("invalid order number")
	ErrOrderOwnedByAnotherUser = errors.New("order owned by another user")
)

type OrderService struct {
	orders storage.OrderRepository
}

func NewOrderService(orders storage.OrderRepository) *OrderService {
	return &OrderService{
		orders: orders,
	}
}

func (s *OrderService) UploadOrder(ctx context.Context, userID int, number string) (bool, error) {
	if !luhn.Valid(number) {
		return false, ErrInvalidOrderNumber
	}
	_, err := s.orders.CreateOrder(ctx, userID, number, models.New)
	if err == nil {
		return true, nil
	}
	if !errors.Is(err, storage.ErrOrderExists) {
		return false, fmt.Errorf("create order: %w", err)
	}
	order, err := s.orders.GetOrderByNumber(ctx, number)
	if err != nil {
		return false, fmt.Errorf("get order: %w", err)
	}
	if order.UserID != userID {
		return false, ErrOrderOwnedByAnotherUser
	}
	return false, nil
}

func (s *OrderService) ListOrders(ctx context.Context, userID int) ([]models.Order, error) {
	orders, err := s.orders.GetOrdersByUserID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("list orders: %w", err)
	}
	return orders, nil
}
