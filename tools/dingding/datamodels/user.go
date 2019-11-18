package datamodels

import (
	"encoding/json"

	"github.com/jinzhu/gorm"
)

type User struct {
	gorm.Model
	Username    string          `gorm:"type:varchar(40);INDEX;NOT NULL" json:"username"`          // 用户名
	DingID      string          `gorm:"size:100;UNIQUE_INDEX" json:"ding_id"`                     // 对应DingDing中的ID
	Mobile      string          `gorm:"type:varchar(40)" json:"mobile"`                           // 手机号
	Position    string          `gorm:"type:varchar(40)" json:"position"`                         // 职位
	Departments []*Department   `gorm:"many2many:user_departments;" json:"departments,omitempty"` // 用户所在的部门
	DingData    json.RawMessage `gorm:"type:text" json:"ding_data,omitempty"`                     // 对应的钉钉数据
	Messages    []*Message      `gorm:"many2many:message_users" json:"messages,omitempty"`        // 用户的消息
}
