package models

import (
	"time"

	"github.com/shopspring/decimal"
)

type User struct {
	ID           int    `json:"id"`
	Login        string `json:"login"`
	PasswordHash string `json:"password_hash"`
}

type Order struct {
	ID         int              `json:"id"`
	UserID     int              `json:"user_id"`
	Number     string           `json:"number"`
	Status     OrderStatus      `json:"status"`
	Accrual    *decimal.Decimal `json:"accrual,omitempty"`
	UploadedAt time.Time        `json:"uploaded_at"`
}

type Withdrawal struct {
	ID          int             `json:"id"`
	UserID      int             `json:"user_id"`
	Order       string          `json:"order"`
	Sum         decimal.Decimal `json:"sum"`
	ProcessedAt time.Time       `json:"processed_at"`
}

type OrderStatus string

const (
	New        OrderStatus = "NEW"
	Processing OrderStatus = "PROCESSING"
	Invalid    OrderStatus = "INVALID"
	Processed  OrderStatus = "PROCESSED"
)
