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

var _ storage.UserRepository = (*UserRepository)(nil)

// UserRepository хранит пользователей в PostgreSQL.
type UserRepository struct {
	pool *pgxpool.Pool
}

// NewUserRepository создаёт UserRepository поверх пула соединений.
func NewUserRepository(pool *pgxpool.Pool) *UserRepository {
	return &UserRepository{
		pool: pool,
	}
}

// CreateUser сохраняет пользователя и возвращает его идентификатор.
// Если логин уже занят, возвращает storage.ErrLoginExists.
func (r *UserRepository) CreateUser(ctx context.Context, user models.User) (int, error) {

	var userID int

	query := `INSERT INTO users(login, password_hash) VALUES ($1, $2) RETURNING id`
	err := r.pool.QueryRow(ctx, query, user.Login, user.PasswordHash).Scan(&userID)
	if pgErr, ok := errors.AsType[*pgconn.PgError](err); ok && pgErr.Code == "23505" {
		return 0, storage.ErrLoginExists
	}
	if err != nil {
		return 0, fmt.Errorf("insert user: %w", err)
	}
	return userID, nil

}

// GetUserByLogin возвращает пользователя по логину
// или storage.ErrUserNotFound, если такого нет.
func (r *UserRepository) GetUserByLogin(ctx context.Context, login string) (models.User, error) {

	var user models.User

	query := `SELECT id, login, password_hash, balance FROM users WHERE login = $1`
	err := r.pool.QueryRow(ctx, query, login).Scan(&user.ID, &user.Login, &user.PasswordHash, &user.Balance)

	if errors.Is(err, pgx.ErrNoRows) {
		return models.User{}, storage.ErrUserNotFound
	}

	if err != nil {
		return models.User{}, fmt.Errorf("select user: %w", err)
	}

	return user, nil
}

// GetCurrentBalanceByUserID возвращает текущий остаток баллов пользователя
// или storage.ErrUserNotFound, если такого нет.
func (r *UserRepository) GetCurrentBalanceByUserID(ctx context.Context, userID int) (decimal.Decimal, error) {
	var balance decimal.Decimal

	query := `SELECT balance FROM users WHERE id = $1`
	err := r.pool.QueryRow(ctx, query, userID).Scan(&balance)

	if errors.Is(err, pgx.ErrNoRows) {
		return balance, storage.ErrUserNotFound
	}

	if err != nil {
		return balance, fmt.Errorf("select user balance: %w", err)
	}

	return balance, nil
}
