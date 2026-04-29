package http

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/Suuu-sh/Monee_Backend/internal/config"
)

var errUnauthorized = errors.New("unauthorized")

type AuthenticatedUser struct {
	ID          string
	IsAnonymous bool
}

type Authenticator interface {
	Authenticate(ctx context.Context, bearerToken string) (AuthenticatedUser, error)
}

type SupabaseAuthenticator struct {
	projectURL     string
	publishableKey string
	client         *http.Client
	logger         *slog.Logger
}

func NewSupabaseAuthenticator(cfg config.Config, logger *slog.Logger) *SupabaseAuthenticator {
	return &SupabaseAuthenticator{
		projectURL:     strings.TrimRight(strings.TrimSpace(cfg.SupabaseProjectURL), "/"),
		publishableKey: strings.TrimSpace(cfg.SupabasePublishableKey),
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
		logger: logger,
	}
}

func (a *SupabaseAuthenticator) Authenticate(ctx context.Context, bearerToken string) (AuthenticatedUser, error) {
	token := strings.TrimSpace(bearerToken)
	if token == "" {
		return AuthenticatedUser{}, errUnauthorized
	}
	if a.projectURL == "" || a.publishableKey == "" {
		return AuthenticatedUser{}, fmt.Errorf("supabase auth is not configured")
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, a.projectURL+"/auth/v1/user", nil)
	if err != nil {
		return AuthenticatedUser{}, err
	}
	request.Header.Set("Accept", "application/json")
	request.Header.Set("Authorization", "Bearer "+token)
	request.Header.Set("Apikey", a.publishableKey)
	request.Header.Set("X-Client-Info", "monee-backend/1.0")
	request.Header.Set("X-Supabase-Api-Version", "2024-01-01")

	response, err := a.client.Do(request)
	if err != nil {
		return AuthenticatedUser{}, err
	}
	defer response.Body.Close()

	if response.StatusCode < 200 || response.StatusCode >= 300 {
		if response.StatusCode == http.StatusUnauthorized || response.StatusCode == http.StatusForbidden {
			return AuthenticatedUser{}, errUnauthorized
		}
		if a.logger != nil {
			a.logger.Warn("supabase auth verification failed", "status", response.StatusCode)
		}
		return AuthenticatedUser{}, errUnauthorized
	}

	var user supabaseUserResponse
	if err := json.NewDecoder(response.Body).Decode(&user); err != nil {
		return AuthenticatedUser{}, err
	}
	if strings.TrimSpace(user.ID) == "" {
		return AuthenticatedUser{}, errUnauthorized
	}

	return AuthenticatedUser{
		ID:          user.ID,
		IsAnonymous: user.IsAnonymous,
	}, nil
}

type supabaseUserResponse struct {
	ID          string `json:"id"`
	IsAnonymous bool   `json:"is_anonymous"`
}
