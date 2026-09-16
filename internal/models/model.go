// Package models содержит доменные сущности системы лояльности.
package models

import (
	"time"

	"github.com/shopspring/decimal"
)

// User — зарегистрированный пользователь и его накопительный счёт.
type User struct {
	ID           int             `json:"id"`
	Login        string          `json:"login"`
	PasswordHash string          `json:"password_hash"`
	Balance      decimal.Decimal `json:"balance"`
}

// Order — номер заказа, загруженный пользователем, и результат его расчёта.
// Accrual равен nil, пока начисление не рассчитано или не положено.
type Order struct {
	ID         int              `json:"id"`
	UserID     int              `json:"user_id"`
	Number     string           `json:"number"`
	Status     OrderStatus      `json:"status"`
	Accrual    *decimal.Decimal `json:"accrual,omitempty"`
	UploadedAt time.Time        `json:"uploaded_at"`
}

// Withdrawal — факт списания баллов в счёт оплаты заказа.
type Withdrawal struct {
	ID          int             `json:"id"`
	UserID      int             `json:"user_id"`
	Order       string          `json:"order"`
	Sum         decimal.Decimal `json:"sum"`
	ProcessedAt time.Time       `json:"processed_at"`
}

// OrderStatus — статус расчёта начисления по заказу.
type OrderStatus string

// Статусы обработки расчёта. Invalid и Processed окончательны.

const (
	New        OrderStatus = "NEW"
	Processing OrderStatus = "PROCESSING"
	Invalid    OrderStatus = "INVALID"
	Processed  OrderStatus = "PROCESSED"
)
