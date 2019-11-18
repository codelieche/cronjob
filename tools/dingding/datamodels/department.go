package datamodels

import (
	"encoding/json"

	"github.com/jinzhu/gorm"
)

// 部门
type Department struct {
	gorm.Model
	Name        string          `gorm:"type:varchar(80);NOT NULL" json:"name"` // 部门名称
	DingID      int             `gorm:"unique_index" json:"ding_id"`           // 部门ID【对应DingDing中的ID】
	DingData    json.RawMessage `gorm:"type:text" json:"ding_data,omitempty"`  // 对应的钉钉数据
	Description string          `gorm:"type:text" json:"description"`          // 描述
	Users       []*User         `gorm:"many2many:user_departments"`            // 部门用户
}
