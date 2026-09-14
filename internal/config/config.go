package config

import (
	"errors"
	"flag"
	"os"
	"strings"
)

type Config struct {
	RunAddress           string
	DatabaseURI          string
	AccrualSystemAddress string
}

var (
	ErrDatabaseURINotFound          = errors.New("database uri not found")
	ErrAccrualSystemAddressNotFound = errors.New("accrual system address not found")
)

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
