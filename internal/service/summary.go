package service

import (
	"time"

	"github.com/Suuu-sh/Monee_Backend/internal/models"
	"gorm.io/gorm"
)

type SummaryService struct {
	db *gorm.DB
}

func NewSummaryService(db *gorm.DB) *SummaryService {
	return &SummaryService{db: db}
}

func (s *SummaryService) Build(rangeKey string, now time.Time) (models.Summary, error) {
	return s.BuildForUser(rangeKey, now, "")
}

func (s *SummaryService) BuildForUser(rangeKey string, now time.Time, userID string) (models.Summary, error) {
	var summary models.Summary
	var start, end *time.Time

	if rangeKey != "all_data" && rangeKey != "allData" {
		intervalStart, intervalEnd := rangeInterval(rangeKey, now)
		start, end = &intervalStart, &intervalEnd
	}

	query := s.db.Model(&models.Transaction{})
	if userID != "" {
		query = query.Where("user_id = ?", userID)
	}
	if start != nil && end != nil {
		query = query.Where("date >= ? AND date < ?", *start, *end)
	}

	var transactions []models.Transaction
	preloadQuery := query
	if userID != "" {
		preloadQuery = preloadQuery.Preload("Category", "user_id = ?", userID)
	} else {
		preloadQuery = preloadQuery.Preload("Category")
	}
	if err := preloadQuery.Order("date DESC").Limit(10).Find(&transactions).Error; err != nil {
		return summary, err
	}

	var allScoped []models.Transaction
	countQuery := s.db.Model(&models.Transaction{})
	if userID != "" {
		countQuery = countQuery.Where("user_id = ?", userID)
	}
	if start != nil && end != nil {
		countQuery = countQuery.Where("date >= ? AND date < ?", *start, *end)
	}
	if err := countQuery.Find(&allScoped).Error; err != nil {
		return summary, err
	}

	var expense, income float64
	for _, item := range allScoped {
		if item.Type == "income" {
			income += item.Amount
		} else {
			expense += item.Amount
		}
	}

	summary = models.Summary{
		Range:            rangeKey,
		StartDate:        start,
		EndDate:          end,
		ExpenseTotal:     expense,
		IncomeTotal:      income,
		NetBalance:       income - expense,
		TransactionCount: int64(len(allScoped)),
		Recent:           transactions,
	}
	return summary, nil
}

func rangeInterval(rangeKey string, now time.Time) (time.Time, time.Time) {
	calendar := time.Now().Location()
	current := now.In(calendar)
	startOfDay := time.Date(current.Year(), current.Month(), current.Day(), 0, 0, 0, 0, current.Location())
	switch rangeKey {
	case "today":
		return startOfDay, startOfDay.Add(24 * time.Hour)
	case "week":
		return startOfDay.AddDate(0, 0, -6), startOfDay.Add(24 * time.Hour)
	case "year":
		monthStart := time.Date(current.Year(), current.Month(), 1, 0, 0, 0, 0, current.Location())
		return monthStart.AddDate(0, -11, 0), monthStart.AddDate(0, 1, 0)
	case "three_months", "threeMonths":
		monthStart := time.Date(current.Year(), current.Month(), 1, 0, 0, 0, 0, current.Location())
		return monthStart.AddDate(0, -2, 0), monthStart.AddDate(0, 1, 0)
	case "six_months", "sixMonths":
		monthStart := time.Date(current.Year(), current.Month(), 1, 0, 0, 0, 0, current.Location())
		return monthStart.AddDate(0, -5, 0), monthStart.AddDate(0, 1, 0)
	case "month":
		fallthrough
	default:
		monthStart := time.Date(current.Year(), current.Month(), 1, 0, 0, 0, 0, current.Location())
		return monthStart, monthStart.AddDate(0, 1, 0)
	}
}
