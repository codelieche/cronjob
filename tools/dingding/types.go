package dingding

import (
	"encoding/json"

	"github.com/jinzhu/gorm"
)

// 部门
type Department struct {
	gorm.Model
	Name        string          `gorm:"type:varchar(80)",json:"name"` // 部门名称
	DingID      int             `gorm:"unique_index",json:"ding_id"`  // 部门ID【对应DingDing中的ID】
	DingData    json.RawMessage `json:"ding_data,omitempty"`          // 对应的钉钉数据
	Description string          `gorm:"type:text",json:"description"` // 描述
	Users       []*User         `gorm:"many2many:user_departments"`   // 部门用户
}

type User struct {
	gorm.Model
	Username    string          `gorm:"type:varchar(40);INDEX",json:"username"` // 用户名
	DingID      string          `gorm:"size:100;UNIQUE_INDEX",json:"ding_id"`   // 对应DingDing中的ID
	Mobile      string          `gorm:"type:varchar(40)",json:"mobile"`         // 手机号
	Position    string          `gorm:"type:varchar(40)",json:"position"`       // 职位
	Departments []*Department   `gorm:"many2many:user_departments;"`            // 用户所在的部门
	DingData    json.RawMessage `json:"ding_data,omitempty"`                    // 对应的钉钉数据
}
