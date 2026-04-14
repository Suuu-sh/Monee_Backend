package config

import (
	"os"
	"strings"
)

type Config struct {
	AppEnv                string
	Port                  string
	DatabaseDriver        string
	DatabaseURL           string
	DatabasePath          string
	SeedDefaultCategories bool
}

func Load() Config {
	seedDefaultCategories := envOrDefault("SEED_DEFAULT_CATEGORIES", os.Getenv("SEED_DEMO_DATA"))
	if strings.TrimSpace(seedDefaultCategories) == "" {
		seedDefaultCategories = "true"
	}

	return Config{
		AppEnv:                envOrDefault("APP_ENV", "development"),
		Port:                  envOrDefault("PORT", "8080"),
		DatabaseDriver:        strings.ToLower(envOrDefault("DATABASE_DRIVER", "sqlite")),
		DatabaseURL:           envOrDefault("DATABASE_URL", ""),
		DatabasePath:          envOrDefault("DATABASE_PATH", "data/monee.db"),
		SeedDefaultCategories: strings.EqualFold(seedDefaultCategories, "true"),
	}
}

func envOrDefault(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
