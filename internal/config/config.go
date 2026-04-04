package config

import "os"

type Config struct {
	AppEnv       string
	Port         string
	DatabasePath string
	SeedDemoData bool
}

func Load() Config {
	return Config{
		AppEnv:       envOrDefault("APP_ENV", "development"),
		Port:         envOrDefault("PORT", "8080"),
		DatabasePath: envOrDefault("DATABASE_PATH", "data/monee.db"),
		SeedDemoData: envOrDefault("SEED_DEMO_DATA", "true") == "true",
	}
}

func envOrDefault(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
