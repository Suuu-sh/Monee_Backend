package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Suuu-sh/Monee_Backend/internal/config"
	"github.com/Suuu-sh/Monee_Backend/internal/database"
	apiserver "github.com/Suuu-sh/Monee_Backend/internal/http"
	"github.com/Suuu-sh/Monee_Backend/internal/seed"
)

func main() {
	cfg := config.Load()
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))

	db, err := database.Open(cfg)
	if err != nil {
		logger.Error("failed to open database", "error", err)
		os.Exit(1)
	}

	if err := database.Migrate(db, cfg); err != nil {
		logger.Error("failed to migrate database", "error", err)
		os.Exit(1)
	}

	if cfg.SeedDefaultCategories && !cfg.RequireAuth {
		if err := seed.EnsureDefaultsForUser(db, cfg.DefaultUserID); err != nil {
			logger.Error("failed to seed default categories", "error", err)
			os.Exit(1)
		}
	}

	router := apiserver.NewRouter(cfg, db, logger)
	server := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           router,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	go func() {
		logger.Info("monee-backend started", "port", cfg.Port, "database_driver", cfg.DatabaseDriver)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("server terminated", "error", err)
			os.Exit(1)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	logger.Info("shutdown signal received")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		logger.Error("graceful shutdown failed", "error", err)
		os.Exit(1)
	}
}
