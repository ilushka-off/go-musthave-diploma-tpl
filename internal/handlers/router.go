package handlers

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/ilushka-off/go-musthave-diploma-tpl/internal/auth"
	"github.com/ilushka-off/go-musthave-diploma-tpl/internal/middleware"
	"go.uber.org/zap"
)

func NewRouter(userHandler *UserHandler, orderHandler *OrderHandler, balanceHandler *BalanceHandler, logger *zap.Logger, tokenManager *auth.TokenManager) http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.Logger(logger))

	r.Post("/api/user/register", userHandler.Register)
	r.Post("/api/user/login", userHandler.Login)

	r.Group(func(r chi.Router) {
		r.Use(middleware.Auth(tokenManager))
		r.Post("/api/user/orders", orderHandler.Upload)
		r.Get("/api/user/orders", orderHandler.List)
		r.Post("/api/user/balance/withdraw", balanceHandler.Withdraw)
		r.Get("/api/user/balance", balanceHandler.Get)
		r.Get("/api/user/withdrawals", balanceHandler.ListWithdrawals)
	})

	return r
}
