package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/ilushka-off/go-musthave-diploma-tpl/internal/auth"
	"github.com/ilushka-off/go-musthave-diploma-tpl/internal/config"
	"github.com/ilushka-off/go-musthave-diploma-tpl/internal/handlers"
	"github.com/ilushka-off/go-musthave-diploma-tpl/internal/service"
	"github.com/ilushka-off/go-musthave-diploma-tpl/internal/storage/postgres"
)

const tokenTTL = 24 * time.Hour

func main() {
	err := run()
	if err != nil {
		log.Fatal(err)
	}
}

func run() error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	conf, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	pool, err := postgres.OpenPool(ctx, conf.DatabaseURI)
	if err != nil {
		return err
	}
	defer pool.Close()

	err = postgres.RunMigrations(conf.DatabaseURI)
	if err != nil {
		return err
	}

	userRepository := postgres.NewUserRepository(pool)
	tokenManager := auth.NewTokenManager([]byte(conf.JWTSecret), tokenTTL)
	userService := service.NewUserService(userRepository, tokenManager)
	userHandler := handlers.NewUserHandler(userService)

	router := handlers.NewRouter(userHandler)

	server := &http.Server{
		Addr:    conf.RunAddress,
		Handler: router,
	}

	errCh := make(chan error, 1)
	go func() {
		errCh <- server.ListenAndServe()
	}()

	select {
	case err := <-errCh:
		return fmt.Errorf("http server: %w", err)
	case <-ctx.Done():
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	return server.Shutdown(shutdownCtx)
}
