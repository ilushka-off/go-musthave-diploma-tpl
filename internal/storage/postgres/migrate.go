package postgres

import (
	"errors"
	"fmt"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/source/iofs"
)

// RunMigrations применяет встроенные миграции схемы к базе databaseURI.
// Повторный вызов на актуальной схеме ошибкой не является.
func RunMigrations(databaseURI string) error {
	sourceDriver, err := iofs.New(migrationFS, "migrations")
	if err != nil {
		return fmt.Errorf("create migrations source: %w", err)
	}

	migrator, err := migrate.NewWithSourceInstance("iofs", sourceDriver, databaseURI)
	if err != nil {
		return fmt.Errorf("create migrator: %w", err)
	}

	defer migrator.Close()

	err = migrator.Up()
	if err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("apply migrations: %w", err)
	}

	return nil
}
