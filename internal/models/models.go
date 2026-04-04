package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Category struct {
	ID         string    `gorm:"primaryKey" json:"id"`
	Slug       string    `gorm:"uniqueIndex;not null" json:"slug"`
	Name       string    `gorm:"not null" json:"name"`
	Type       string    `gorm:"not null;index" json:"type"`
	Icon       string    `gorm:"not null" json:"icon"`
	ColorToken string    `gorm:"not null" json:"color_token"`
	Order      int       `gorm:"not null;default:0" json:"order"`
	IsActive   bool      `gorm:"not null;default:true" json:"is_active"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

type Transaction struct {
	ID                      string     `gorm:"primaryKey" json:"id"`
	Title                   string     `gorm:"not null" json:"title"`
	Amount                  float64    `gorm:"not null" json:"amount"`
	Type                    string     `gorm:"not null;index" json:"type"`
	Date                    time.Time  `gorm:"not null;index" json:"date"`
	Note                    *string    `json:"note,omitempty"`
	MerchantName            *string    `json:"merchant_name,omitempty"`
	CategoryID              *string    `gorm:"index" json:"category_id,omitempty"`
	Category                *Category  `json:"category,omitempty"`
	IsSubscriptionCandidate bool       `gorm:"not null;default:false" json:"is_subscription_candidate"`
	RecurrenceHint          *string    `json:"recurrence_hint,omitempty"`
	CreatedAt               time.Time  `json:"created_at"`
	UpdatedAt               time.Time  `json:"updated_at"`
}

type Budget struct {
	ID           string     `gorm:"primaryKey" json:"id"`
	Name         string     `gorm:"not null" json:"name"`
	Scope        string     `gorm:"not null;index" json:"scope"`
	MonthlyLimit float64    `gorm:"not null" json:"monthly_limit"`
	CategoryID   *string    `gorm:"index" json:"category_id,omitempty"`
	Category     *Category  `json:"category,omitempty"`
	IsActive     bool       `gorm:"not null;default:true" json:"is_active"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
}

type SavingsGoal struct {
	ID           string     `gorm:"primaryKey" json:"id"`
	Name         string     `gorm:"not null" json:"name"`
	TargetAmount float64    `gorm:"not null" json:"target_amount"`
	SavedAmount  float64    `gorm:"not null;default:0" json:"saved_amount"`
	TargetDate   *time.Time `json:"target_date,omitempty"`
	IsActive     bool       `gorm:"not null;default:true" json:"is_active"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
}

type Summary struct {
	Range            string        `json:"range"`
	StartDate        *time.Time    `json:"start_date,omitempty"`
	EndDate          *time.Time    `json:"end_date,omitempty"`
	ExpenseTotal     float64       `json:"expense_total"`
	IncomeTotal      float64       `json:"income_total"`
	NetBalance       float64       `json:"net_balance"`
	TransactionCount int64         `json:"transaction_count"`
	Recent           []Transaction `json:"recent_transactions"`
}

func (c *Category) BeforeCreate(_ *gorm.DB) error {
	if c.ID == "" {
		c.ID = uuid.NewString()
	}
	return nil
}

func (t *Transaction) BeforeCreate(_ *gorm.DB) error {
	if t.ID == "" {
		t.ID = uuid.NewString()
	}
	return nil
}

func (b *Budget) BeforeCreate(_ *gorm.DB) error {
	if b.ID == "" {
		b.ID = uuid.NewString()
	}
	return nil
}

func (s *SavingsGoal) BeforeCreate(_ *gorm.DB) error {
	if s.ID == "" {
		s.ID = uuid.NewString()
	}
	return nil
}
