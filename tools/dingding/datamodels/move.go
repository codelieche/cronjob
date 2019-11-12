package datamodels

import "github.com/jinzhu/gorm"

type Movie struct {
	gorm.Model
	Title       string `gorm:"type:title(60)" json:"title"`        // 标题
	Description string `gorm:"type:vachar(128" json:"description"` // 描述
}
