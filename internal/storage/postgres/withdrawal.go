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

var _ storage.WithdrawalRepository = (*WithdrawalRepository)(nil)

// WithdrawalRepository хранит списания баллов в PostgreSQL.
type WithdrawalRepository struct {
	pool *pgxpool.Pool
}

// NewWithdrawalRepository создаёт WithdrawalRepository поверх пула соединений.
func NewWithdrawalRepository(pool *pgxpool.Pool) *WithdrawalRepository {
	return &WithdrawalRepository{
		pool: pool,
	}
}

// CreateWithdrawal в одной транзакции списывает баллы со счёта пользователя
// и записывает факт списания. Если баллов не хватает, возвращает
// storage.ErrUserInsufficientFunds и счёт не трогает.
func (r *WithdrawalRepository) CreateWithdrawal(ctx context.Context, userID int, order string, sum decimal.Decimal) (int, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return 0, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	query := `SELECT balance FROM users WHERE id = $1 FOR UPDATE`

	var currentBalance decimal.Decimal

	err = tx.QueryRow(ctx, query, userID).Scan(&currentBalance)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, storage.ErrUserInsufficientFunds
	}
	if err != nil {
		return 0, fmt.Errorf("lock user balance: %w", err)
	}
	if currentBalance.LessThan(sum) {
		return 0, storage.ErrUserInsufficientFunds
	}

	query = `UPDATE users SET balance = balance - $1 WHERE id = $2`

	if _, err = tx.Exec(ctx, query, sum, userID); err != nil {
		return 0, fmt.Errorf("debit balance: %w", err)
	}

	query = `INSERT INTO withdrawals(user_id, "order", sum) VALUES ($1, $2, $3) RETURNING id`

	var withdrawalID int

	err = tx.QueryRow(ctx, query, userID, order, sum).Scan(&withdrawalID)
	if err != nil {
		return 0, fmt.Errorf("insert withdrawal: %w", err)
	}

	if err = tx.Commit(ctx); err != nil {
		return 0, fmt.Errorf("commit tx: %w", err)
	}

	return withdrawalID, nil
}

// GetWithdrawalsByUserID возвращает списания пользователя от самых новых
// к самым старым.
func (r *WithdrawalRepository) GetWithdrawalsByUserID(ctx context.Context, userID int) ([]models.Withdrawal, error) {
	var withdrawals []models.Withdrawal

	query := `SELECT id, user_id, "order", sum, processed_at FROM withdrawals WHERE user_id = $1 ORDER BY processed_at DESC, id DESC`
	rows, err := r.pool.Query(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("select withdrawal: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var withdrawal models.Withdrawal
		if err := rows.Scan(&withdrawal.ID, &withdrawal.UserID, &withdrawal.Order, &withdrawal.Sum, &withdrawal.ProcessedAt); err != nil {
			return nil, fmt.Errorf("scan withdrawal: %w", err)
		}
		withdrawals = append(withdrawals, withdrawal)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate withdrawals: %w", err)
	}
	return withdrawals, nil
}

// GetWithdrawnByUserID возвращает сумму всех списаний пользователя.
func (r *WithdrawalRepository) GetWithdrawnByUserID(ctx context.Context, userID int) (decimal.Decimal, error) {
	var withdrawn decimal.Decimal

	query := `SELECT COALESCE(SUM(sum), 0) FROM withdrawals WHERE user_id = $1`

	err := r.pool.QueryRow(ctx, query, userID).Scan(&withdrawn)
	if err != nil {
		return decimal.Decimal{}, fmt.Errorf("select withdrawn sum: %w", err)
	}
	return withdrawn, nil
}

// GetBalance возвращает текущий остаток и сумму списаний пользователя из
// одного снапшота REPEATABLE READ, чтобы конкурентное списание не могло
// разъехаться между двумя величинами. При ошибке сериализации повторяет
// попытку.
func (r *WithdrawalRepository) GetBalance(ctx context.Context, userID int) (decimal.Decimal, decimal.Decimal, error) {
	for {
		current, withdrawn, err := r.getBalanceOnce(ctx, userID)
		if pgErr, ok := errors.AsType[*pgconn.PgError](err); ok && pgErr.Code == "40001" {
			continue
		}
		return current, withdrawn, err
	}
}

func (r *WithdrawalRepository) getBalanceOnce(ctx context.Context, userID int) (decimal.Decimal, decimal.Decimal, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead})
	if err != nil {
		return decimal.Decimal{}, decimal.Decimal{}, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	var current decimal.Decimal
	if err := tx.QueryRow(ctx, `SELECT balance FROM users WHERE id = $1`, userID).Scan(&current); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return decimal.Decimal{}, decimal.Decimal{}, storage.ErrUserNotFound
		}
		return decimal.Decimal{}, decimal.Decimal{}, fmt.Errorf("select balance: %w", err)
	}

	var withdrawn decimal.Decimal
	if err := tx.QueryRow(ctx, `SELECT COALESCE(SUM(sum), 0) FROM withdrawals WHERE user_id = $1`, userID).Scan(&withdrawn); err != nil {
		return decimal.Decimal{}, decimal.Decimal{}, fmt.Errorf("select withdrawn: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return decimal.Decimal{}, decimal.Decimal{}, fmt.Errorf("commit tx: %w", err)
	}

	return current, withdrawn, nil
}
