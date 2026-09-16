package handlers

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/ilushka-off/go-musthave-diploma-tpl/internal/middleware"
	"github.com/ilushka-off/go-musthave-diploma-tpl/internal/models"
	"github.com/ilushka-off/go-musthave-diploma-tpl/internal/service"
	"go.uber.org/zap"
)

// OrderHandler обслуживает загрузку и выдачу номеров заказов.
type OrderHandler struct {
	orderService *service.OrderService
	logger       *zap.Logger
}

// NewOrderHandler создаёт OrderHandler поверх OrderService.
func NewOrderHandler(orderService *service.OrderService, logger *zap.Logger) *OrderHandler {
	return &OrderHandler{
		orderService: orderService,
		logger:       logger,
	}
}

// Upload обслуживает POST /api/user/orders. Отвечает 202 на новый номер,
// 200 если этот же пользователь уже загружал его, 400 при пустом теле,
// 401 без аутентификации, 409 если номер занят другим пользователем,
// 422 при неверном формате номера, 500 при внутренней ошибке.
func (h *OrderHandler) Upload(w http.ResponseWriter, r *http.Request) {

	userID, ok := middleware.UserIDFromContext(r.Context())
	if !ok {
		http.Error(w, "user unauthorized", http.StatusUnauthorized)
		return
	}
	data, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	number := strings.TrimSpace(string(data))
	if number == "" {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	created, err := h.orderService.UploadOrder(r.Context(), userID, number)
	switch {
	case errors.Is(err, service.ErrInvalidOrderNumber):
		http.Error(w, "invalid order number", http.StatusUnprocessableEntity)
		return
	case errors.Is(err, service.ErrOrderOwnedByAnotherUser):
		http.Error(w, "order owned by another user", http.StatusConflict)
		return
	case err != nil:
		h.logger.Error("upload order", zap.Error(err))
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	if created {
		w.WriteHeader(http.StatusAccepted)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func toOrderDTO(order models.Order) OrderDTO {
	return OrderDTO{
		Number:     order.Number,
		Status:     string(order.Status),
		Accrual:    order.Accrual,
		UploadedAt: order.UploadedAt.Format(time.RFC3339),
	}
}

// List обслуживает GET /api/user/orders. Отвечает 200 со списком заказов
// от самых новых к самым старым, 204 если заказов нет, 401 без аутентификации,
// 500 при внутренней ошибке.
func (h *OrderHandler) List(w http.ResponseWriter, r *http.Request) {

	userID, ok := middleware.UserIDFromContext(r.Context())
	if !ok {
		http.Error(w, "user unauthorized", http.StatusUnauthorized)
		return
	}
	orders, err := h.orderService.ListOrders(r.Context(), userID)
	if err != nil {
		h.logger.Error("list orders", zap.Error(err))
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	if len(orders) == 0 {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	ordersList := make([]OrderDTO, 0, len(orders))

	for _, order := range orders {
		ordersList = append(ordersList, toOrderDTO(order))
	}
	w.Header().Set("Content-Type", "application/json")
	err = json.NewEncoder(w).Encode(ordersList)
	if err != nil {
		h.logger.Error("encode orders", zap.Error(err))
	}
}
