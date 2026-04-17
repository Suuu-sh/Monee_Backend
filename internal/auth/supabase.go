package auth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/Suuu-sh/Monee_Backend/internal/config"
	"github.com/golang-jwt/jwt/v5"
	jwk "github.com/lestrrat-go/jwx/v2/jwk"
)

const jwksCacheTTL = 10 * time.Minute

var ErrMissingBearerToken = errors.New("missing bearer token")

type SupabaseVerifier struct {
	cfg        config.Config
	httpClient *http.Client

	mu       sync.RWMutex
	cachedAt time.Time
	cached   jwk.Set
}

type Claims struct {
	Role        string `json:"role"`
	IsAnonymous bool   `json:"is_anonymous"`
	jwt.RegisteredClaims
}

func NewSupabaseVerifier(cfg config.Config) *SupabaseVerifier {
	return &SupabaseVerifier{
		cfg: cfg,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

func (v *SupabaseVerifier) Enabled() bool {
	return v.cfg.RequireAuth
}

func (v *SupabaseVerifier) Verify(ctx context.Context, tokenString string) (Claims, error) {
	tokenString = strings.TrimSpace(tokenString)
	if tokenString == "" {
		return Claims{}, ErrMissingBearerToken
	}

	if strings.TrimSpace(v.cfg.SupabaseJWTSecret) != "" {
		return v.verifyWithSecret(tokenString)
	}

	return v.verifyWithJWKS(ctx, tokenString)
}

func (v *SupabaseVerifier) verifyWithSecret(tokenString string) (Claims, error) {
	claims := Claims{}
	parsed, err := jwt.ParseWithClaims(
		tokenString,
		&claims,
		func(token *jwt.Token) (any, error) {
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("unexpected signing method: %s", token.Method.Alg())
			}
			return []byte(v.cfg.SupabaseJWTSecret), nil
		},
		jwt.WithIssuer(v.cfg.SupabaseJWTIssuer),
		jwt.WithAudience("authenticated"),
	)
	if err != nil {
		return Claims{}, fmt.Errorf("verify supabase jwt: %w", err)
	}
	if !parsed.Valid {
		return Claims{}, errors.New("supabase jwt is invalid")
	}
	if strings.TrimSpace(claims.Subject) == "" {
		return Claims{}, errors.New("supabase jwt subject is missing")
	}
	return claims, nil
}

func (v *SupabaseVerifier) verifyWithJWKS(ctx context.Context, tokenString string) (Claims, error) {
	set, err := v.jwks(ctx)
	if err != nil {
		return Claims{}, err
	}

	parser := jwt.NewParser(
		jwt.WithIssuer(v.cfg.SupabaseJWTIssuer),
		jwt.WithAudience("authenticated"),
	)

	claims := Claims{}
	parsed, err := parser.ParseWithClaims(
		tokenString,
		&claims,
		func(token *jwt.Token) (any, error) {
			kid, _ := token.Header["kid"].(string)
			if kid == "" {
				return nil, errors.New("supabase jwt kid is missing")
			}

			key, ok := set.LookupKeyID(kid)
			if !ok {
				return nil, fmt.Errorf("supabase jwk not found for kid %s", kid)
			}

			var publicKey any
			if err := key.Raw(&publicKey); err != nil {
				return nil, fmt.Errorf("extract jwk public key: %w", err)
			}
			return publicKey, nil
		},
	)
	if err != nil {
		return Claims{}, fmt.Errorf("verify supabase jwt with jwks: %w", err)
	}
	if !parsed.Valid {
		return Claims{}, errors.New("supabase jwt is invalid")
	}
	if strings.TrimSpace(claims.Subject) == "" {
		return Claims{}, errors.New("supabase jwt subject is missing")
	}
	return claims, nil
}

func (v *SupabaseVerifier) jwks(ctx context.Context) (jwk.Set, error) {
	v.mu.RLock()
	if v.cached != nil && time.Since(v.cachedAt) < jwksCacheTTL {
		defer v.mu.RUnlock()
		return v.cached, nil
	}
	v.mu.RUnlock()

	if strings.TrimSpace(v.cfg.SupabaseJWKSURL) == "" {
		return nil, errors.New("supabase jwks url is not configured")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, v.cfg.SupabaseJWKSURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create supabase jwks request: %w", err)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := v.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch supabase jwks: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("fetch supabase jwks: unexpected status %d", resp.StatusCode)
	}

	var payload struct {
		Keys []json.RawMessage `json:"keys"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, fmt.Errorf("decode supabase jwks response: %w", err)
	}

	set := jwk.NewSet()
	for _, rawKey := range payload.Keys {
		key, err := jwk.ParseKey(rawKey)
		if err != nil {
			return nil, fmt.Errorf("parse supabase jwk: %w", err)
		}
		if err := set.AddKey(key); err != nil {
			return nil, fmt.Errorf("cache supabase jwk: %w", err)
		}
	}

	v.mu.Lock()
	v.cached = set
	v.cachedAt = time.Now()
	v.mu.Unlock()

	return set, nil
}
