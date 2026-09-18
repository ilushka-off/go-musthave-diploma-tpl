package handlers

import "github.com/shopspring/decimal"

// UserDTO — пара логин/пароль в запросах регистрации и аутентификации.
type UserDTO struct {
	Login    string `json:"login"`
	Password string `json:"password"`
}

// OrderDTO — заказ в ответе GET /api/user/orders. Accrual опускается,
// если начисление по заказу не рассчитано или не положено.
type OrderDTO struct {
	Number     string           `json:"number"`
	Status     string           `json:"status"`
	Accrual    *decimal.Decimal `json:"accrual,omitempty"`
	UploadedAt string           `json:"uploaded_at"`
}

// BalanceResponseDTO — ответ GET /api/user/balance.
type BalanceResponseDTO struct {
	Current   decimal.Decimal `json:"current"`
	Withdrawn decimal.Decimal `json:"withdrawn"`
}

// WithdrawRequestDTO — запрос POST /api/user/balance/withdraw.
type WithdrawRequestDTO struct {
	Order string          `json:"order"`
	Sum   decimal.Decimal `json:"sum"`
}

// WithdrawalResponseDTO — списание в ответе GET /api/user/withdrawals.
type WithdrawalResponseDTO struct {
	Order       string          `json:"order"`
	Sum         decimal.Decimal `json:"sum"`
	ProcessedAt string          `json:"processed_at"`
}
