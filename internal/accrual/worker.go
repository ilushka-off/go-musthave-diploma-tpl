package accrual

import (
	"context"
	"errors"
	"time"

	"github.com/ilushka-off/go-musthave-diploma-tpl/internal/storage"
	"go.uber.org/zap"
)

type Worker struct {
	orderRepository storage.OrderRepository
	logger          *zap.Logger
	client          *Client
	pollInterval    time.Duration
}

func NewWorker(orderRepository storage.OrderRepository, logger *zap.Logger, client *Client, pollInterval time.Duration) *Worker {
	return &Worker{
		orderRepository: orderRepository,
		logger:          logger,
		client:          client,
		pollInterval:    pollInterval,
	}
}

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
		result, err := w.client.GetOrderAccrual(ctx, order.Number)
		if rlErr, ok := errors.AsType[*RateLimitError](err); ok {
			w.logger.Warn("accrual rate limit, pausing", zap.Duration("retry_after", rlErr.RetryAfter))
			select {
			case <-ctx.Done():
				return
			case <-time.After(rlErr.RetryAfter):
				return
			}
		}
		if errors.Is(err, ErrOrderNotRegistered) {
			continue
		}
		if err != nil {
			w.logger.Error("get order accrual", zap.Error(err))
			continue
		}
		err = w.orderRepository.UpdateStatusByNumber(ctx, order.Number, result.Accrual, result.Status)
		if err != nil {
			w.logger.Error("update order", zap.Error(err))
		}
	}
}
