package database

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Suuu-sh/Monee_Backend/internal/config"
	"github.com/Suuu-sh/Monee_Backend/internal/models"
	"github.com/glebarez/sqlite"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func Open(cfg config.Config) (*gorm.DB, error) {
	switch strings.ToLower(strings.TrimSpace(cfg.DatabaseDriver)) {
	case "", "sqlite":
		return openSQLite(cfg.DatabasePath)
	case "postgres", "postgresql":
		return openPostgres(cfg.DatabaseURL)
	default:
		return nil, fmt.Errorf("unsupported database driver: %s", cfg.DatabaseDriver)
	}
}

func openSQLite(databasePath string) (*gorm.DB, error) {
	if err := os.MkdirAll(filepath.Dir(databasePath), 0o755); err != nil {
		return nil, fmt.Errorf("create database directory: %w", err)
	}

	db, err := gorm.Open(sqlite.Open(databasePath), &gorm.Config{})
	if err != nil {
		return nil, fmt.Errorf("open sqlite database: %w", err)
	}
	return db, nil
}

func openPostgres(databaseURL string) (*gorm.DB, error) {
	if strings.TrimSpace(databaseURL) == "" {
		return nil, fmt.Errorf("database url is required for postgres")
	}

	db, err := gorm.Open(postgres.Open(databaseURL), &gorm.Config{})
	if err != nil {
		return nil, fmt.Errorf("open postgres database: %w", err)
	}
	return db, nil
}

func Migrate(db *gorm.DB, cfg config.Config) error {
	if shouldSkipManagedPostgresMigrations(cfg) {
		return nil
	}

	if err := db.AutoMigrate(
		&models.Category{},
		&models.Transaction{},
		&models.Budget{},
		&models.SavingsGoal{},
		&models.SubscriptionRecord{},
		&models.AppPreference{},
	); err != nil {
		return err
	}

	return backfillOwnership(db, models.DefaultLocalUserID)
}

func shouldSkipManagedPostgresMigrations(cfg config.Config) bool {
	driver := strings.ToLower(strings.TrimSpace(cfg.DatabaseDriver))
	appEnv := strings.ToLower(strings.TrimSpace(cfg.AppEnv))

	return (driver == "postgres" || driver == "postgresql") && appEnv == "production"
}

func backfillOwnership(db *gorm.DB, defaultUserID string) error {
	tables := []string{
		"categories",
		"transactions",
		"budgets",
		"savings_goals",
		"subscription_records",
		"app_preferences",
	}

	for _, table := range tables {
		if err := db.Exec(
			fmt.Sprintf("UPDATE %s SET user_id = ? WHERE user_id IS NULL OR user_id = ''", table),
			defaultUserID,
		).Error; err != nil {
			return fmt.Errorf("backfill %s ownership: %w", table, err)
		}
	}

	return nil
}
