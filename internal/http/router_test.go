package http

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	backendauth "github.com/Suuu-sh/Monee_Backend/internal/auth"
	"github.com/Suuu-sh/Monee_Backend/internal/config"
	"github.com/Suuu-sh/Monee_Backend/internal/database"
	"github.com/Suuu-sh/Monee_Backend/internal/models"
	"github.com/Suuu-sh/Monee_Backend/internal/seed"
	"github.com/golang-jwt/jwt/v5"
	"gorm.io/gorm"
)

func testConfig(seedDefaults bool) config.Config {
	return config.Config{
		AppEnv:                "test",
		Port:                  "8080",
		DatabaseDriver:        "sqlite",
		DatabasePath:          "file::memory:?cache=shared",
		SeedDefaultCategories: seedDefaults,
		DefaultUserID:         models.DefaultLocalUserID,
	}
}

func testDB(t *testing.T, seedDefaults bool) *gorm.DB {
	t.Helper()
	cfg := testConfig(seedDefaults)
	db, err := database.Open(cfg)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := database.Migrate(db); err != nil {
		t.Fatalf("migrate db: %v", err)
	}
	if seedDefaults {
		if err := seed.EnsureDefaultsForUser(db, models.DefaultLocalUserID); err != nil {
			t.Fatalf("seed defaults: %v", err)
		}
	}
	return db
}

func TestHealthz(t *testing.T) {
	router := NewRouter(testConfig(true), testDB(t, true), slog.Default())
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestListSeededCategories(t *testing.T) {
	router := NewRouter(testConfig(true), testDB(t, true), slog.Default())
	req := httptest.NewRequest(http.MethodGet, "/api/v1/categories", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	body, _ := io.ReadAll(w.Body)
	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	items, ok := payload["items"].([]any)
	if !ok || len(items) == 0 {
		t.Fatalf("expected seeded categories, got %s", string(body))
	}
}

func TestCreateCategoryWithProvidedIDAndTimestamps(t *testing.T) {
	router := NewRouter(testConfig(false), testDB(t, false), slog.Default())

	createdAt := time.Date(2026, 4, 10, 8, 0, 0, 0, time.UTC)
	updatedAt := time.Date(2026, 4, 11, 9, 30, 0, 0, time.UTC)
	body := bytes.NewBufferString(`{"id":"11111111-1111-1111-1111-111111111111","slug":"groceries","name":"Groceries","type":"expense","icon":"cart.fill","color_token":"mint","order":99,"created_at":"2026-04-10T08:00:00Z","updated_at":"2026-04-11T09:30:00Z"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/categories", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}

	var payload map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload["id"] != "11111111-1111-1111-1111-111111111111" {
		t.Fatalf("expected custom id to be preserved, got %#v", payload["id"])
	}
	if payload["created_at"] != createdAt.Format(time.RFC3339) {
		t.Fatalf("expected created_at to be preserved, got %#v", payload["created_at"])
	}
	if payload["updated_at"] != updatedAt.Format(time.RFC3339) {
		t.Fatalf("expected updated_at to be preserved, got %#v", payload["updated_at"])
	}
}

func TestCreatePreferenceAndSubscriptionWithProvidedIDs(t *testing.T) {
	router := NewRouter(testConfig(false), testDB(t, false), slog.Default())

	preferenceBody := bytes.NewBufferString(`{"id":"22222222-2222-2222-2222-222222222222","currency_code":"JPY","month_start_day":1,"is_ai_summaries_enabled":true,"appearance_raw":"system","language_raw":"ja","home_summary_range_raw":"month","budget_warning_threshold":0.8,"seed_scenario_raw":"balanced"}`)
	preferenceReq := httptest.NewRequest(http.MethodPost, "/api/v1/preferences", preferenceBody)
	preferenceReq.Header.Set("Content-Type", "application/json")
	preferenceRes := httptest.NewRecorder()
	router.ServeHTTP(preferenceRes, preferenceReq)
	if preferenceRes.Code != http.StatusCreated {
		t.Fatalf("expected preference create 201, got %d: %s", preferenceRes.Code, preferenceRes.Body.String())
	}

	subscriptionBody := bytes.NewBufferString(`{"id":"33333333-3333-3333-3333-333333333333","merchant_key":"manual-netflix","display_name":"Netflix","label":"Netflix","average_amount":1490,"cadence":"monthly","state":"active","monthly_equivalent_amount":1490,"yearly_equivalent_amount":17880}`)
	subscriptionReq := httptest.NewRequest(http.MethodPost, "/api/v1/subscriptions", subscriptionBody)
	subscriptionReq.Header.Set("Content-Type", "application/json")
	subscriptionRes := httptest.NewRecorder()
	router.ServeHTTP(subscriptionRes, subscriptionReq)
	if subscriptionRes.Code != http.StatusCreated {
		t.Fatalf("expected subscription create 201, got %d: %s", subscriptionRes.Code, subscriptionRes.Body.String())
	}

	listReq := httptest.NewRequest(http.MethodGet, "/api/v1/subscriptions", nil)
	listRes := httptest.NewRecorder()
	router.ServeHTTP(listRes, listReq)
	if listRes.Code != http.StatusOK {
		t.Fatalf("expected subscription list 200, got %d: %s", listRes.Code, listRes.Body.String())
	}

	var payload map[string]any
	if err := json.Unmarshal(listRes.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode list response: %v", err)
	}
	items, ok := payload["items"].([]any)
	if !ok || len(items) != 1 {
		t.Fatalf("expected 1 subscription, got %s", listRes.Body.String())
	}
}

func TestAuthScopesDataBySupabaseSubject(t *testing.T) {
	cfg := testConfig(true)
	cfg.RequireAuth = true
	cfg.SupabaseJWTSecret = "test-supabase-secret"
	cfg.SupabaseJWTIssuer = "https://example.supabase.co/auth/v1"

	db := testDB(t, false)
	router := NewRouter(cfg, db, slog.Default())

	userA := "11111111-1111-1111-1111-111111111111"
	userB := "22222222-2222-2222-2222-222222222222"

	createCategory := func(userID, slug string) {
		body := bytes.NewBufferString(`{"slug":"` + slug + `","name":"` + slug + `","type":"expense","icon":"cart.fill","color_token":"mint","order":1}`)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/categories", body)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+signedToken(t, cfg, userID))
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		if w.Code != http.StatusCreated {
			t.Fatalf("expected 201 when creating category for %s, got %d: %s", userID, w.Code, w.Body.String())
		}
	}

	createCategory(userA, "groceries")
	createCategory(userB, "salary-b")

	reqA := httptest.NewRequest(http.MethodGet, "/api/v1/categories", nil)
	reqA.Header.Set("Authorization", "Bearer "+signedToken(t, cfg, userA))
	resA := httptest.NewRecorder()
	router.ServeHTTP(resA, reqA)
	if resA.Code != http.StatusOK {
		t.Fatalf("expected 200 for user A, got %d: %s", resA.Code, resA.Body.String())
	}

	var payloadA map[string]any
	if err := json.Unmarshal(resA.Body.Bytes(), &payloadA); err != nil {
		t.Fatalf("decode user A response: %v", err)
	}
	itemsA, ok := payloadA["items"].([]any)
	if !ok {
		t.Fatalf("expected categories array for user A, got %s", resA.Body.String())
	}

	for _, item := range itemsA {
		row := item.(map[string]any)
		if row["user_id"] != userA {
			t.Fatalf("expected only user A data, got row %#v", row)
		}
	}

	reqMissing := httptest.NewRequest(http.MethodGet, "/api/v1/categories", nil)
	resMissing := httptest.NewRecorder()
	router.ServeHTTP(resMissing, reqMissing)
	if resMissing.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 without auth header, got %d", resMissing.Code)
	}

	_, err := backendauth.NewSupabaseVerifier(cfg).Verify(reqA.Context(), "")
	if err == nil {
		t.Fatalf("expected missing token verification to fail")
	}
}

func signedToken(t *testing.T, cfg config.Config, subject string) string {
	t.Helper()
	now := time.Now()
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"aud":  "authenticated",
		"iss":  cfg.SupabaseJWTIssuer,
		"sub":  subject,
		"exp":  now.Add(30 * time.Minute).Unix(),
		"iat":  now.Unix(),
		"role": "authenticated",
	})
	signed, err := token.SignedString([]byte(cfg.SupabaseJWTSecret))
	if err != nil {
		t.Fatalf("sign token: %v", err)
	}
	return signed
}
