package datamodels

import (
	"encoding/json"

	"github.com/jinzhu/gorm"
)

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
