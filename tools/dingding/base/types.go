package base

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

// 发送的消息
type Message struct {
	gorm.Model
	//UserID       uint            `gorm:"index" json:"user_id"`                             // 用户ID
	Success      bool            `gorm:"index;NOT NULL" json:"success"`                    // 消息是否成功
	Users        []*User         `gorm:"many2many:message_users" json:"users"`             // 消息接收的用户
	Title        string          `gorm:"type:varchar(128)" json:"title,omitempty"`         // 消息标题
	MsgType      string          `gorm:"type:varchar(40);NOT NULL" json:"msg_type"`        // 消息类型：text、markdown等
	Content      string          `gorm:"type:text;NOT NULL" json:"content"`                // 消息内容
	DingData     json.RawMessage `gorm:"type:text" json:"ding_data,omitempty"`             // Ding Message
	DingResponse json.RawMessage `gorm:"type:varchar(512)" json:"ding_response",omitempty` // 发送消息的响应结果
}

func (msg *Message) Save() {
	//	保存消息到数据库中
	if msg.ID == 0 {
		// 新创建
		db.Model(&Message{}).Create(msg)
	} else {
		db.Model(&Message{}).Update(msg)
	}
}
