package http

import (
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/Suuu-sh/Monee_Backend/internal/config"
	"github.com/Suuu-sh/Monee_Backend/internal/models"
	"github.com/Suuu-sh/Monee_Backend/internal/service"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type Server struct {
	cfg            config.Config
	db             *gorm.DB
	logger         *slog.Logger
	summaryService *service.SummaryService
}

func NewRouter(cfg config.Config, db *gorm.DB, logger *slog.Logger) *gin.Engine {
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

	server := &Server{cfg: cfg, db: db, logger: logger, summaryService: service.NewSummaryService(db)}

	router.GET("/healthz", server.healthz)
	router.GET("/readyz", server.readyz)

	api := router.Group("/api/v1")
	{
		api.GET("/summary", server.getSummary)
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

func (s *Server) getSummary(c *gin.Context) {
	rangeKey := c.DefaultQuery("range", "month")
	summary, err := s.summaryService.Build(rangeKey, time.Now())
	if err != nil {
		s.respondError(c, http.StatusInternalServerError, "failed_to_build_summary", err)
		return
	}
	c.JSON(http.StatusOK, summary)
}

func (s *Server) listCategories(c *gin.Context) {
	var categories []models.Category
	query := s.db.Order("`order` ASC, created_at ASC")
	if categoryType := strings.TrimSpace(c.Query("type")); categoryType != "" {
		query = query.Where("type = ?", categoryType)
	}
	if err := query.Find(&categories).Error; err != nil {
		s.respondError(c, http.StatusInternalServerError, "failed_to_list_categories", err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": categories})
}

type categoryPayload struct {
	Slug       string `json:"slug" binding:"required"`
	Name       string `json:"name" binding:"required"`
	Type       string `json:"type" binding:"required,oneof=expense income"`
	Icon       string `json:"icon" binding:"required"`
	ColorToken string `json:"color_token" binding:"required"`
	Order      int    `json:"order"`
	IsActive   *bool  `json:"is_active"`
}

func (s *Server) createCategory(c *gin.Context) {
	var payload categoryPayload
	if !bindJSON(c, &payload) {
		return
	}
	item := models.Category{Slug: payload.Slug, Name: payload.Name, Type: payload.Type, Icon: payload.Icon, ColorToken: payload.ColorToken, Order: payload.Order, IsActive: boolValue(payload.IsActive, true)}
	if err := s.db.Create(&item).Error; err != nil {
		s.respondError(c, http.StatusBadRequest, "failed_to_create_category", err)
		return
	}
	c.JSON(http.StatusCreated, item)
}

func (s *Server) updateCategory(c *gin.Context) {
	var item models.Category
	if err := s.db.First(&item, "id = ?", c.Param("id")).Error; err != nil {
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
	if err := s.db.Save(&item).Error; err != nil {
		s.respondError(c, http.StatusBadRequest, "failed_to_update_category", err)
		return
	}
	c.JSON(http.StatusOK, item)
}

func (s *Server) deleteCategory(c *gin.Context) {
	if err := s.db.Delete(&models.Category{}, "id = ?", c.Param("id")).Error; err != nil {
		s.respondError(c, http.StatusBadRequest, "failed_to_delete_category", err)
		return
	}
	c.Status(http.StatusNoContent)
}

type transactionPayload struct {
	Title                   string     `json:"title" binding:"required"`
	Amount                  float64    `json:"amount" binding:"required"`
	Type                    string     `json:"type" binding:"required,oneof=expense income"`
	Date                    time.Time  `json:"date" binding:"required"`
	Note                    *string    `json:"note"`
	MerchantName            *string    `json:"merchant_name"`
	CategoryID              *string    `json:"category_id"`
	IsSubscriptionCandidate *bool      `json:"is_subscription_candidate"`
	RecurrenceHint          *string    `json:"recurrence_hint"`
}

func (s *Server) listTransactions(c *gin.Context) {
	var items []models.Transaction
	query := s.db.Preload("Category").Order("date DESC").Order("created_at DESC")
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
	item := models.Transaction{
		Title:                   payload.Title,
		Amount:                  payload.Amount,
		Type:                    payload.Type,
		Date:                    payload.Date,
		Note:                    payload.Note,
		MerchantName:            payload.MerchantName,
		CategoryID:              payload.CategoryID,
		IsSubscriptionCandidate: boolValue(payload.IsSubscriptionCandidate, false),
		RecurrenceHint:          payload.RecurrenceHint,
	}
	if err := s.db.Create(&item).Error; err != nil {
		s.respondError(c, http.StatusBadRequest, "failed_to_create_transaction", err)
		return
	}
	if err := s.db.Preload("Category").First(&item, "id = ?", item.ID).Error; err != nil {
		s.respondError(c, http.StatusInternalServerError, "failed_to_load_transaction", err)
		return
	}
	c.JSON(http.StatusCreated, item)
}

func (s *Server) updateTransaction(c *gin.Context) {
	var item models.Transaction
	if err := s.db.First(&item, "id = ?", c.Param("id")).Error; err != nil {
		s.respondError(c, http.StatusNotFound, "transaction_not_found", err)
		return
	}
	var payload transactionPayload
	if !bindJSON(c, &payload) {
		return
	}
	item.Title = payload.Title
	item.Amount = payload.Amount
	item.Type = payload.Type
	item.Date = payload.Date
	item.Note = payload.Note
	item.MerchantName = payload.MerchantName
	item.CategoryID = payload.CategoryID
	item.IsSubscriptionCandidate = boolValue(payload.IsSubscriptionCandidate, item.IsSubscriptionCandidate)
	item.RecurrenceHint = payload.RecurrenceHint
	if err := s.db.Save(&item).Error; err != nil {
		s.respondError(c, http.StatusBadRequest, "failed_to_update_transaction", err)
		return
	}
	if err := s.db.Preload("Category").First(&item, "id = ?", item.ID).Error; err != nil {
		s.respondError(c, http.StatusInternalServerError, "failed_to_load_transaction", err)
		return
	}
	c.JSON(http.StatusOK, item)
}

func (s *Server) deleteTransaction(c *gin.Context) {
	if err := s.db.Delete(&models.Transaction{}, "id = ?", c.Param("id")).Error; err != nil {
		s.respondError(c, http.StatusBadRequest, "failed_to_delete_transaction", err)
		return
	}
	c.Status(http.StatusNoContent)
}

type budgetPayload struct {
	Name         string  `json:"name" binding:"required"`
	Scope        string  `json:"scope" binding:"required,oneof=overall category"`
	MonthlyLimit float64 `json:"monthly_limit" binding:"required"`
	CategoryID   *string `json:"category_id"`
	IsActive     *bool   `json:"is_active"`
}

func (s *Server) listBudgets(c *gin.Context) {
	var items []models.Budget
	if err := s.db.Preload("Category").Order("created_at DESC").Find(&items).Error; err != nil {
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
	item := models.Budget{Name: payload.Name, Scope: payload.Scope, MonthlyLimit: payload.MonthlyLimit, CategoryID: payload.CategoryID, IsActive: boolValue(payload.IsActive, true)}
	if err := s.db.Create(&item).Error; err != nil {
		s.respondError(c, http.StatusBadRequest, "failed_to_create_budget", err)
		return
	}
	if err := s.db.Preload("Category").First(&item, "id = ?", item.ID).Error; err != nil {
		s.respondError(c, http.StatusInternalServerError, "failed_to_load_budget", err)
		return
	}
	c.JSON(http.StatusCreated, item)
}

func (s *Server) updateBudget(c *gin.Context) {
	var item models.Budget
	if err := s.db.First(&item, "id = ?", c.Param("id")).Error; err != nil {
		s.respondError(c, http.StatusNotFound, "budget_not_found", err)
		return
	}
	var payload budgetPayload
	if !bindJSON(c, &payload) {
		return
	}
	item.Name = payload.Name
	item.Scope = payload.Scope
	item.MonthlyLimit = payload.MonthlyLimit
	item.CategoryID = payload.CategoryID
	item.IsActive = boolValue(payload.IsActive, item.IsActive)
	if err := s.db.Save(&item).Error; err != nil {
		s.respondError(c, http.StatusBadRequest, "failed_to_update_budget", err)
		return
	}
	if err := s.db.Preload("Category").First(&item, "id = ?", item.ID).Error; err != nil {
		s.respondError(c, http.StatusInternalServerError, "failed_to_load_budget", err)
		return
	}
	c.JSON(http.StatusOK, item)
}

func (s *Server) deleteBudget(c *gin.Context) {
	if err := s.db.Delete(&models.Budget{}, "id = ?", c.Param("id")).Error; err != nil {
		s.respondError(c, http.StatusBadRequest, "failed_to_delete_budget", err)
		return
	}
	c.Status(http.StatusNoContent)
}

type savingsGoalPayload struct {
	Name         string     `json:"name" binding:"required"`
	TargetAmount float64    `json:"target_amount" binding:"required"`
	SavedAmount  float64    `json:"saved_amount"`
	TargetDate   *time.Time `json:"target_date"`
	IsActive     *bool      `json:"is_active"`
}

func (s *Server) listSavingsGoals(c *gin.Context) {
	var items []models.SavingsGoal
	if err := s.db.Order("created_at DESC").Find(&items).Error; err != nil {
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
	item := models.SavingsGoal{Name: payload.Name, TargetAmount: payload.TargetAmount, SavedAmount: payload.SavedAmount, TargetDate: payload.TargetDate, IsActive: boolValue(payload.IsActive, true)}
	if err := s.db.Create(&item).Error; err != nil {
		s.respondError(c, http.StatusBadRequest, "failed_to_create_savings_goal", err)
		return
	}
	c.JSON(http.StatusCreated, item)
}

func (s *Server) updateSavingsGoal(c *gin.Context) {
	var item models.SavingsGoal
	if err := s.db.First(&item, "id = ?", c.Param("id")).Error; err != nil {
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
	if err := s.db.Save(&item).Error; err != nil {
		s.respondError(c, http.StatusBadRequest, "failed_to_update_savings_goal", err)
		return
	}
	c.JSON(http.StatusOK, item)
}

func (s *Server) deleteSavingsGoal(c *gin.Context) {
	if err := s.db.Delete(&models.SavingsGoal{}, "id = ?", c.Param("id")).Error; err != nil {
		s.respondError(c, http.StatusBadRequest, "failed_to_delete_savings_goal", err)
		return
	}
	c.Status(http.StatusNoContent)
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

func parseInt(raw string, fallback int) int {
	var out int
	_, err := fmt.Sscanf(raw, "%d", &out)
	if err != nil || out <= 0 {
		return fallback
	}
	return out
}
