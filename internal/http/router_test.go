package http

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Suuu-sh/Monee_Backend/internal/config"
	"github.com/Suuu-sh/Monee_Backend/internal/database"
	"github.com/Suuu-sh/Monee_Backend/internal/seed"
	"github.com/gin-gonic/gin"
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
	cfg.DatabasePath = "file:" + strings.NewReplacer("/", "_", " ", "_").Replace(t.Name()) + "?mode=memory&cache=shared"
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

type fakeAuthenticator map[string]AuthenticatedUser

func (f fakeAuthenticator) Authenticate(_ context.Context, bearerToken string) (AuthenticatedUser, error) {
	user, ok := f[bearerToken]
	if !ok {
		return AuthenticatedUser{}, errUnauthorized
	}
	return user, nil
}

func testRouter(t *testing.T, seedDefaults bool) *gin.Engine {
	t.Helper()
	cfg := testConfig(seedDefaults)
	db := testDB(t, seedDefaults)
	return NewRouterWithAuthenticator(cfg, db, slog.Default(), fakeAuthenticator{
		"user-a-token": {ID: "user-a"},
		"user-b-token": {ID: "user-b"},
	})
}

func authedRequest(method string, target string, body io.Reader) *http.Request {
	return authedRequestFor("user-a-token", method, target, body)
}

func authedRequestFor(token string, method string, target string, body io.Reader) *http.Request {
	req := httptest.NewRequest(method, target, body)
	req.Header.Set("Authorization", "Bearer "+token)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	return req
}

func TestHealthz(t *testing.T) {
	router := testRouter(t, true)
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestAPIRequiresAuthorization(t *testing.T) {
	router := testRouter(t, true)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/categories", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestListSeededCategories(t *testing.T) {
	router := testRouter(t, true)
	req := authedRequest(http.MethodGet, "/api/v1/categories", nil)
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
	router := testRouter(t, false)

	createdAt := time.Date(2026, 4, 10, 8, 0, 0, 0, time.UTC)
	updatedAt := time.Date(2026, 4, 11, 9, 30, 0, 0, time.UTC)
	body := bytes.NewBufferString(`{"id":"11111111-1111-1111-1111-111111111111","slug":"groceries","name":"Groceries","type":"expense","icon":"cart.fill","color_token":"mint","order":99,"created_at":"2026-04-10T08:00:00Z","updated_at":"2026-04-11T09:30:00Z"}`)
	req := authedRequest(http.MethodPost, "/api/v1/categories", body)
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

func TestCategoriesAreScopedByAuthenticatedUser(t *testing.T) {
	router := testRouter(t, false)

	userACategory := bytes.NewBufferString(`{"id":"aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa","slug":"shared","name":"Shared A","type":"expense","icon":"tag.fill","color_token":"mint","order":1}`)
	userAReq := authedRequestFor("user-a-token", http.MethodPost, "/api/v1/categories", userACategory)
	userARes := httptest.NewRecorder()
	router.ServeHTTP(userARes, userAReq)
	if userARes.Code != http.StatusCreated {
		t.Fatalf("expected user A category create 201, got %d: %s", userARes.Code, userARes.Body.String())
	}

	userBCategory := bytes.NewBufferString(`{"id":"bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb","slug":"shared","name":"Shared B","type":"expense","icon":"tag.fill","color_token":"mint","order":1}`)
	userBReq := authedRequestFor("user-b-token", http.MethodPost, "/api/v1/categories", userBCategory)
	userBRes := httptest.NewRecorder()
	router.ServeHTTP(userBRes, userBReq)
	if userBRes.Code != http.StatusCreated {
		t.Fatalf("expected same slug to be allowed for user B, got %d: %s", userBRes.Code, userBRes.Body.String())
	}

	listReq := authedRequestFor("user-a-token", http.MethodGet, "/api/v1/categories", nil)
	listRes := httptest.NewRecorder()
	router.ServeHTTP(listRes, listReq)
	if listRes.Code != http.StatusOK {
		t.Fatalf("expected list 200, got %d: %s", listRes.Code, listRes.Body.String())
	}
	if strings.Contains(listRes.Body.String(), "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb") {
		t.Fatalf("user A list leaked user B category: %s", listRes.Body.String())
	}
	if !strings.Contains(listRes.Body.String(), "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa") {
		t.Fatalf("user A list should include user A category: %s", listRes.Body.String())
	}
}

func TestCreatePreferenceAndSubscriptionWithProvidedIDs(t *testing.T) {
	router := testRouter(t, false)

	preferenceBody := bytes.NewBufferString(`{"id":"22222222-2222-2222-2222-222222222222","currency_code":"JPY","month_start_day":1,"is_ai_summaries_enabled":true,"appearance_raw":"system","language_raw":"ja","home_summary_range_raw":"month","budget_warning_threshold":0.8,"seed_scenario_raw":"balanced"}`)
	preferenceReq := authedRequest(http.MethodPost, "/api/v1/preferences", preferenceBody)
	preferenceRes := httptest.NewRecorder()
	router.ServeHTTP(preferenceRes, preferenceReq)
	if preferenceRes.Code != http.StatusCreated {
		t.Fatalf("expected preference create 201, got %d: %s", preferenceRes.Code, preferenceRes.Body.String())
	}

	subscriptionBody := bytes.NewBufferString(`{"id":"33333333-3333-3333-3333-333333333333","merchant_key":"manual-netflix","display_name":"Netflix","label":"Netflix","average_amount":1490,"cadence":"monthly","state":"active","monthly_equivalent_amount":1490,"yearly_equivalent_amount":17880}`)
	subscriptionReq := authedRequest(http.MethodPost, "/api/v1/subscriptions", subscriptionBody)
	subscriptionRes := httptest.NewRecorder()
	router.ServeHTTP(subscriptionRes, subscriptionReq)
	if subscriptionRes.Code != http.StatusCreated {
		t.Fatalf("expected subscription create 201, got %d: %s", subscriptionRes.Code, subscriptionRes.Body.String())
	}

	listReq := authedRequest(http.MethodGet, "/api/v1/subscriptions", nil)
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

func TestSnapshotBackupCreateListRestoreAndDelete(t *testing.T) {
	router := testRouter(t, false)

	createBody := bytes.NewBufferString(`{"payload_version":1,"encrypted_payload":"encrypted-json-payload"}`)
	createReq := authedRequest(http.MethodPost, "/api/v1/snapshots", createBody)
	createRes := httptest.NewRecorder()
	router.ServeHTTP(createRes, createReq)
	if createRes.Code != http.StatusCreated {
		t.Fatalf("expected snapshot create 201, got %d: %s", createRes.Code, createRes.Body.String())
	}

	var created map[string]any
	if err := json.Unmarshal(createRes.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode create response: %v", err)
	}
	restoreCode, ok := created["restore_code"].(string)
	if !ok || !strings.HasPrefix(restoreCode, "MONEE-") {
		t.Fatalf("expected restore code in create response, got %s", createRes.Body.String())
	}
	if strings.Contains(createRes.Body.String(), "encrypted-json-payload") {
		t.Fatalf("create response should not include encrypted payload: %s", createRes.Body.String())
	}

	listReq := authedRequest(http.MethodGet, "/api/v1/snapshots", nil)
	listRes := httptest.NewRecorder()
	router.ServeHTTP(listRes, listReq)
	if listRes.Code != http.StatusOK {
		t.Fatalf("expected snapshot list 200, got %d: %s", listRes.Code, listRes.Body.String())
	}
	if strings.Contains(listRes.Body.String(), "encrypted-json-payload") {
		t.Fatalf("list response should not include encrypted payload: %s", listRes.Body.String())
	}

	restoreReq := authedRequestFor("user-b-token", http.MethodGet, "/api/v1/snapshots/"+strings.ReplaceAll(strings.ToLower(restoreCode), "-", ""), nil)
	restoreRes := httptest.NewRecorder()
	router.ServeHTTP(restoreRes, restoreReq)
	if restoreRes.Code != http.StatusOK {
		t.Fatalf("expected snapshot restore 200, got %d: %s", restoreRes.Code, restoreRes.Body.String())
	}
	if !strings.Contains(restoreRes.Body.String(), "encrypted-json-payload") {
		t.Fatalf("restore response should include encrypted payload: %s", restoreRes.Body.String())
	}

	deleteReq := authedRequest(http.MethodDelete, "/api/v1/snapshots/"+created["id"].(string), nil)
	deleteRes := httptest.NewRecorder()
	router.ServeHTTP(deleteRes, deleteReq)
	if deleteRes.Code != http.StatusNoContent {
		t.Fatalf("expected snapshot delete 204, got %d: %s", deleteRes.Code, deleteRes.Body.String())
	}
}

func TestCreatePreferenceWithExistingIDUpdatesForSameUser(t *testing.T) {
	router := testRouter(t, false)

	preferenceID := "22222222-2222-2222-2222-222222222222"
	createBody := bytes.NewBufferString(`{"id":"` + preferenceID + `","currency_code":"JPY","month_start_day":1,"is_ai_summaries_enabled":true,"appearance_raw":"system","language_raw":"ja","home_summary_range_raw":"month","budget_warning_threshold":0.8,"seed_scenario_raw":"balanced","created_at":"2026-04-10T08:00:00Z","updated_at":"2026-04-10T08:00:00Z"}`)
	createReq := authedRequest(http.MethodPost, "/api/v1/preferences", createBody)
	createRes := httptest.NewRecorder()
	router.ServeHTTP(createRes, createReq)
	if createRes.Code != http.StatusCreated {
		t.Fatalf("expected initial preference create 201, got %d: %s", createRes.Code, createRes.Body.String())
	}

	updateBody := bytes.NewBufferString(`{"id":"` + preferenceID + `","currency_code":"USD","month_start_day":15,"is_ai_summaries_enabled":false,"appearance_raw":"dark","language_raw":"en","home_summary_range_raw":"week","budget_warning_threshold":0.5,"seed_scenario_raw":"minimal","created_at":"2026-04-10T08:00:00Z","updated_at":"2026-04-11T09:00:00Z"}`)
	updateReq := authedRequest(http.MethodPost, "/api/v1/preferences", updateBody)
	updateRes := httptest.NewRecorder()
	router.ServeHTTP(updateRes, updateReq)
	if updateRes.Code != http.StatusCreated {
		t.Fatalf("expected duplicate preference POST to upsert with 201, got %d: %s", updateRes.Code, updateRes.Body.String())
	}

	var preference map[string]any
	if err := json.Unmarshal(updateRes.Body.Bytes(), &preference); err != nil {
		t.Fatalf("decode updated preference: %v", err)
	}
	if preference["currency_code"] != "USD" || preference["appearance_raw"] != "dark" {
		t.Fatalf("expected duplicate POST to return updated preference, got %s", updateRes.Body.String())
	}

	listReq := authedRequest(http.MethodGet, "/api/v1/preferences", nil)
	listRes := httptest.NewRecorder()
	router.ServeHTTP(listRes, listReq)
	if listRes.Code != http.StatusOK {
		t.Fatalf("expected preference list 200, got %d: %s", listRes.Code, listRes.Body.String())
	}
	var payload map[string][]map[string]any
	if err := json.Unmarshal(listRes.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode list response: %v", err)
	}
	if len(payload["items"]) != 1 {
		t.Fatalf("expected exactly one preference after upsert, got %s", listRes.Body.String())
	}
}

func TestCreatePreferenceWithExistingIDCreatesNewPreferenceForAnotherUser(t *testing.T) {
	router := testRouter(t, false)

	preferenceID := "22222222-2222-2222-2222-222222222222"
	userABody := bytes.NewBufferString(`{"id":"` + preferenceID + `","currency_code":"JPY","month_start_day":1,"is_ai_summaries_enabled":true,"appearance_raw":"system","language_raw":"ja","home_summary_range_raw":"month","budget_warning_threshold":0.8,"seed_scenario_raw":"balanced"}`)
	userAReq := authedRequestFor("user-a-token", http.MethodPost, "/api/v1/preferences", userABody)
	userARes := httptest.NewRecorder()
	router.ServeHTTP(userARes, userAReq)
	if userARes.Code != http.StatusCreated {
		t.Fatalf("expected user A preference create 201, got %d: %s", userARes.Code, userARes.Body.String())
	}

	userBBody := bytes.NewBufferString(`{"id":"` + preferenceID + `","currency_code":"USD","month_start_day":15,"is_ai_summaries_enabled":false,"appearance_raw":"dark","language_raw":"en","home_summary_range_raw":"week","budget_warning_threshold":0.5,"seed_scenario_raw":"minimal"}`)
	userBReq := authedRequestFor("user-b-token", http.MethodPost, "/api/v1/preferences", userBBody)
	userBRes := httptest.NewRecorder()
	router.ServeHTTP(userBRes, userBReq)
	if userBRes.Code != http.StatusCreated {
		t.Fatalf("expected user B duplicate preference create 201 with a new ID, got %d: %s", userBRes.Code, userBRes.Body.String())
	}
	var userBPreference map[string]any
	if err := json.Unmarshal(userBRes.Body.Bytes(), &userBPreference); err != nil {
		t.Fatalf("decode user B preference: %v", err)
	}
	if userBPreference["id"] == preferenceID {
		t.Fatalf("expected user B preference to get a new ID, got %s", userBRes.Body.String())
	}
	if userBPreference["currency_code"] != "USD" {
		t.Fatalf("expected user B preference to use submitted fields, got %s", userBRes.Body.String())
	}

	listReq := authedRequestFor("user-a-token", http.MethodGet, "/api/v1/preferences", nil)
	listRes := httptest.NewRecorder()
	router.ServeHTTP(listRes, listReq)
	if listRes.Code != http.StatusOK {
		t.Fatalf("expected user A preference list 200, got %d: %s", listRes.Code, listRes.Body.String())
	}
	if strings.Contains(listRes.Body.String(), `"currency_code":"USD"`) {
		t.Fatalf("user B duplicate POST overwrote user A preference: %s", listRes.Body.String())
	}

	userBListReq := authedRequestFor("user-b-token", http.MethodGet, "/api/v1/preferences", nil)
	userBListRes := httptest.NewRecorder()
	router.ServeHTTP(userBListRes, userBListReq)
	if userBListRes.Code != http.StatusOK {
		t.Fatalf("expected user B preference list 200, got %d: %s", userBListRes.Code, userBListRes.Body.String())
	}
	if !strings.Contains(userBListRes.Body.String(), `"currency_code":"USD"`) {
		t.Fatalf("user B list should include the newly created preference: %s", userBListRes.Body.String())
	}
}
