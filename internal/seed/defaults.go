package seed

import (
	"time"

	"github.com/Suuu-sh/Monee_Backend/internal/models"
	"gorm.io/gorm"
)

func EnsureDefaults(db *gorm.DB) error {
	var count int64
	if err := db.Model(&models.Category{}).Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return nil
	}

	now := time.Now()
	categories := []models.Category{
		{Slug: "food", Name: "食費", Type: "expense", Icon: "fork.knife", ColorToken: "coral", Order: 0, IsActive: true, CreatedAt: now, UpdatedAt: now},
		{Slug: "cafe", Name: "カフェ", Type: "expense", Icon: "cup.and.saucer.fill", ColorToken: "amber", Order: 1, IsActive: true, CreatedAt: now, UpdatedAt: now},
		{Slug: "transportation", Name: "交通", Type: "expense", Icon: "tram.fill", ColorToken: "sky", Order: 2, IsActive: true, CreatedAt: now, UpdatedAt: now},
		{Slug: "rent", Name: "家賃", Type: "expense", Icon: "house.fill", ColorToken: "navy", Order: 3, IsActive: true, CreatedAt: now, UpdatedAt: now},
		{Slug: "utilities", Name: "公共料金", Type: "expense", Icon: "bolt.fill", ColorToken: "gold", Order: 4, IsActive: true, CreatedAt: now, UpdatedAt: now},
		{Slug: "shopping", Name: "買い物", Type: "expense", Icon: "bag.fill", ColorToken: "rose", Order: 5, IsActive: true, CreatedAt: now, UpdatedAt: now},
		{Slug: "entertainment", Name: "娯楽", Type: "expense", Icon: "sparkles.tv.fill", ColorToken: "violet", Order: 6, IsActive: true, CreatedAt: now, UpdatedAt: now},
		{Slug: "medical", Name: "医療", Type: "expense", Icon: "cross.case.fill", ColorToken: "mint", Order: 7, IsActive: true, CreatedAt: now, UpdatedAt: now},
		{Slug: "education", Name: "教育", Type: "expense", Icon: "book.closed.fill", ColorToken: "indigo", Order: 8, IsActive: true, CreatedAt: now, UpdatedAt: now},
		{Slug: "beauty", Name: "美容", Type: "expense", Icon: "face.smiling.fill", ColorToken: "peach", Order: 9, IsActive: true, CreatedAt: now, UpdatedAt: now},
		{Slug: "travel", Name: "旅行", Type: "expense", Icon: "airplane", ColorToken: "teal", Order: 10, IsActive: true, CreatedAt: now, UpdatedAt: now},
		{Slug: "subscription", Name: "固定費", Type: "expense", Icon: "repeat.circle.fill", ColorToken: "emerald", Order: 11, IsActive: true, CreatedAt: now, UpdatedAt: now},
		{Slug: "other_expense", Name: "その他", Type: "expense", Icon: "ellipsis.circle.fill", ColorToken: "slate", Order: 12, IsActive: true, CreatedAt: now, UpdatedAt: now},
		{Slug: "salary", Name: "給与", Type: "income", Icon: "banknote.fill", ColorToken: "emerald", Order: 13, IsActive: true, CreatedAt: now, UpdatedAt: now},
		{Slug: "bonus", Name: "ボーナス", Type: "income", Icon: "gift.fill", ColorToken: "gold", Order: 14, IsActive: true, CreatedAt: now, UpdatedAt: now},
		{Slug: "side_income", Name: "副収入", Type: "income", Icon: "briefcase.fill", ColorToken: "indigo", Order: 15, IsActive: true, CreatedAt: now, UpdatedAt: now},
		{Slug: "refund", Name: "返金", Type: "income", Icon: "arrow.uturn.backward.circle.fill", ColorToken: "mint", Order: 16, IsActive: true, CreatedAt: now, UpdatedAt: now},
		{Slug: "other_income", Name: "その他", Type: "income", Icon: "plus.circle.fill", ColorToken: "sky", Order: 17, IsActive: true, CreatedAt: now, UpdatedAt: now},
	}

	return db.Create(&categories).Error
}
