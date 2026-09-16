package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/ilushka-off/go-musthave-diploma-tpl/internal/luhn"
	"github.com/ilushka-off/go-musthave-diploma-tpl/internal/models"
	"github.com/ilushka-off/go-musthave-diploma-tpl/internal/storage"
)

// Ошибки загрузки номера заказа.
var (
	ErrInvalidOrderNumber      = errors.New("invalid order number")
	ErrOrderOwnedByAnotherUser = errors.New("order owned by another user")
)

// OrderService реализует приём и выдачу номеров заказов пользователя.
type OrderService struct {
	orders storage.OrderRepository
}

// NewOrderService создаёт OrderService поверх хранилища заказов.
func NewOrderService(orders storage.OrderRepository) *OrderService {
	return &OrderService{
		orders: orders,
	}
}

// UploadOrder принимает номер заказа от пользователя. Возвращает true, если
// заказ принят в обработку впервые, и false, если этот же пользователь уже
// загружал его ранее. Возвращает ErrInvalidOrderNumber, если номер не проходит
// проверку по алгоритму Луна, и ErrOrderOwnedByAnotherUser, если номер занят
// другим пользователем.
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

// ListOrders возвращает заказы пользователя от самых новых к самым старым.
func (s *OrderService) ListOrders(ctx context.Context, userID int) ([]models.Order, error) {
	orders, err := s.orders.GetOrdersByUserID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("list orders: %w", err)
	}
	return orders, nil
}
