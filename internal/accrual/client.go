package accrual

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/ilushka-off/go-musthave-diploma-tpl/internal/models"
	"github.com/shopspring/decimal"
)

// ErrOrderNotRegistered возвращается, когда заказ не зарегистрирован
// в системе расчёта начислений.
var (
	ErrOrderNotRegistered = errors.New("order is not registered in accrual system")
)

type clientResponse struct {
	Order   string           `json:"order"`
	Status  string           `json:"status"`
	Accrual *decimal.Decimal `json:"accrual,omitempty"`
}

// Result — результат расчёта по заказу. Accrual равен nil, если начисление
// не положено или ещё не рассчитано.
type Result struct {
	Status  models.OrderStatus
	Accrual *decimal.Decimal
}

// Client — HTTP-клиент внешней системы расчёта начислений.
type Client struct {
	baseURL    string
	httpClient *http.Client
}

// RateLimitError сообщает о превышении лимита запросов к системе начислений
// и о том, через сколько её можно опрашивать снова.
type RateLimitError struct {
	RetryAfter time.Duration
}

// Error реализует интерфейс error.
func (rl *RateLimitError) Error() string {
	return fmt.Sprintf("rate limit exceeded, retry after %s", rl.RetryAfter)
}

// NewClient создаёт клиент системы начислений, доступной по адресу baseURL.
func NewClient(baseURL string) *Client {
	return &Client{
		baseURL:    baseURL,
		httpClient: &http.Client{Timeout: 5 * time.Second},
	}
}

func mapStatus(status string) (models.OrderStatus, error) {
	switch status {
	case "REGISTERED", "PROCESSING":
		return models.Processing, nil
	case "INVALID":
		return models.Invalid, nil
	case "PROCESSED":
		return models.Processed, nil
	default:
		return "", fmt.Errorf("unknown accrual status %q", status)
	}
}

// GetOrderAccrual запрашивает расчёт по номеру заказа. Возвращает
// ErrOrderNotRegistered, если заказ системе неизвестен, и *RateLimitError
// при превышении лимита запросов.
func (c *Client) GetOrderAccrual(ctx context.Context, number string) (Result, error) {
	endpoint, err := url.JoinPath(c.baseURL, "api", "orders", number)
	if err != nil {
		return Result{}, fmt.Errorf("build url: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return Result{}, fmt.Errorf("create request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return Result{}, fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK:

		var body clientResponse

		if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
			return Result{}, fmt.Errorf("decode response: %w", err)
		}

		status, err := mapStatus(body.Status)
		if err != nil {
			return Result{}, err
		}

		return Result{
			Accrual: body.Accrual,
			Status:  status,
		}, nil
	case http.StatusNoContent:
		return Result{}, ErrOrderNotRegistered
	case http.StatusTooManyRequests:
		seconds, err := strconv.Atoi(resp.Header.Get("Retry-After"))
		if err != nil || seconds <= 0 {
			seconds = 60
		}
		return Result{}, &RateLimitError{
			RetryAfter: time.Duration(seconds) * time.Second,
		}
	default:
		return Result{}, fmt.Errorf("unexpected status code %d", resp.StatusCode)
	}
}
