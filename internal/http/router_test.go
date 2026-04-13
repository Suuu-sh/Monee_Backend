package http

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Suuu-sh/Monee_Backend/internal/config"
	"github.com/Suuu-sh/Monee_Backend/internal/database"
	"github.com/Suuu-sh/Monee_Backend/internal/seed"
	"gorm.io/gorm"
)

func testDB(t *testing.T) *gorm.DB {
	t.Helper()
	cfg := config.Config{AppEnv: "test", Port: "8080", DatabasePath: "file::memory:?cache=shared", SeedDemoData: true}
	db, err := database.Open(cfg)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := database.Migrate(db); err != nil {
		t.Fatalf("migrate db: %v", err)
	}
	if err := seed.EnsureDefaults(db); err != nil {
		t.Fatalf("seed defaults: %v", err)
	}
	return db
}

func TestHealthz(t *testing.T) {
	router := NewRouter(config.Config{AppEnv: "test", Port: "8080", DatabasePath: "file::memory:?cache=shared", SeedDemoData: true}, testDB(t), slog.Default())
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestListSeededCategories(t *testing.T) {
	router := NewRouter(config.Config{AppEnv: "test", Port: "8080", DatabasePath: "file::memory:?cache=shared", SeedDemoData: true}, testDB(t), slog.Default())
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

func TestCreateCategoryWithProvidedID(t *testing.T) {
	router := NewRouter(config.Config{AppEnv: "test", Port: "8080", DatabasePath: "file::memory:?cache=shared", SeedDemoData: false}, testDB(t), slog.Default())

	body := bytes.NewBufferString(`{"id":"11111111-1111-1111-1111-111111111111","slug":"groceries","name":"Groceries","type":"expense","icon":"cart.fill","color_token":"mint","order":99}`)
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
}
