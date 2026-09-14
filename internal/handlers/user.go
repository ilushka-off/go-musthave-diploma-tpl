package handlers

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/ilushka-off/go-musthave-diploma-tpl/internal/service"
)

const authCookieName = "token"

type UserHandler struct {
	userService *service.UserService
}

func NewUserHandler(userService *service.UserService) *UserHandler {
	return &UserHandler{
		userService: userService,
	}
}

func setAuthToken(w http.ResponseWriter, token string) {
	w.Header().Set("Authorization", "Bearer "+token)
	http.SetCookie(w, &http.Cookie{
		Name:     authCookieName,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
	})
}

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
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	setAuthToken(w, token)
	w.WriteHeader(http.StatusOK)
}

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
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	setAuthToken(w, token)
	w.WriteHeader(http.StatusOK)
}
