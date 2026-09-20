package handlers

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/ilushka-off/go-musthave-diploma-tpl/internal/auth"
	"github.com/ilushka-off/go-musthave-diploma-tpl/internal/service"
	"go.uber.org/zap"
)

// UserHandler обслуживает регистрацию и аутентификацию пользователей.
type UserHandler struct {
	userService *service.UserService
	logger      *zap.Logger
}

// NewUserHandler создаёт UserHandler поверх UserService.
func NewUserHandler(userService *service.UserService, logger *zap.Logger) *UserHandler {
	return &UserHandler{
		userService: userService,
		logger:      logger,
	}
}

func setAuthToken(w http.ResponseWriter, token string) {
	w.Header().Set("Authorization", "Bearer "+token)
	http.SetCookie(w, &http.Cookie{
		Name:     auth.CookieName,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
	})
}

// Register обслуживает POST /api/user/register. Отвечает 200 с токеном
// в заголовке Authorization и cookie, 400 при неверном формате запроса,
// 409 если логин занят, 500 при внутренней ошибке.
func (h *UserHandler) Register(w http.ResponseWriter, r *http.Request) {

	var req UserDTO

	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	token, err := h.userService.Register(r.Context(), req.Login, req.Password)
	switch {
	case errors.Is(err, service.ErrInvalidInput):
		http.Error(w, "invalid login or password format", http.StatusBadRequest)
		return
	case errors.Is(err, service.ErrLoginTaken):
		http.Error(w, "login already exists", http.StatusConflict)
		return
	case err != nil:
		h.logger.Error("register user", zap.Error(err))
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	setAuthToken(w, token)
	w.WriteHeader(http.StatusOK)
}

// Login обслуживает POST /api/user/login. Отвечает 200 с токеном в заголовке
// Authorization и cookie, 400 при неверном формате запроса, 401 при неверной
// паре логин/пароль, 500 при внутренней ошибке.
func (h *UserHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req UserDTO

	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	token, err := h.userService.Login(r.Context(), req.Login, req.Password)
	switch {
	case errors.Is(err, service.ErrInvalidCredentials):
		http.Error(w, "invalid login or password", http.StatusUnauthorized)
		return
	case err != nil:
		h.logger.Error("login user", zap.Error(err))
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	setAuthToken(w, token)
	w.WriteHeader(http.StatusOK)
}
