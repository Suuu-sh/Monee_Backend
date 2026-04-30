package http

import (
	"crypto/rand"
	"encoding/base32"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/Suuu-sh/Monee_Backend/internal/config"
	"github.com/Suuu-sh/Monee_Backend/internal/models"
	"github.com/Suuu-sh/Monee_Backend/internal/seed"
	"github.com/Suuu-sh/Monee_Backend/internal/service"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type Server struct {
	cfg            config.Config
	db             *gorm.DB
	logger         *slog.Logger
	summaryService *service.SummaryService
	authenticator  Authenticator
}

func NewRouter(cfg config.Config, db *gorm.DB, logger *slog.Logger) *gin.Engine {
	return NewRouterWithAuthenticator(cfg, db, logger, NewAuthenticator(cfg))
}

func NewRouterWithAuthenticator(cfg config.Config, db *gorm.DB, logger *slog.Logger, authenticator Authenticator) *gin.Engine {
	if cfg.AppEnv == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	router := gin.New()
	router.Use(gin.Recovery())
	router.Use(gin.Logger())
	router.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"*"},
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Accept", "Authorization"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: false,
		MaxAge:           12 * time.Hour,
	}))

	server := &Server{
		cfg:            cfg,
		db:             db,
		logger:         logger,
		summaryService: service.NewSummaryService(db),
		authenticator:  authenticator,
	}

	router.GET("/healthz", server.healthz)
	router.GET("/readyz", server.readyz)

	api := router.Group("/api/v1", server.requireAuth)
	{
		api.GET("/summary", server.getSummary)

		api.GET("/preferences", server.listPreferences)
		api.POST("/preferences", server.createPreference)
		api.PUT("/preferences/:id", server.updatePreference)
		api.DELETE("/preferences/:id", server.deletePreference)

		api.GET("/categories", server.listCategories)
		api.POST("/categories", server.createCategory)
		api.PUT("/categories/:id", server.updateCategory)
		api.DELETE("/categories/:id", server.deleteCategory)

		api.GET("/transactions", server.listTransactions)
		api.POST("/transactions", server.createTransaction)
		api.PUT("/transactions/:id", server.updateTransaction)
		api.DELETE("/transactions/:id", server.deleteTransaction)

		api.GET("/budgets", server.listBudgets)
		api.POST("/budgets", server.createBudget)
		api.PUT("/budgets/:id", server.updateBudget)
		api.DELETE("/budgets/:id", server.deleteBudget)

		api.GET("/savings-goals", server.listSavingsGoals)
		api.POST("/savings-goals", server.createSavingsGoal)
		api.PUT("/savings-goals/:id", server.updateSavingsGoal)
		api.DELETE("/savings-goals/:id", server.deleteSavingsGoal)

		api.GET("/subscriptions", server.listSubscriptions)
		api.POST("/subscriptions", server.createSubscription)
		api.PUT("/subscriptions/:id", server.updateSubscription)
		api.DELETE("/subscriptions/:id", server.deleteSubscription)

		api.GET("/snapshots", server.listSnapshotBackups)
		api.POST("/snapshots", server.createSnapshotBackup)
		api.GET("/snapshots/:restore_code", server.getSnapshotBackup)
		api.DELETE("/snapshots/:id", server.deleteSnapshotBackup)
	}

	return router
}

func (s *Server) healthz(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok", "service": "monee-backend"})
}

func (s *Server) readyz(c *gin.Context) {
	sqlDB, err := s.db.DB()
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"status": "error", "message": err.Error()})
		return
	}
	if err := sqlDB.PingContext(c.Request.Context()); err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"status": "error", "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ready"})
}

func (s *Server) requireAuth(c *gin.Context) {
	scheme, token, ok := strings.Cut(strings.TrimSpace(c.GetHeader("Authorization")), " ")
	if !ok || !strings.EqualFold(scheme, "Bearer") || strings.TrimSpace(token) == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "missing_authorization", "message": "Authorization bearer token is required"})
		c.Abort()
		return
	}
	token = strings.TrimSpace(token)

	if s.authenticator == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "auth_not_configured", "message": "Backend authentication is not configured"})
		c.Abort()
		return
	}

	user, err := s.authenticator.Authenticate(c.Request.Context(), token)
	if err != nil {
		if !errors.Is(err, errUnauthorized) {
			s.logger.Warn("failed to authenticate request", "error", err)
		}
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid_authorization", "message": "Authorization bearer token is invalid"})
		c.Abort()
		return
	}

	requestDB := s.db.WithContext(c.Request.Context()).Begin()
	if requestDB.Error != nil {
		s.respondError(c, http.StatusInternalServerError, "failed_to_start_request_transaction", requestDB.Error)
		c.Abort()
		return
	}
	defer func() {
		if recovered := recover(); recovered != nil {
			_ = requestDB.Rollback().Error
			panic(recovered)
		}
	}()

	if isPostgresDriver(s.cfg.DatabaseDriver) {
		if err := requestDB.Exec("SELECT set_config('app.current_user_id', ?, true)", user.ID).Error; err != nil {
			_ = requestDB.Rollback().Error
			s.respondError(c, http.StatusInternalServerError, "failed_to_set_user_context", err)
			c.Abort()
			return
		}
	}

	c.Set("request_db", requestDB)
	c.Set("authenticated_user", user)

	if s.cfg.SeedDefaultCategories {
		if err := seed.EnsureDefaultsForUser(requestDB, user.ID); err != nil {
			_ = requestDB.Rollback().Error
			s.respondError(c, http.StatusInternalServerError, "failed_to_seed_user_defaults", err)
			c.Abort()
			return
		}
	}

	c.Next()

	if c.Writer.Status() >= http.StatusBadRequest {
		_ = requestDB.Rollback().Error
		return
	}
	if err := requestDB.Commit().Error; err != nil {
		s.respondError(c, http.StatusInternalServerError, "failed_to_commit_request_transaction", err)
	}
}

func authenticatedUser(c *gin.Context) AuthenticatedUser {
	value, exists := c.Get("authenticated_user")
	if !exists {
		return AuthenticatedUser{}
	}
	user, _ := value.(AuthenticatedUser)
	return user
}

func authenticatedUserID(c *gin.Context) string {
	return authenticatedUser(c).ID
}

func requestDB(c *gin.Context) *gorm.DB {
	value, exists := c.Get("request_db")
	if !exists {
		return nil
	}
	db, _ := value.(*gorm.DB)
	return db
}

func (s *Server) requestDB(c *gin.Context) *gorm.DB {
	if db := requestDB(c); db != nil {
		return db
	}
	return s.db
}

func isPostgresDriver(driver string) bool {
	switch strings.ToLower(strings.TrimSpace(driver)) {
	case "postgres", "postgresql":
		return true
	default:
		return false
	}
}

func (s *Server) getSummary(c *gin.Context) {
	rangeKey := c.DefaultQuery("range", "month")
	summary, err := service.NewSummaryService(s.requestDB(c)).BuildForUser(rangeKey, time.Now(), authenticatedUserID(c))
	if err != nil {
		s.respondError(c, http.StatusInternalServerError, "failed_to_build_summary", err)
		return
	}
	c.JSON(http.StatusOK, summary)
}

type appPreferencePayload struct {
	ID                     *string    `json:"id"`
	CurrencyCode           string     `json:"currency_code" binding:"required"`
	MonthStartDay          int        `json:"month_start_day" binding:"required,gte=1,lte=28"`
	IsAISummariesEnabled   *bool      `json:"is_ai_summaries_enabled"`
	AppearanceRaw          string     `json:"appearance_raw" binding:"required,oneof=system light dark"`
	LanguageRaw            *string    `json:"language_raw"`
	HomeSummaryRangeRaw    *string    `json:"home_summary_range_raw"`
	HomeSelectedDate       *time.Time `json:"home_selected_date"`
	HomeRangeStartDate     *time.Time `json:"home_range_start_date"`
	HomeRangeEndDate       *time.Time `json:"home_range_end_date"`
	BudgetWarningThreshold float64    `json:"budget_warning_threshold" binding:"required"`
	SeedScenarioRaw        *string    `json:"seed_scenario_raw"`
	CreatedAt              *time.Time `json:"created_at"`
	UpdatedAt              *time.Time `json:"updated_at"`
}

func (s *Server) listPreferences(c *gin.Context) {
	var items []models.AppPreference
	userID := authenticatedUserID(c)
	if err := s.requestDB(c).Where("user_id = ?", userID).Order("created_at ASC").Find(&items).Error; err != nil {
		s.respondError(c, http.StatusInternalServerError, "failed_to_list_preferences", err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": items})
}

func (s *Server) createPreference(c *gin.Context) {
	var payload appPreferencePayload
	if !bindJSON(c, &payload) {
		return
	}

	item := models.AppPreference{
		UserID:                 authenticatedUserID(c),
		ID:                     stringValue(payload.ID),
		CurrencyCode:           payload.CurrencyCode,
		MonthStartDay:          payload.MonthStartDay,
		IsAISummariesEnabled:   boolValue(payload.IsAISummariesEnabled, true),
		AppearanceRaw:          payload.AppearanceRaw,
		LanguageRaw:            payload.LanguageRaw,
		HomeSummaryRangeRaw:    payload.HomeSummaryRangeRaw,
		HomeSelectedDate:       payload.HomeSelectedDate,
		HomeRangeStartDate:     payload.HomeRangeStartDate,
		HomeRangeEndDate:       payload.HomeRangeEndDate,
		BudgetWarningThreshold: payload.BudgetWarningThreshold,
		SeedScenarioRaw:        stringValueOr(payload.SeedScenarioRaw, "balanced"),
		CreatedAt:              timeValue(payload.CreatedAt, time.Now()),
		UpdatedAt:              timeValue(payload.UpdatedAt, time.Now()),
	}
	result := s.requestDB(c).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "id"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"currency_code",
			"month_start_day",
			"is_ai_summaries_enabled",
			"appearance_raw",
			"language_raw",
			"home_summary_range_raw",
			"home_selected_date",
			"home_range_start_date",
			"home_range_end_date",
			"budget_warning_threshold",
			"seed_scenario_raw",
			"created_at",
			"updated_at",
		}),
		Where: clause.Where{Exprs: []clause.Expression{
			clause.Expr{SQL: "app_preferences.user_id = ?", Vars: []any{item.UserID}},
		}},
	}).Create(&item)
	if result.Error != nil {
		s.respondError(c, http.StatusBadRequest, "failed_to_create_preference", result.Error)
		return
	}
	if result.RowsAffected == 0 {
		item.ID = ""
		if err := s.requestDB(c).Create(&item).Error; err != nil {
			s.respondError(c, http.StatusBadRequest, "failed_to_create_preference", err)
			return
		}
	}
	if err := s.overrideTimestamps(c, &item, payload.CreatedAt, payload.UpdatedAt); err != nil {
		s.respondError(c, http.StatusBadRequest, "failed_to_create_preference", err)
		return
	}
	if err := s.requestDB(c).First(&item, "id = ? AND user_id = ?", item.ID, item.UserID).Error; err != nil {
		s.respondError(c, http.StatusInternalServerError, "failed_to_load_preference", err)
		return
	}
	c.JSON(http.StatusCreated, item)
}

func (s *Server) updatePreference(c *gin.Context) {
	var item models.AppPreference
	userID := authenticatedUserID(c)
	if err := s.requestDB(c).First(&item, "id = ? AND user_id = ?", c.Param("id"), userID).Error; err != nil {
		s.respondError(c, http.StatusNotFound, "preference_not_found", err)
		return
	}

	var payload appPreferencePayload
	if !bindJSON(c, &payload) {
		return
	}

	item.CurrencyCode = payload.CurrencyCode
	item.MonthStartDay = payload.MonthStartDay
	item.IsAISummariesEnabled = boolValue(payload.IsAISummariesEnabled, item.IsAISummariesEnabled)
	item.AppearanceRaw = payload.AppearanceRaw
	item.LanguageRaw = payload.LanguageRaw
	item.HomeSummaryRangeRaw = payload.HomeSummaryRangeRaw
	item.HomeSelectedDate = payload.HomeSelectedDate
	item.HomeRangeStartDate = payload.HomeRangeStartDate
	item.HomeRangeEndDate = payload.HomeRangeEndDate
	item.BudgetWarningThreshold = payload.BudgetWarningThreshold
	item.SeedScenarioRaw = stringValueOr(payload.SeedScenarioRaw, item.SeedScenarioRaw)
	item.CreatedAt = timeValue(payload.CreatedAt, item.CreatedAt)
	item.UpdatedAt = timeValue(payload.UpdatedAt, item.UpdatedAt)

	if err := s.requestDB(c).Save(&item).Error; err != nil {
		s.respondError(c, http.StatusBadRequest, "failed_to_update_preference", err)
		return
	}
	if err := s.overrideTimestamps(c, &item, payload.CreatedAt, payload.UpdatedAt); err != nil {
		s.respondError(c, http.StatusBadRequest, "failed_to_update_preference", err)
		return
	}
	if err := s.requestDB(c).First(&item, "id = ? AND user_id = ?", item.ID, userID).Error; err != nil {
		s.respondError(c, http.StatusInternalServerError, "failed_to_load_preference", err)
		return
	}
	c.JSON(http.StatusOK, item)
}

func (s *Server) deletePreference(c *gin.Context) {
	userID := authenticatedUserID(c)
	if err := s.requestDB(c).Delete(&models.AppPreference{}, "id = ? AND user_id = ?", c.Param("id"), userID).Error; err != nil {
		s.respondError(c, http.StatusBadRequest, "failed_to_delete_preference", err)
		return
	}
	c.Status(http.StatusNoContent)
}

type snapshotBackupPayload struct {
	PayloadVersion   int    `json:"payload_version"`
	EncryptedPayload string `json:"encrypted_payload" binding:"required"`
}

func (s *Server) listSnapshotBackups(c *gin.Context) {
	var items []models.SnapshotBackup
	userID := authenticatedUserID(c)
	if err := s.requestDB(c).
		Select("id", "user_id", "restore_code", "payload_version", "created_at", "expires_at").
		Where("user_id = ?", userID).
		Order("created_at DESC").
		Find(&items).Error; err != nil {
		s.respondError(c, http.StatusInternalServerError, "failed_to_list_snapshots", err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": items})
}

func (s *Server) createSnapshotBackup(c *gin.Context) {
	var payload snapshotBackupPayload
	if !bindJSON(c, &payload) {
		return
	}
	if len(payload.EncryptedPayload) > 5*1024*1024 {
		s.respondError(c, http.StatusRequestEntityTooLarge, "snapshot_too_large", fmt.Errorf("snapshot payload must be 5MB or less"))
		return
	}

	version := payload.PayloadVersion
	if version <= 0 {
		version = 1
	}

	item := models.SnapshotBackup{
		UserID:           authenticatedUserID(c),
		PayloadVersion:   version,
		EncryptedPayload: payload.EncryptedPayload,
		ExpiresAt:        time.Now().Add(90 * 24 * time.Hour),
	}

	for attempts := 0; attempts < 5; attempts++ {
		restoreCode, err := generateRestoreCode()
		if err != nil {
			s.respondError(c, http.StatusInternalServerError, "failed_to_generate_restore_code", err)
			return
		}
		item.RestoreCode = restoreCode
		if err := s.requestDB(c).Create(&item).Error; err != nil {
			if attempts < 4 && strings.Contains(strings.ToLower(err.Error()), "unique") {
				continue
			}
			s.respondError(c, http.StatusBadRequest, "failed_to_create_snapshot", err)
			return
		}
		c.JSON(http.StatusCreated, snapshotBackupMetadata(item))
		return
	}

	s.respondError(c, http.StatusInternalServerError, "failed_to_create_snapshot", fmt.Errorf("restore code collision"))
}

func (s *Server) getSnapshotBackup(c *gin.Context) {
	restoreCode := normalizeRestoreCode(c.Param("restore_code"))
	if restoreCode == "" {
		s.respondError(c, http.StatusBadRequest, "invalid_restore_code", fmt.Errorf("restore code is required"))
		return
	}

	var item models.SnapshotBackup
	if err := s.requestDB(c).First(&item, "restore_code = ? AND expires_at > ?", restoreCode, time.Now()).Error; err != nil {
		s.respondError(c, http.StatusNotFound, "snapshot_not_found", err)
		return
	}
	c.JSON(http.StatusOK, item)
}

func (s *Server) deleteSnapshotBackup(c *gin.Context) {
	userID := authenticatedUserID(c)
	if err := s.requestDB(c).Delete(&models.SnapshotBackup{}, "id = ? AND user_id = ?", c.Param("id"), userID).Error; err != nil {
		s.respondError(c, http.StatusBadRequest, "failed_to_delete_snapshot", err)
		return
	}
	c.Status(http.StatusNoContent)
}

func snapshotBackupMetadata(item models.SnapshotBackup) gin.H {
	return gin.H{
		"id":              item.ID,
		"restore_code":    item.RestoreCode,
		"payload_version": item.PayloadVersion,
		"created_at":      item.CreatedAt,
		"expires_at":      item.ExpiresAt,
	}
}

func generateRestoreCode() (string, error) {
	bytes := make([]byte, 13)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	encoded := base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(bytes)
	encoded = strings.TrimRight(encoded, "=")
	if len(encoded) > 20 {
		encoded = encoded[:20]
	}
	return "MONEE-" + encoded[0:4] + "-" + encoded[4:8] + "-" + encoded[8:12] + "-" + encoded[12:16] + "-" + encoded[16:20], nil
}

func normalizeRestoreCode(raw string) string {
	raw = strings.ToUpper(strings.TrimSpace(raw))
	raw = strings.ReplaceAll(raw, " ", "")
	raw = strings.ReplaceAll(raw, "-", "")
	if strings.HasPrefix(raw, "MONEE") {
		raw = strings.TrimPrefix(raw, "MONEE")
	}
	if len(raw) != 20 {
		return ""
	}
	return "MONEE-" + raw[0:4] + "-" + raw[4:8] + "-" + raw[8:12] + "-" + raw[12:16] + "-" + raw[16:20]
}

type categoryPayload struct {
	ID         *string    `json:"id"`
	Slug       string     `json:"slug" binding:"required"`
	Name       string     `json:"name" binding:"required"`
	Type       string     `json:"type" binding:"required,oneof=expense income"`
	Icon       string     `json:"icon" binding:"required"`
	ColorToken string     `json:"color_token" binding:"required"`
	Order      int        `json:"order"`
	IsActive   *bool      `json:"is_active"`
	CreatedAt  *time.Time `json:"created_at"`
	UpdatedAt  *time.Time `json:"updated_at"`
}

func (s *Server) listCategories(c *gin.Context) {
	var categories []models.Category
	userID := authenticatedUserID(c)
	query := s.requestDB(c).
		Where("user_id = ?", userID).
		Order(clause.OrderByColumn{Column: clause.Column{Name: "order"}}).
		Order(clause.OrderByColumn{Column: clause.Column{Name: "created_at"}})
	if categoryType := strings.TrimSpace(c.Query("type")); categoryType != "" {
		query = query.Where("type = ?", categoryType)
	}
	if err := query.Find(&categories).Error; err != nil {
		s.respondError(c, http.StatusInternalServerError, "failed_to_list_categories", err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": categories})
}

func (s *Server) createCategory(c *gin.Context) {
	var payload categoryPayload
	if !bindJSON(c, &payload) {
		return
	}
	item := models.Category{
		UserID:     authenticatedUserID(c),
		ID:         stringValue(payload.ID),
		Slug:       payload.Slug,
		Name:       payload.Name,
		Type:       payload.Type,
		Icon:       payload.Icon,
		ColorToken: payload.ColorToken,
		Order:      payload.Order,
		IsActive:   boolValue(payload.IsActive, true),
		CreatedAt:  timeValue(payload.CreatedAt, time.Now()),
		UpdatedAt:  timeValue(payload.UpdatedAt, time.Now()),
	}
	if err := s.requestDB(c).Create(&item).Error; err != nil {
		s.respondError(c, http.StatusBadRequest, "failed_to_create_category", err)
		return
	}
	if err := s.overrideTimestamps(c, &item, payload.CreatedAt, payload.UpdatedAt); err != nil {
		s.respondError(c, http.StatusBadRequest, "failed_to_create_category", err)
		return
	}
	if err := s.requestDB(c).First(&item, "id = ? AND user_id = ?", item.ID, item.UserID).Error; err != nil {
		s.respondError(c, http.StatusInternalServerError, "failed_to_load_category", err)
		return
	}
	c.JSON(http.StatusCreated, item)
}

func (s *Server) updateCategory(c *gin.Context) {
	var item models.Category
	userID := authenticatedUserID(c)
	if err := s.requestDB(c).First(&item, "id = ? AND user_id = ?", c.Param("id"), userID).Error; err != nil {
		s.respondError(c, http.StatusNotFound, "category_not_found", err)
		return
	}
	var payload categoryPayload
	if !bindJSON(c, &payload) {
		return
	}
	item.Slug = payload.Slug
	item.Name = payload.Name
	item.Type = payload.Type
	item.Icon = payload.Icon
	item.ColorToken = payload.ColorToken
	item.Order = payload.Order
	item.IsActive = boolValue(payload.IsActive, item.IsActive)
	item.CreatedAt = timeValue(payload.CreatedAt, item.CreatedAt)
	item.UpdatedAt = timeValue(payload.UpdatedAt, item.UpdatedAt)
	if err := s.requestDB(c).Save(&item).Error; err != nil {
		s.respondError(c, http.StatusBadRequest, "failed_to_update_category", err)
		return
	}
	if err := s.overrideTimestamps(c, &item, payload.CreatedAt, payload.UpdatedAt); err != nil {
		s.respondError(c, http.StatusBadRequest, "failed_to_update_category", err)
		return
	}
	if err := s.requestDB(c).First(&item, "id = ? AND user_id = ?", item.ID, userID).Error; err != nil {
		s.respondError(c, http.StatusInternalServerError, "failed_to_load_category", err)
		return
	}
	c.JSON(http.StatusOK, item)
}

func (s *Server) deleteCategory(c *gin.Context) {
	userID := authenticatedUserID(c)
	if err := s.requestDB(c).Delete(&models.Category{}, "id = ? AND user_id = ?", c.Param("id"), userID).Error; err != nil {
		s.respondError(c, http.StatusBadRequest, "failed_to_delete_category", err)
		return
	}
	c.Status(http.StatusNoContent)
}

type transactionPayload struct {
	ID                      *string    `json:"id"`
	Title                   string     `json:"title" binding:"required"`
	Amount                  float64    `json:"amount" binding:"required"`
	Type                    string     `json:"type" binding:"required,oneof=expense income"`
	Date                    time.Time  `json:"date" binding:"required"`
	Note                    *string    `json:"note"`
	MerchantName            *string    `json:"merchant_name"`
	CategoryID              *string    `json:"category_id"`
	IsSubscriptionCandidate *bool      `json:"is_subscription_candidate"`
	RecurrenceHint          *string    `json:"recurrence_hint"`
	CreatedAt               *time.Time `json:"created_at"`
	UpdatedAt               *time.Time `json:"updated_at"`
}

func (s *Server) listTransactions(c *gin.Context) {
	var items []models.Transaction
	userID := authenticatedUserID(c)
	query := s.requestDB(c).
		Where("user_id = ?", userID).
		Preload("Category", "user_id = ?", userID).
		Order("date DESC").
		Order("created_at DESC")
	if txType := strings.TrimSpace(c.Query("type")); txType != "" {
		query = query.Where("type = ?", txType)
	}
	if categoryID := strings.TrimSpace(c.Query("category_id")); categoryID != "" {
		query = query.Where("category_id = ?", categoryID)
	}
	if limit := strings.TrimSpace(c.Query("limit")); limit != "" {
		query = query.Limit(parseInt(limit, 50))
	}
	if err := query.Find(&items).Error; err != nil {
		s.respondError(c, http.StatusInternalServerError, "failed_to_list_transactions", err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": items})
}

func (s *Server) createTransaction(c *gin.Context) {
	var payload transactionPayload
	if !bindJSON(c, &payload) {
		return
	}
	userID := authenticatedUserID(c)
	categoryID, ok := s.validatedCategoryID(c, userID, payload.CategoryID)
	if !ok {
		return
	}
	item := models.Transaction{
		UserID:                  userID,
		ID:                      stringValue(payload.ID),
		Title:                   payload.Title,
		Amount:                  payload.Amount,
		Type:                    payload.Type,
		Date:                    payload.Date,
		Note:                    payload.Note,
		MerchantName:            payload.MerchantName,
		CategoryID:              categoryID,
		IsSubscriptionCandidate: boolValue(payload.IsSubscriptionCandidate, false),
		RecurrenceHint:          payload.RecurrenceHint,
		CreatedAt:               timeValue(payload.CreatedAt, time.Now()),
		UpdatedAt:               timeValue(payload.UpdatedAt, time.Now()),
	}
	if err := s.requestDB(c).Create(&item).Error; err != nil {
		s.respondError(c, http.StatusBadRequest, "failed_to_create_transaction", err)
		return
	}
	if err := s.overrideTimestamps(c, &item, payload.CreatedAt, payload.UpdatedAt); err != nil {
		s.respondError(c, http.StatusBadRequest, "failed_to_create_transaction", err)
		return
	}
	if err := s.requestDB(c).Preload("Category", "user_id = ?", userID).First(&item, "id = ? AND user_id = ?", item.ID, userID).Error; err != nil {
		s.respondError(c, http.StatusInternalServerError, "failed_to_load_transaction", err)
		return
	}
	c.JSON(http.StatusCreated, item)
}

func (s *Server) updateTransaction(c *gin.Context) {
	var item models.Transaction
	userID := authenticatedUserID(c)
	if err := s.requestDB(c).First(&item, "id = ? AND user_id = ?", c.Param("id"), userID).Error; err != nil {
		s.respondError(c, http.StatusNotFound, "transaction_not_found", err)
		return
	}
	var payload transactionPayload
	if !bindJSON(c, &payload) {
		return
	}
	categoryID, ok := s.validatedCategoryID(c, userID, payload.CategoryID)
	if !ok {
		return
	}
	item.Title = payload.Title
	item.Amount = payload.Amount
	item.Type = payload.Type
	item.Date = payload.Date
	item.Note = payload.Note
	item.MerchantName = payload.MerchantName
	item.CategoryID = categoryID
	item.IsSubscriptionCandidate = boolValue(payload.IsSubscriptionCandidate, item.IsSubscriptionCandidate)
	item.RecurrenceHint = payload.RecurrenceHint
	item.CreatedAt = timeValue(payload.CreatedAt, item.CreatedAt)
	item.UpdatedAt = timeValue(payload.UpdatedAt, item.UpdatedAt)
	if err := s.requestDB(c).Save(&item).Error; err != nil {
		s.respondError(c, http.StatusBadRequest, "failed_to_update_transaction", err)
		return
	}
	if err := s.overrideTimestamps(c, &item, payload.CreatedAt, payload.UpdatedAt); err != nil {
		s.respondError(c, http.StatusBadRequest, "failed_to_update_transaction", err)
		return
	}
	if err := s.requestDB(c).Preload("Category", "user_id = ?", userID).First(&item, "id = ? AND user_id = ?", item.ID, userID).Error; err != nil {
		s.respondError(c, http.StatusInternalServerError, "failed_to_load_transaction", err)
		return
	}
	c.JSON(http.StatusOK, item)
}

func (s *Server) deleteTransaction(c *gin.Context) {
	userID := authenticatedUserID(c)
	if err := s.requestDB(c).Delete(&models.Transaction{}, "id = ? AND user_id = ?", c.Param("id"), userID).Error; err != nil {
		s.respondError(c, http.StatusBadRequest, "failed_to_delete_transaction", err)
		return
	}
	c.Status(http.StatusNoContent)
}

type budgetPayload struct {
	ID           *string    `json:"id"`
	Name         string     `json:"name" binding:"required"`
	Scope        string     `json:"scope" binding:"required,oneof=overall category"`
	MonthlyLimit float64    `json:"monthly_limit" binding:"required"`
	CategoryID   *string    `json:"category_id"`
	IsActive     *bool      `json:"is_active"`
	CreatedAt    *time.Time `json:"created_at"`
	UpdatedAt    *time.Time `json:"updated_at"`
}

func (s *Server) listBudgets(c *gin.Context) {
	var items []models.Budget
	userID := authenticatedUserID(c)
	if err := s.requestDB(c).Where("user_id = ?", userID).Preload("Category", "user_id = ?", userID).Order("created_at DESC").Find(&items).Error; err != nil {
		s.respondError(c, http.StatusInternalServerError, "failed_to_list_budgets", err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": items})
}

func (s *Server) createBudget(c *gin.Context) {
	var payload budgetPayload
	if !bindJSON(c, &payload) {
		return
	}
	userID := authenticatedUserID(c)
	categoryID, ok := s.validatedCategoryID(c, userID, payload.CategoryID)
	if !ok {
		return
	}
	item := models.Budget{
		UserID:       userID,
		ID:           stringValue(payload.ID),
		Name:         payload.Name,
		Scope:        payload.Scope,
		MonthlyLimit: payload.MonthlyLimit,
		CategoryID:   categoryID,
		IsActive:     boolValue(payload.IsActive, true),
		CreatedAt:    timeValue(payload.CreatedAt, time.Now()),
		UpdatedAt:    timeValue(payload.UpdatedAt, time.Now()),
	}
	if err := s.requestDB(c).Create(&item).Error; err != nil {
		s.respondError(c, http.StatusBadRequest, "failed_to_create_budget", err)
		return
	}
	if err := s.overrideTimestamps(c, &item, payload.CreatedAt, payload.UpdatedAt); err != nil {
		s.respondError(c, http.StatusBadRequest, "failed_to_create_budget", err)
		return
	}
	if err := s.requestDB(c).Preload("Category", "user_id = ?", userID).First(&item, "id = ? AND user_id = ?", item.ID, userID).Error; err != nil {
		s.respondError(c, http.StatusInternalServerError, "failed_to_load_budget", err)
		return
	}
	c.JSON(http.StatusCreated, item)
}

func (s *Server) updateBudget(c *gin.Context) {
	var item models.Budget
	userID := authenticatedUserID(c)
	if err := s.requestDB(c).First(&item, "id = ? AND user_id = ?", c.Param("id"), userID).Error; err != nil {
		s.respondError(c, http.StatusNotFound, "budget_not_found", err)
		return
	}
	var payload budgetPayload
	if !bindJSON(c, &payload) {
		return
	}
	categoryID, ok := s.validatedCategoryID(c, userID, payload.CategoryID)
	if !ok {
		return
	}
	item.Name = payload.Name
	item.Scope = payload.Scope
	item.MonthlyLimit = payload.MonthlyLimit
	item.CategoryID = categoryID
	item.IsActive = boolValue(payload.IsActive, item.IsActive)
	item.CreatedAt = timeValue(payload.CreatedAt, item.CreatedAt)
	item.UpdatedAt = timeValue(payload.UpdatedAt, item.UpdatedAt)
	if err := s.requestDB(c).Save(&item).Error; err != nil {
		s.respondError(c, http.StatusBadRequest, "failed_to_update_budget", err)
		return
	}
	if err := s.overrideTimestamps(c, &item, payload.CreatedAt, payload.UpdatedAt); err != nil {
		s.respondError(c, http.StatusBadRequest, "failed_to_update_budget", err)
		return
	}
	if err := s.requestDB(c).Preload("Category", "user_id = ?", userID).First(&item, "id = ? AND user_id = ?", item.ID, userID).Error; err != nil {
		s.respondError(c, http.StatusInternalServerError, "failed_to_load_budget", err)
		return
	}
	c.JSON(http.StatusOK, item)
}

func (s *Server) deleteBudget(c *gin.Context) {
	userID := authenticatedUserID(c)
	if err := s.requestDB(c).Delete(&models.Budget{}, "id = ? AND user_id = ?", c.Param("id"), userID).Error; err != nil {
		s.respondError(c, http.StatusBadRequest, "failed_to_delete_budget", err)
		return
	}
	c.Status(http.StatusNoContent)
}

type savingsGoalPayload struct {
	ID           *string    `json:"id"`
	Name         string     `json:"name" binding:"required"`
	TargetAmount float64    `json:"target_amount" binding:"required"`
	SavedAmount  float64    `json:"saved_amount"`
	TargetDate   *time.Time `json:"target_date"`
	IsActive     *bool      `json:"is_active"`
	CreatedAt    *time.Time `json:"created_at"`
	UpdatedAt    *time.Time `json:"updated_at"`
}

func (s *Server) listSavingsGoals(c *gin.Context) {
	var items []models.SavingsGoal
	userID := authenticatedUserID(c)
	if err := s.requestDB(c).Where("user_id = ?", userID).Order("created_at DESC").Find(&items).Error; err != nil {
		s.respondError(c, http.StatusInternalServerError, "failed_to_list_savings_goals", err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": items})
}

func (s *Server) createSavingsGoal(c *gin.Context) {
	var payload savingsGoalPayload
	if !bindJSON(c, &payload) {
		return
	}
	item := models.SavingsGoal{
		UserID:       authenticatedUserID(c),
		ID:           stringValue(payload.ID),
		Name:         payload.Name,
		TargetAmount: payload.TargetAmount,
		SavedAmount:  payload.SavedAmount,
		TargetDate:   payload.TargetDate,
		IsActive:     boolValue(payload.IsActive, true),
		CreatedAt:    timeValue(payload.CreatedAt, time.Now()),
		UpdatedAt:    timeValue(payload.UpdatedAt, time.Now()),
	}
	if err := s.requestDB(c).Create(&item).Error; err != nil {
		s.respondError(c, http.StatusBadRequest, "failed_to_create_savings_goal", err)
		return
	}
	if err := s.overrideTimestamps(c, &item, payload.CreatedAt, payload.UpdatedAt); err != nil {
		s.respondError(c, http.StatusBadRequest, "failed_to_create_savings_goal", err)
		return
	}
	if err := s.requestDB(c).First(&item, "id = ? AND user_id = ?", item.ID, item.UserID).Error; err != nil {
		s.respondError(c, http.StatusInternalServerError, "failed_to_load_savings_goal", err)
		return
	}
	c.JSON(http.StatusCreated, item)
}

func (s *Server) updateSavingsGoal(c *gin.Context) {
	var item models.SavingsGoal
	userID := authenticatedUserID(c)
	if err := s.requestDB(c).First(&item, "id = ? AND user_id = ?", c.Param("id"), userID).Error; err != nil {
		s.respondError(c, http.StatusNotFound, "savings_goal_not_found", err)
		return
	}
	var payload savingsGoalPayload
	if !bindJSON(c, &payload) {
		return
	}
	item.Name = payload.Name
	item.TargetAmount = payload.TargetAmount
	item.SavedAmount = payload.SavedAmount
	item.TargetDate = payload.TargetDate
	item.IsActive = boolValue(payload.IsActive, item.IsActive)
	item.CreatedAt = timeValue(payload.CreatedAt, item.CreatedAt)
	item.UpdatedAt = timeValue(payload.UpdatedAt, item.UpdatedAt)
	if err := s.requestDB(c).Save(&item).Error; err != nil {
		s.respondError(c, http.StatusBadRequest, "failed_to_update_savings_goal", err)
		return
	}
	if err := s.overrideTimestamps(c, &item, payload.CreatedAt, payload.UpdatedAt); err != nil {
		s.respondError(c, http.StatusBadRequest, "failed_to_update_savings_goal", err)
		return
	}
	if err := s.requestDB(c).First(&item, "id = ? AND user_id = ?", item.ID, userID).Error; err != nil {
		s.respondError(c, http.StatusInternalServerError, "failed_to_load_savings_goal", err)
		return
	}
	c.JSON(http.StatusOK, item)
}

func (s *Server) deleteSavingsGoal(c *gin.Context) {
	userID := authenticatedUserID(c)
	if err := s.requestDB(c).Delete(&models.SavingsGoal{}, "id = ? AND user_id = ?", c.Param("id"), userID).Error; err != nil {
		s.respondError(c, http.StatusBadRequest, "failed_to_delete_savings_goal", err)
		return
	}
	c.Status(http.StatusNoContent)
}

type subscriptionPayload struct {
	ID                       *string    `json:"id"`
	MerchantKey              string     `json:"merchant_key" binding:"required"`
	DisplayName              string     `json:"display_name" binding:"required"`
	Label                    string     `json:"label" binding:"required"`
	AverageAmount            float64    `json:"average_amount" binding:"required"`
	Cadence                  string     `json:"cadence" binding:"required,oneof=monthly yearly irregular"`
	State                    string     `json:"state" binding:"required,oneof=active uncertain rejected canceled"`
	EstimatedNextBillingDate *time.Time `json:"estimated_next_billing_date"`
	LastChargeDate           *time.Time `json:"last_charge_date"`
	MonthlyEquivalentAmount  float64    `json:"monthly_equivalent_amount" binding:"required"`
	YearlyEquivalentAmount   float64    `json:"yearly_equivalent_amount" binding:"required"`
	LatestTransactionTitle   *string    `json:"latest_transaction_title"`
	CreatedAt                *time.Time `json:"created_at"`
	UpdatedAt                *time.Time `json:"updated_at"`
}

func (s *Server) listSubscriptions(c *gin.Context) {
	var items []models.SubscriptionRecord
	userID := authenticatedUserID(c)
	if err := s.requestDB(c).Where("user_id = ?", userID).Order("updated_at DESC").Find(&items).Error; err != nil {
		s.respondError(c, http.StatusInternalServerError, "failed_to_list_subscriptions", err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": items})
}

func (s *Server) createSubscription(c *gin.Context) {
	var payload subscriptionPayload
	if !bindJSON(c, &payload) {
		return
	}
	item := models.SubscriptionRecord{
		UserID:                   authenticatedUserID(c),
		ID:                       stringValue(payload.ID),
		MerchantKey:              payload.MerchantKey,
		DisplayName:              payload.DisplayName,
		Label:                    payload.Label,
		AverageAmount:            payload.AverageAmount,
		Cadence:                  payload.Cadence,
		State:                    payload.State,
		EstimatedNextBillingDate: payload.EstimatedNextBillingDate,
		LastChargeDate:           payload.LastChargeDate,
		MonthlyEquivalentAmount:  payload.MonthlyEquivalentAmount,
		YearlyEquivalentAmount:   payload.YearlyEquivalentAmount,
		LatestTransactionTitle:   payload.LatestTransactionTitle,
		CreatedAt:                timeValue(payload.CreatedAt, time.Now()),
		UpdatedAt:                timeValue(payload.UpdatedAt, time.Now()),
	}
	if err := s.requestDB(c).Create(&item).Error; err != nil {
		s.respondError(c, http.StatusBadRequest, "failed_to_create_subscription", err)
		return
	}
	if err := s.overrideTimestamps(c, &item, payload.CreatedAt, payload.UpdatedAt); err != nil {
		s.respondError(c, http.StatusBadRequest, "failed_to_create_subscription", err)
		return
	}
	if err := s.requestDB(c).First(&item, "id = ? AND user_id = ?", item.ID, item.UserID).Error; err != nil {
		s.respondError(c, http.StatusInternalServerError, "failed_to_load_subscription", err)
		return
	}
	c.JSON(http.StatusCreated, item)
}

func (s *Server) updateSubscription(c *gin.Context) {
	var item models.SubscriptionRecord
	userID := authenticatedUserID(c)
	if err := s.requestDB(c).First(&item, "id = ? AND user_id = ?", c.Param("id"), userID).Error; err != nil {
		s.respondError(c, http.StatusNotFound, "subscription_not_found", err)
		return
	}
	var payload subscriptionPayload
	if !bindJSON(c, &payload) {
		return
	}
	item.MerchantKey = payload.MerchantKey
	item.DisplayName = payload.DisplayName
	item.Label = payload.Label
	item.AverageAmount = payload.AverageAmount
	item.Cadence = payload.Cadence
	item.State = payload.State
	item.EstimatedNextBillingDate = payload.EstimatedNextBillingDate
	item.LastChargeDate = payload.LastChargeDate
	item.MonthlyEquivalentAmount = payload.MonthlyEquivalentAmount
	item.YearlyEquivalentAmount = payload.YearlyEquivalentAmount
	item.LatestTransactionTitle = payload.LatestTransactionTitle
	item.CreatedAt = timeValue(payload.CreatedAt, item.CreatedAt)
	item.UpdatedAt = timeValue(payload.UpdatedAt, item.UpdatedAt)
	if err := s.requestDB(c).Save(&item).Error; err != nil {
		s.respondError(c, http.StatusBadRequest, "failed_to_update_subscription", err)
		return
	}
	if err := s.overrideTimestamps(c, &item, payload.CreatedAt, payload.UpdatedAt); err != nil {
		s.respondError(c, http.StatusBadRequest, "failed_to_update_subscription", err)
		return
	}
	if err := s.requestDB(c).First(&item, "id = ? AND user_id = ?", item.ID, userID).Error; err != nil {
		s.respondError(c, http.StatusInternalServerError, "failed_to_load_subscription", err)
		return
	}
	c.JSON(http.StatusOK, item)
}

func (s *Server) deleteSubscription(c *gin.Context) {
	userID := authenticatedUserID(c)
	if err := s.requestDB(c).Delete(&models.SubscriptionRecord{}, "id = ? AND user_id = ?", c.Param("id"), userID).Error; err != nil {
		s.respondError(c, http.StatusBadRequest, "failed_to_delete_subscription", err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (s *Server) validatedCategoryID(c *gin.Context, userID string, rawValue *string) (*string, bool) {
	if rawValue == nil {
		return nil, true
	}

	categoryID := strings.TrimSpace(*rawValue)
	if categoryID == "" {
		return nil, true
	}

	var category models.Category
	if err := s.requestDB(c).Select("id").First(&category, "id = ? AND user_id = ?", categoryID, userID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "category_not_found", "message": "category_id must belong to the authenticated user"})
			return nil, false
		}
		s.respondError(c, http.StatusInternalServerError, "failed_to_validate_category", err)
		return nil, false
	}

	return &categoryID, true
}

func (s *Server) respondError(c *gin.Context, status int, code string, err error) {
	s.logger.Error(code, "error", err, "path", c.Request.URL.Path)
	c.JSON(status, gin.H{"error": code, "message": err.Error()})
}

func bindJSON(c *gin.Context, payload any) bool {
	if err := c.ShouldBindJSON(payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request", "message": err.Error()})
		return false
	}
	return true
}

func boolValue(value *bool, fallback bool) bool {
	if value != nil {
		return *value
	}
	return fallback
}

func stringValue(value *string) string {
	if value == nil {
		return ""
	}
	return strings.TrimSpace(*value)
}

func stringValueOr(value *string, fallback string) string {
	if trimmed := stringValue(value); trimmed != "" {
		return trimmed
	}
	return fallback
}

func timeValue(value *time.Time, fallback time.Time) time.Time {
	if value != nil {
		return *value
	}
	return fallback
}

func parseInt(raw string, fallback int) int {
	var out int
	_, err := fmt.Sscanf(raw, "%d", &out)
	if err != nil || out <= 0 {
		return fallback
	}
	return out
}

func (s *Server) overrideTimestamps(c *gin.Context, model any, createdAt, updatedAt *time.Time) error {
	updates := map[string]any{}
	if createdAt != nil {
		updates["created_at"] = *createdAt
	}
	if updatedAt != nil {
		updates["updated_at"] = *updatedAt
	}
	if len(updates) == 0 {
		return nil
	}
	return s.requestDB(c).Model(model).UpdateColumns(updates).Error
}
