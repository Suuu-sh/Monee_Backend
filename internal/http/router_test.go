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

	"github.com/Suuu-sh/Monee_Backend/internal/config"
	"github.com/Suuu-sh/Monee_Backend/internal/database"
	"github.com/Suuu-sh/Monee_Backend/internal/seed"
	"gorm.io/gorm"
)

func testConfig(seedDefaults bool) config.Config {
	return config.Config{
		AppEnv:                "test",
		Port:                  "8080",
		DatabaseDriver:        "sqlite",
		DatabasePath:          "file::memory:?cache=shared",
		SeedDefaultCategories: seedDefaults,
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
		if err := seed.EnsureDefaults(db); err != nil {
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
