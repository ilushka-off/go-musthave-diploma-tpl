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

type BalanceResponseDTO struct {
	Current   decimal.Decimal `json:"current"`
	Withdrawn decimal.Decimal `json:"withdrawn"`
}

type WithdrawRequestDTO struct {
	Order string          `json:"order"`
	Sum   decimal.Decimal `json:"sum"`
}

type WithdrawalResponseDTO struct {
	Order       string          `json:"order"`
	Sum         decimal.Decimal `json:"sum"`
	ProcessedAt string          `json:"processed_at"`
}
