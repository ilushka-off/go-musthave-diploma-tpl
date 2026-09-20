// Package accrual связывает сервис лояльности с внешней системой расчёта
// начислений: опрашивает её по незавершённым заказам и сохраняет результат.
package accrual

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/ilushka-off/go-musthave-diploma-tpl/internal/models"
	"github.com/ilushka-off/go-musthave-diploma-tpl/internal/storage"
	"go.uber.org/zap"
)

const (
	maxServerErrorRetries = 3
	serverErrorRetryDelay = 2 * time.Second
)

// Worker периодически опрашивает систему расчёта начислений по заказам
// в статусах NEW и PROCESSING и обновляет их в хранилище.
type Worker struct {
	orderRepository storage.OrderRepository
	logger          *zap.Logger
	client          *Client
	pollInterval    time.Duration
	poolSize        int

	pauseMu     sync.Mutex
	pausedUntil time.Time
}

// NewWorker создаёт Worker, опрашивающий систему начислений раз в pollInterval.
func NewWorker(orderRepository storage.OrderRepository, logger *zap.Logger, client *Client, pollInterval time.Duration, poolSize int) *Worker {
	return &Worker{
		orderRepository: orderRepository,
		logger:          logger,
		client:          client,
		pollInterval:    pollInterval,
		poolSize:        poolSize,
	}
}

// awaitPause ждёт окончания активной паузы (если она есть) и сообщает,
// можно ли продолжать. false означает отмену ctx во время ожидания.
func (w *Worker) awaitPause(ctx context.Context) bool {
	w.pauseMu.Lock()
	until := w.pausedUntil
	w.pauseMu.Unlock()

	wait := time.Until(until)
	if wait <= 0 {
		return true
	}

	timer := time.NewTimer(wait)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}

// setPause продлевает общую паузу до until, если она позже текущей —
// не позволяет более раннему 429 сократить уже действующую паузу.
func (w *Worker) setPause(until time.Time) {
	w.pauseMu.Lock()
	defer w.pauseMu.Unlock()
	if until.After(w.pausedUntil) {
		w.pausedUntil = until
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
	if len(orders) == 0 {
		return
	}

	poolSize := w.poolSize
	if poolSize <= 0 {
		poolSize = 1
	}
	if poolSize > len(orders) {
		poolSize = len(orders)
	}

	jobs := make(chan models.Order, poolSize)

	var wg sync.WaitGroup
	for i := 0; i < poolSize; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for order := range jobs {
				w.processOrder(ctx, order)
			}
		}()
	}

feed:
	for _, order := range orders {
		select {
		case jobs <- order:
		case <-ctx.Done():
			break feed
		}
	}
	close(jobs)

	wg.Wait()
}

func (w *Worker) processOrder(ctx context.Context, order models.Order) {
	serverErrorRetries := 0
	for {
		if !w.awaitPause(ctx) {
			return
		}

		result, err := w.client.GetOrderAccrual(ctx, order.Number)
		if rlErr, ok := errors.AsType[*RateLimitError](err); ok {
			if rlErr.ParseErr != nil {
				w.logger.Warn("invalid retry-after header, using default", zap.Error(rlErr.ParseErr))
			}
			w.logger.Warn("accrual rate limit, pausing", zap.Duration("retry_after", rlErr.RetryAfter))
			w.setPause(time.Now().Add(rlErr.RetryAfter))
			continue
		}
		if svErr, ok := errors.AsType[*ServerError](err); ok {
			serverErrorRetries++
			if serverErrorRetries > maxServerErrorRetries {
				w.logger.Error("accrual server error, giving up", zap.Int("status_code", svErr.StatusCode), zap.Int("attempts", serverErrorRetries))
				return
			}
			w.logger.Warn("accrual server error, retrying", zap.Int("status_code", svErr.StatusCode), zap.Int("attempt", serverErrorRetries))

			timer := time.NewTimer(serverErrorRetryDelay)
			select {
			case <-ctx.Done():
				timer.Stop()
				return
			case <-timer.C:
			}
			continue
		}
		if errors.Is(err, ErrOrderNotRegistered) {
			return
		}
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			w.logger.Error("get order accrual", zap.Error(err))
			return
		}

		err = w.orderRepository.UpdateStatusByNumber(ctx, order.Number, result.Accrual, result.Status)
		if err != nil {
			w.logger.Error("update order", zap.Error(err))
		}
		return
	}
}
