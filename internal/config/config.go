// Package config читает параметры запуска сервиса из флагов и переменных окружения.
package config

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"flag"
	"os"
	"strings"
)

// Config — параметры запуска сервиса накопительной системы лояльности.
type Config struct {
	RunAddress           string
	DatabaseURI          string
	AccrualSystemAddress string
	JWTSecret            string
}

// Ошибки конфигурации, при которых сервис не может быть запущен.
var (
	ErrDatabaseURINotFound          = errors.New("database uri not found")
	ErrAccrualSystemAddressNotFound = errors.New("accrual system address not found")
)

// Load собирает конфигурацию из флагов -a, -d, -r и переменных окружения
// RUN_ADDRESS, DATABASE_URI, ACCRUAL_SYSTEM_ADDRESS, JWT_SECRET. Явно
// переданный флаг имеет приоритет над переменной окружения, переменная
// окружения — над значением по умолчанию. Адрес системы начислений дополняется
// схемой http://, если она не указана. Если JWT_SECRET не задан, секрет
// генерируется случайно на время работы процесса.
func Load() (Config, error) {

	var cfg Config

	runAddress := "localhost:8080"
	if v, ok := os.LookupEnv("RUN_ADDRESS"); ok {
		runAddress = v
	}

	var databaseURI string
	if v, ok := os.LookupEnv("DATABASE_URI"); ok {
		databaseURI = v
	}

	var accrualSystemAddress string
	if v, ok := os.LookupEnv("ACCRUAL_SYSTEM_ADDRESS"); ok {
		accrualSystemAddress = v
	}

	if v, ok := os.LookupEnv("JWT_SECRET"); ok {
		cfg.JWTSecret = v
	}

	flag.StringVar(&cfg.RunAddress, "a", runAddress, "HTTP server address")
	flag.StringVar(&cfg.DatabaseURI, "d", databaseURI, "Database URL connection")
	flag.StringVar(&cfg.AccrualSystemAddress, "r", accrualSystemAddress, "Accrual System Address")
	flag.Parse()

	if cfg.JWTSecret == "" {
		jwtSecret := make([]byte, 32)
		rand.Read(jwtSecret)
		cfg.JWTSecret = hex.EncodeToString(jwtSecret)

	}

	if cfg.DatabaseURI == "" {
		return Config{}, ErrDatabaseURINotFound
	}

	if cfg.AccrualSystemAddress == "" {
		return Config{}, ErrAccrualSystemAddressNotFound
	}

	if !strings.HasPrefix(cfg.AccrualSystemAddress, "http://") && !strings.HasPrefix(cfg.AccrualSystemAddress, "https://") {
		cfg.AccrualSystemAddress = "http://" + cfg.AccrualSystemAddress
	}

	return cfg, nil
}
