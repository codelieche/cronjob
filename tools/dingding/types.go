package dingding

import (
	"encoding/json"

	"github.com/jinzhu/gorm"
)

// 部门
type Department struct {
	gorm.Model
	Name     string          `json:"name"`                // 部门名称
	DingID   int             `json:"ding_id"`             // 部门ID【对应DingDing中的ID】
	DingData json.RawMessage `json:"ding_data,omitempty"` // 对应的钉钉数据
}

type User struct {
	gorm.Model
	Username string          `json:"username"`            // 用户名
	DingID   string          `json:"ding_id"`             // 对应DingDing中的ID
	Mobile   string          `json:"mobile"`              // 手机号
	Position string          `json:"position"`            // 职位
	DingData json.RawMessage `json:"ding_data,omitempty"` // 对应的钉钉数据
}
