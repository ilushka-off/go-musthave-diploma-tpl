package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/ilushka-off/go-musthave-diploma-tpl/internal/middleware"
	"github.com/ilushka-off/go-musthave-diploma-tpl/internal/models"
	"github.com/ilushka-off/go-musthave-diploma-tpl/internal/service"
	"go.uber.org/zap"
)

type BalanceHandler struct {
	balanceService *service.BalanceService
	logger         *zap.Logger
}

func NewBalanceHandler(balanceService *service.BalanceService, logger *zap.Logger) *BalanceHandler {
	return &BalanceHandler{
		balanceService: balanceService,
		logger:         logger,
	}
}

func toWithdrawalResponse(withdrawal models.Withdrawal) WithdrawalResponseDTO {
	return WithdrawalResponseDTO{
		Order:       withdrawal.Order,
		Sum:         withdrawal.Sum,
		ProcessedAt: withdrawal.ProcessedAt.Format(time.RFC3339),
	}
}

func (h *BalanceHandler) Get(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.UserIDFromContext(r.Context())
	if !ok {
		http.Error(w, "user unauthorized", http.StatusUnauthorized)
		return
	}
	balance, err := h.balanceService.GetBalance(r.Context(), userID)
	if err != nil {
		h.logger.Error("get balance", zap.Error(err))
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")

	response := BalanceResponseDTO{
		Current:   balance.Current,
		Withdrawn: balance.Withdrawn,
	}

	err = json.NewEncoder(w).Encode(response)
	if err != nil {
		h.logger.Error("encode balance", zap.Error(err))
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
}

func (h *BalanceHandler) Withdraw(w http.ResponseWriter, r *http.Request) {
	var req WithdrawRequestDTO

	userID, ok := middleware.UserIDFromContext(r.Context())
	if !ok {
		http.Error(w, "user unauthorized", http.StatusUnauthorized)
		return
	}

	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	err = h.balanceService.Withdraw(r.Context(), userID, req.Order, req.Sum)

	switch {
	case errors.Is(err, service.ErrInvalidOrderNumber), errors.Is(err, service.ErrInvalidWithdrawalSum):
		http.Error(w, err.Error(), http.StatusUnprocessableEntity)
		return
	case errors.Is(err, service.ErrInsufficientFunds):
		http.Error(w, "insufficient funds", http.StatusPaymentRequired)
		return
	case err != nil:
		h.logger.Error("withdraw", zap.Error(err))
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (h *BalanceHandler) ListWithdrawals(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.UserIDFromContext(r.Context())
	if !ok {
		http.Error(w, "user unauthorized", http.StatusUnauthorized)
		return
	}
	withdrawals, err := h.balanceService.ListWithdrawals(r.Context(), userID)
	if err != nil {
		h.logger.Error("list withdrawals", zap.Error(err))
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	if len(withdrawals) == 0 {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	withdrawalList := make([]WithdrawalResponseDTO, 0, len(withdrawals))

	for _, withdrawal := range withdrawals {
		withdrawalList = append(withdrawalList, toWithdrawalResponse(withdrawal))
	}
	w.Header().Set("Content-Type", "application/json")
	err = json.NewEncoder(w).Encode(withdrawalList)
	if err != nil {
		h.logger.Error("encode withdrawals", zap.Error(err))
	}
}
