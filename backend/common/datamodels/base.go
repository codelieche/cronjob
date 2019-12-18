package datamodels

import "time"

type BaseFields struct {
	ID        uint       `gorm:"primary_key;unsigned auto_increment;not null" json:"id"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
	DeletedAt *time.Time `sql:"index" json:"deleted_at"`
}
