package handlers

import "github.com/shopspring/decimal"

type UserDTO struct {
	Login    string `json:"login"`
	Password string `json:"password"`
}

type OrderDTO struct {
	Number     string           `json:"number"`
	Status     string           `json:"status"`
	Accrual    *decimal.Decimal `json:"accrual,omitempty"`
	UploadedAt string           `json:"uploaded_at"`
}
