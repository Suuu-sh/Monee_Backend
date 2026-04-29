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

func Migrate(db *gorm.DB) error {
	dropLegacyUniqueIndexes(db)

	return db.AutoMigrate(
		&models.Category{},
		&models.Transaction{},
		&models.Budget{},
		&models.SavingsGoal{},
		&models.SubscriptionRecord{},
		&models.AppPreference{},
	)
}

func dropLegacyUniqueIndexes(db *gorm.DB) {
	// User scoping changed globally unique natural keys into per-user keys.
	// AutoMigrate creates the new composite indexes but intentionally does not
	// remove old indexes, so best-effort drop the legacy single-column indexes.
	for _, item := range []struct {
		model any
		name  string
	}{
		{model: &models.Category{}, name: "idx_categories_slug"},
		{model: &models.SubscriptionRecord{}, name: "idx_subscription_records_merchant_key"},
	} {
		if db.Migrator().HasIndex(item.model, item.name) {
			_ = db.Migrator().DropIndex(item.model, item.name)
		}
	}
}
