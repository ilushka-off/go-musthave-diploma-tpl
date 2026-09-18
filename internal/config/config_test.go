package config

import (
	"errors"
	"flag"
	"os"
	"testing"
)

func TestLoad(t *testing.T) {
	tests := []struct {
		name        string
		args        []string
		env         map[string]string
		wantErr     error
		wantAddress string
		wantDB      string
		wantAccrual string
		wantSecret  string
	}{
		{
			name:        "flags only",
			args:        []string{"-a", ":9090", "-d", "postgres://localhost/db", "-r", "http://localhost:8080"},
			wantAddress: ":9090",
			wantDB:      "postgres://localhost/db",
			wantAccrual: "http://localhost:8080",
		},
		{
			name: "flags win over environment",
			args: []string{"-a", ":9090", "-d", "postgres://flag/db", "-r", "http://flag:8080"},
			env: map[string]string{
				"RUN_ADDRESS":            ":7070",
				"DATABASE_URI":           "postgres://env/db",
				"ACCRUAL_SYSTEM_ADDRESS": "http://env:8080",
			},
			wantAddress: ":9090",
			wantDB:      "postgres://flag/db",
			wantAccrual: "http://flag:8080",
		},
		{
			name: "environment wins over defaults",
			env: map[string]string{
				"RUN_ADDRESS":            ":7070",
				"DATABASE_URI":           "postgres://env/db",
				"ACCRUAL_SYSTEM_ADDRESS": "http://env:8080",
			},
			wantAddress: ":7070",
			wantDB:      "postgres://env/db",
			wantAccrual: "http://env:8080",
		},
		{
			name: "flags override environment selectively",
			args: []string{"-a", ":9090"},
			env: map[string]string{
				"RUN_ADDRESS":            ":7070",
				"DATABASE_URI":           "postgres://env/db",
				"ACCRUAL_SYSTEM_ADDRESS": "http://env:8080",
			},
			wantAddress: ":9090",
			wantDB:      "postgres://env/db",
			wantAccrual: "http://env:8080",
		},
		{
			name: "empty environment value is respected",
			env: map[string]string{
				"DATABASE_URI":           "postgres://env/db",
				"ACCRUAL_SYSTEM_ADDRESS": "http://env:8080",
				"RUN_ADDRESS":            "",
			},
			wantAddress: "",
			wantDB:      "postgres://env/db",
			wantAccrual: "http://env:8080",
		},
		{
			name:        "default run address",
			args:        []string{"-d", "postgres://localhost/db", "-r", "http://localhost:8080"},
			wantAddress: "localhost:8080",
			wantDB:      "postgres://localhost/db",
			wantAccrual: "http://localhost:8080",
		},
		{
			name:        "accrual address gets http scheme",
			args:        []string{"-d", "postgres://localhost/db", "-r", "localhost:8080"},
			wantAddress: "localhost:8080",
			wantDB:      "postgres://localhost/db",
			wantAccrual: "http://localhost:8080",
		},
		{
			name:        "https accrual address is kept",
			args:        []string{"-d", "postgres://localhost/db", "-r", "https://accrual.test"},
			wantAddress: "localhost:8080",
			wantDB:      "postgres://localhost/db",
			wantAccrual: "https://accrual.test",
		},
		{
			name:        "jwt secret from environment",
			args:        []string{"-d", "postgres://localhost/db", "-r", "http://localhost:8080"},
			env:         map[string]string{"JWT_SECRET": "from-env"},
			wantAddress: "localhost:8080",
			wantDB:      "postgres://localhost/db",
			wantAccrual: "http://localhost:8080",
			wantSecret:  "from-env",
		},
		{
			name:    "missing database uri",
			args:    []string{"-r", "http://localhost:8080"},
			wantErr: ErrDatabaseURINotFound,
		},
		{
			name:    "missing accrual address",
			args:    []string{"-d", "postgres://localhost/db"},
			wantErr: ErrAccrualSystemAddressNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for _, name := range []string{"RUN_ADDRESS", "DATABASE_URI", "ACCRUAL_SYSTEM_ADDRESS", "JWT_SECRET"} {
				t.Setenv(name, "")
				os.Unsetenv(name)
			}
			for name, value := range tt.env {
				t.Setenv(name, value)
			}

			originalArgs := os.Args
			originalFlags := flag.CommandLine
			t.Cleanup(func() {
				os.Args = originalArgs
				flag.CommandLine = originalFlags
			})

			os.Args = append([]string{"gophermart"}, tt.args...)
			flag.CommandLine = flag.NewFlagSet("gophermart", flag.ContinueOnError)

			cfg, err := Load()

			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("error = %v, want %v", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if cfg.RunAddress != tt.wantAddress {
				t.Errorf("RunAddress = %q, want %q", cfg.RunAddress, tt.wantAddress)
			}
			if cfg.DatabaseURI != tt.wantDB {
				t.Errorf("DatabaseURI = %q, want %q", cfg.DatabaseURI, tt.wantDB)
			}
			if cfg.AccrualSystemAddress != tt.wantAccrual {
				t.Errorf("AccrualSystemAddress = %q, want %q", cfg.AccrualSystemAddress, tt.wantAccrual)
			}
			if tt.wantSecret != "" {
				if cfg.JWTSecret != tt.wantSecret {
					t.Errorf("JWTSecret = %q, want %q", cfg.JWTSecret, tt.wantSecret)
				}
				return
			}
			if cfg.JWTSecret == "" {
				t.Error("JWTSecret is empty, want a generated one")
			}
		})
	}
}

func TestLoadGeneratesDistinctSecrets(t *testing.T) {
	load := func() string {
		t.Helper()

		originalArgs := os.Args
		originalFlags := flag.CommandLine
		defer func() {
			os.Args = originalArgs
			flag.CommandLine = originalFlags
		}()

		os.Args = []string{"gophermart", "-d", "postgres://localhost/db", "-r", "http://localhost:8080"}
		flag.CommandLine = flag.NewFlagSet("gophermart", flag.ContinueOnError)

		cfg, err := Load()
		if err != nil {
			t.Fatalf("load config: %v", err)
		}
		return cfg.JWTSecret
	}

	t.Setenv("JWT_SECRET", "")
	os.Unsetenv("JWT_SECRET")

	if first, second := load(), load(); first == second {
		t.Error("two runs produced the same generated secret")
	}
}
