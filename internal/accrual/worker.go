// Package accrual связывает сервис лояльности с внешней системой расчёта
// начислений: опрашивает её по незавершённым заказам и сохраняет результат.
package accrual

import (
	"context"
	"errors"
	"time"

	"github.com/ilushka-off/go-musthave-diploma-tpl/internal/models"
	"github.com/ilushka-off/go-musthave-diploma-tpl/internal/storage"
	"go.uber.org/zap"
)

// Worker периодически опрашивает систему расчёта начислений по заказам
// в статусах NEW и PROCESSING и обновляет их в хранилище.
type Worker struct {
	orderRepository storage.OrderRepository
	logger          *zap.Logger
	client          *Client
	pollInterval    time.Duration
}

// NewWorker создаёт Worker, опрашивающий систему начислений раз в pollInterval.
func NewWorker(orderRepository storage.OrderRepository, logger *zap.Logger, client *Client, pollInterval time.Duration) *Worker {
	return &Worker{
		orderRepository: orderRepository,
		logger:          logger,
		client:          client,
		pollInterval:    pollInterval,
	}
}

// Run запускает цикл опроса и возвращает управление после отмены ctx.
func (w *Worker) Run(ctx context.Context) {
	ticker := time.NewTicker(w.pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			w.processPending(ctx)
		}
	}
}

func (w *Worker) processPending(ctx context.Context) {
	orders, err := w.orderRepository.GetPendingOrders(ctx)
	if err != nil {
		w.logger.Error("get orders", zap.Error(err))
		return
	}

	for _, order := range orders {
		if !w.processOrder(ctx, order) {
			return
		}
	}
}

func (w *Worker) processOrder(ctx context.Context, order models.Order) bool {
	for {
		result, err := w.client.GetOrderAccrual(ctx, order.Number)
		if rlErr, ok := errors.AsType[*RateLimitError](err); ok {
			w.logger.Warn("accrual rate limit, pausing", zap.Duration("retry_after", rlErr.RetryAfter))

			timer := time.NewTimer(rlErr.RetryAfter)
			select {
			case <-ctx.Done():
				timer.Stop()
				return false
			case <-timer.C:
			}
			continue
		}
		if errors.Is(err, ErrOrderNotRegistered) {
			return true
		}
		if err != nil {
			if ctx.Err() != nil {
				return false
			}
			w.logger.Error("get order accrual", zap.Error(err))
			return true
		}

		err = w.orderRepository.UpdateStatusByNumber(ctx, order.Number, result.Accrual, result.Status)
		if err != nil {
			w.logger.Error("update order", zap.Error(err))
		}
		return true
	}
}
