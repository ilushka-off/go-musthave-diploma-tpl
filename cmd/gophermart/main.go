package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/ilushka-off/go-musthave-diploma-tpl/internal/accrual"
	"github.com/ilushka-off/go-musthave-diploma-tpl/internal/auth"
	"github.com/ilushka-off/go-musthave-diploma-tpl/internal/config"
	"github.com/ilushka-off/go-musthave-diploma-tpl/internal/handlers"
	"github.com/ilushka-off/go-musthave-diploma-tpl/internal/service"
	"github.com/ilushka-off/go-musthave-diploma-tpl/internal/storage/postgres"
	"github.com/shopspring/decimal"
	"go.uber.org/zap"
)

const (
	tokenTTL            = 24 * time.Hour
	accrualPollInterval = time.Second
)

func main() {
	err := run()
	if err != nil {
		log.Fatal(err)
	}
}

func run() error {
	decimal.MarshalJSONWithoutQuotes = true

	logger, err := zap.NewDevelopment()
	if err != nil {
		return fmt.Errorf("create logger: %w", err)
	}
	defer logger.Sync()

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

	tokenManager := auth.NewTokenManager([]byte(conf.JWTSecret), tokenTTL)

	userRepository := postgres.NewUserRepository(pool)
	userService := service.NewUserService(userRepository, tokenManager)
	userHandler := handlers.NewUserHandler(userService, logger)

	orderRepository := postgres.NewOrderRepository(pool)
	orderService := service.NewOrderService(orderRepository)
	orderHandler := handlers.NewOrderHandler(orderService, logger)

	withdrawalRepository := postgres.NewWithdrawalRepository(pool)
	balanceService := service.NewBalanceService(withdrawalRepository, userRepository)
	balanceHandler := handlers.NewBalanceHandler(balanceService, logger)

	router := handlers.NewRouter(userHandler, orderHandler, balanceHandler, logger, tokenManager)

	accrualClient := accrual.NewClient(conf.AccrualSystemAddress)

	var wg sync.WaitGroup

	workerCtx, cancelWorker := context.WithCancel(ctx)
	defer func() {
		cancelWorker()
		wg.Wait()
	}()
	worker := accrual.NewWorker(orderRepository, logger, accrualClient, accrualPollInterval)
	wg.Go(func() {
		worker.Run(workerCtx)
	})

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
