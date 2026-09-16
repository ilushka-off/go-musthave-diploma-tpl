// Package handlers реализует HTTP API накопительной системы лояльности.
package handlers

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/ilushka-off/go-musthave-diploma-tpl/internal/auth"
	"github.com/ilushka-off/go-musthave-diploma-tpl/internal/middleware"
	"go.uber.org/zap"
)

// NewRouter собирает HTTP-роутер сервиса. Публичны только регистрация
// и аутентификация, остальные хендлеры закрыты проверкой токена. Ко всем
// запросам применяются логирование, сжатие ответа и распаковка тела.
func NewRouter(userHandler *UserHandler, orderHandler *OrderHandler, balanceHandler *BalanceHandler, logger *zap.Logger, tokenManager *auth.TokenManager) http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.Logger(logger))
	r.Use(chimiddleware.Compress(5))
	r.Use(middleware.Decompress)

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
