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
		{Slug: "food", Name: "食費", Type: "expense", Icon: "fork.knife", ColorToken: "coral", Order: 10, IsActive: true, CreatedAt: now, UpdatedAt: now},
		{Slug: "cafe", Name: "カフェ", Type: "expense", Icon: "cup.and.saucer.fill", ColorToken: "amber", Order: 20, IsActive: true, CreatedAt: now, UpdatedAt: now},
		{Slug: "transportation", Name: "交通", Type: "expense", Icon: "tram.fill", ColorToken: "sky", Order: 30, IsActive: true, CreatedAt: now, UpdatedAt: now},
		{Slug: "shopping", Name: "買い物", Type: "expense", Icon: "bag.fill", ColorToken: "rose", Order: 40, IsActive: true, CreatedAt: now, UpdatedAt: now},
		{Slug: "entertainment", Name: "娯楽", Type: "expense", Icon: "gamecontroller.fill", ColorToken: "violet", Order: 50, IsActive: true, CreatedAt: now, UpdatedAt: now},
		{Slug: "rent", Name: "家賃", Type: "expense", Icon: "house.fill", ColorToken: "navy", Order: 60, IsActive: true, CreatedAt: now, UpdatedAt: now},
		{Slug: "utilities", Name: "公共料金", Type: "expense", Icon: "bolt.fill", ColorToken: "amber", Order: 70, IsActive: true, CreatedAt: now, UpdatedAt: now},
		{Slug: "travel", Name: "旅行", Type: "expense", Icon: "airplane", ColorToken: "teal", Order: 80, IsActive: true, CreatedAt: now, UpdatedAt: now},
		{Slug: "salary", Name: "給与", Type: "income", Icon: "banknote.fill", ColorToken: "emerald", Order: 10, IsActive: true, CreatedAt: now, UpdatedAt: now},
		{Slug: "bonus", Name: "ボーナス", Type: "income", Icon: "gift.fill", ColorToken: "mint", Order: 20, IsActive: true, CreatedAt: now, UpdatedAt: now},
		{Slug: "side_income", Name: "副収入", Type: "income", Icon: "plus.circle.fill", ColorToken: "lime", Order: 30, IsActive: true, CreatedAt: now, UpdatedAt: now},
	}

	return db.Create(&categories).Error
}
