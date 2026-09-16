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
// RUN_ADDRESS, DATABASE_URI, ACCRUAL_SYSTEM_ADDRESS, JWT_SECRET. Переменные
// окружения имеют приоритет над флагами. Адрес системы начислений дополняется
// схемой http://, если она не указана. Если JWT_SECRET не задан, секрет
// генерируется случайно на время работы процесса.
func Load() (Config, error) {

	var cfg Config

	flag.StringVar(&cfg.RunAddress, "a", "localhost:8080", "HTTP server address")
	flag.StringVar(&cfg.DatabaseURI, "d", "", "Database URL connection")
	flag.StringVar(&cfg.AccrualSystemAddress, "r", "", "Accrual System Address")
	flag.Parse()

	if v := os.Getenv("RUN_ADDRESS"); v != "" {
		cfg.RunAddress = v
	}

	if v := os.Getenv("DATABASE_URI"); v != "" {
		cfg.DatabaseURI = v
	}

	if v := os.Getenv("ACCRUAL_SYSTEM_ADDRESS"); v != "" {
		cfg.AccrualSystemAddress = v
	}

	if v := os.Getenv("JWT_SECRET"); v != "" {
		cfg.JWTSecret = v
	}

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
