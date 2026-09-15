package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/ilushka-off/go-musthave-diploma-tpl/internal/models"
	"github.com/ilushka-off/go-musthave-diploma-tpl/internal/storage"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"
)

var _ storage.OrderRepository = (*OrderRepository)(nil)

type OrderRepository struct {
	pool *pgxpool.Pool
}

func NewOrderRepository(pool *pgxpool.Pool) *OrderRepository {
	return &OrderRepository{
		pool: pool,
	}
}

func (r *OrderRepository) CreateOrder(ctx context.Context, userID int, number string, status models.OrderStatus) (int, error) {
	var orderID int

	query := `INSERT INTO orders(user_id, number, status) VALUES ($1, $2, $3) RETURNING id`
	err := r.pool.QueryRow(ctx, query, userID, number, string(status)).Scan(&orderID)
	if pgErr, ok := errors.AsType[*pgconn.PgError](err); ok && pgErr.Code == "23505" {
		return 0, storage.ErrOrderExists
	}
	if err != nil {
		return 0, fmt.Errorf("insert order: %w", err)
	}
	return orderID, nil
}

func (r *OrderRepository) GetOrderByNumber(ctx context.Context, number string) (models.Order, error) {
	var order models.Order

	query := `SELECT id, user_id, number, status, accrual, uploaded_at FROM orders WHERE number = $1`
	err := r.pool.QueryRow(ctx, query, number).Scan(&order.ID, &order.UserID, &order.Number, &order.Status, &order.Accrual, &order.UploadedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return models.Order{}, storage.ErrOrderNotFound
	}
	if err != nil {
		return models.Order{}, fmt.Errorf("select order: %w", err)
	}
	return order, nil
}

func (r *OrderRepository) GetOrdersByUserID(ctx context.Context, userID int) ([]models.Order, error) {
	var orders []models.Order

	query := `SELECT id, user_id, number, status, accrual, uploaded_at FROM orders WHERE user_id = $1 ORDER BY uploaded_at DESC, id DESC`
	rows, err := r.pool.Query(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("select orders: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var order models.Order
		if err := rows.Scan(&order.ID, &order.UserID, &order.Number, &order.Status, &order.Accrual, &order.UploadedAt); err != nil {
			return nil, fmt.Errorf("scan order: %w", err)
		}
		orders = append(orders, order)
	}
	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate orders: %w", err)
	}
	return orders, nil
}

func (r *OrderRepository) UpdateStatusByNumber(ctx context.Context, number string, accrual *decimal.Decimal, status models.OrderStatus) error {

	var userID int

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)
	query := `UPDATE orders SET status = $1, accrual = $2 WHERE number = $3 AND status IN ('NEW', 'PROCESSING') RETURNING user_id`

	err = tx.QueryRow(ctx, query, string(status), accrual, number).Scan(&userID)

	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("update order: %w", err)
	}

	if status == models.Processed && accrual != nil {
		query = `UPDATE users SET balance = balance + $1 WHERE id = $2`
		_, err = tx.Exec(ctx, query, accrual, userID)
		if err != nil {
			return fmt.Errorf("update users: %w", err)
		}
	}

	if err = tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit tx: %w", err)
	}

	return nil
}

func (r *OrderRepository) GetPendingOrders(ctx context.Context) ([]models.Order, error) {
	var orders []models.Order

	query := `SELECT id, user_id, number, status, accrual, uploaded_at FROM orders WHERE status IN ('NEW', 'PROCESSING') ORDER BY uploaded_at, id`
	rows, err := r.pool.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("select orders: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var order models.Order
		if err := rows.Scan(&order.ID, &order.UserID, &order.Number, &order.Status, &order.Accrual, &order.UploadedAt); err != nil {
			return nil, fmt.Errorf("scan order: %w", err)
		}
		orders = append(orders, order)
	}
	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate orders: %w", err)
	}
	return orders, nil

}
