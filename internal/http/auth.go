package http

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"regexp"
	"strings"

	"github.com/Suuu-sh/Monee_Backend/internal/config"
)

var errUnauthorized = errors.New("unauthorized")

var guestTokenPattern = regexp.MustCompile(`^[A-Za-z0-9._~:-]{16,256}$`)

type AuthenticatedUser struct {
	ID          string
	IsAnonymous bool
}

type Authenticator interface {
	Authenticate(ctx context.Context, bearerToken string) (AuthenticatedUser, error)
}

func NewAuthenticator(_ config.Config) Authenticator {
	return NewGuestCodeAuthenticator()
}

type GuestCodeAuthenticator struct{}

func NewGuestCodeAuthenticator() *GuestCodeAuthenticator {
	return &GuestCodeAuthenticator{}
}

func (a *GuestCodeAuthenticator) Authenticate(_ context.Context, bearerToken string) (AuthenticatedUser, error) {
	token := strings.TrimSpace(bearerToken)
	if strings.HasPrefix(token, "Bearer ") {
		token = strings.TrimSpace(strings.TrimPrefix(token, "Bearer "))
	}
	if token == "" || !guestTokenPattern.MatchString(token) {
		return AuthenticatedUser{}, errUnauthorized
	}

	sum := sha256.Sum256([]byte(token))
	userID := "guest_" + hex.EncodeToString(sum[:])[:48]
	return AuthenticatedUser{ID: userID, IsAnonymous: true}, nil
}
