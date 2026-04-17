package config

import (
	"os"
	"strings"

	"github.com/Suuu-sh/Monee_Backend/internal/models"
)

type Config struct {
	AppEnv                string
	Port                  string
	DatabaseDriver        string
	DatabaseURL           string
	DatabasePath          string
	SeedDefaultCategories bool
	SupabaseProjectURL    string
	SupabaseJWKSURL       string
	SupabaseJWTIssuer     string
	SupabaseJWTSecret     string
	RequireAuth           bool
	DefaultUserID         string
}

func Load() Config {
	seedDefaultCategories := envOrDefault("SEED_DEFAULT_CATEGORIES", os.Getenv("SEED_DEMO_DATA"))
	if strings.TrimSpace(seedDefaultCategories) == "" {
		seedDefaultCategories = "true"
	}

	supabaseProjectURL := strings.TrimRight(envOrDefault("SUPABASE_PROJECT_URL", envOrDefault("SUPABASE_URL", "")), "/")
	supabaseJWTIssuer := envOrDefault("SUPABASE_JWT_ISSUER", "")
	if supabaseJWTIssuer == "" && supabaseProjectURL != "" {
		supabaseJWTIssuer = supabaseProjectURL + "/auth/v1"
	}

	supabaseJWKSURL := envOrDefault("SUPABASE_JWKS_URL", "")
	if supabaseJWKSURL == "" && supabaseJWTIssuer != "" {
		supabaseJWKSURL = strings.TrimRight(supabaseJWTIssuer, "/") + "/.well-known/jwks.json"
	}

	supabaseJWTSecret := envOrDefault("SUPABASE_JWT_SECRET", "")
	requireAuthDefault := "false"
	if supabaseProjectURL != "" || supabaseJWTSecret != "" {
		requireAuthDefault = "true"
	}

	return Config{
		AppEnv:                envOrDefault("APP_ENV", "development"),
		Port:                  envOrDefault("PORT", "8080"),
		DatabaseDriver:        strings.ToLower(envOrDefault("DATABASE_DRIVER", "sqlite")),
		DatabaseURL:           envOrDefault("DATABASE_URL", ""),
		DatabasePath:          envOrDefault("DATABASE_PATH", "data/monee.db"),
		SeedDefaultCategories: strings.EqualFold(seedDefaultCategories, "true"),
		SupabaseProjectURL:    supabaseProjectURL,
		SupabaseJWKSURL:       supabaseJWKSURL,
		SupabaseJWTIssuer:     supabaseJWTIssuer,
		SupabaseJWTSecret:     supabaseJWTSecret,
		RequireAuth:           strings.EqualFold(envOrDefault("SUPABASE_REQUIRE_AUTH", requireAuthDefault), "true"),
		DefaultUserID:         envOrDefault("DEFAULT_USER_ID", models.DefaultLocalUserID),
	}
}

func envOrDefault(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
