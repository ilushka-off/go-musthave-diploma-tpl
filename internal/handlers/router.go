package handlers

import "net/http"

func NewRouter(userHandler *UserHandler) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/user/register", userHandler.Register)
	mux.HandleFunc("POST /api/user/login", userHandler.Login)

	return mux
}
