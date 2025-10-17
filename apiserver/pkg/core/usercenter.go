package core

import "github.com/google/uuid"

// ==================== Usercenter 响应格式 ====================

// UsercenterResponse Usercenter API统一响应格式
type UsercenterResponse struct {
	Code    int         `json:"code"`    // 返回的code，如果是0就表示正常
	Message string      `json:"message"` // 返回的消息
	Data    interface{} `json:"data"`    // 返回的数据
}

// ==================== 消息相关结构 ====================

// MessageCreateRequest 创建消息请求
type MessageCreateRequest struct {
	ReceiverID  uuid.UUID  `json:"receiver_id"`            // 接收用户ID（必填）
	TeamID      *uuid.UUID `json:"team_id,omitempty"`      // 团队上下文ID（用于跳转时切换团队）
	Platform    string     `json:"platform"`               // 来源平台: apiserver/todolist/usercenter
	Category    string     `json:"category"`               // 消息类型: default/info/success/warning/error/system/safe
	Title       string     `json:"title"`                  // 消息标题（必填）
	Content     string     `json:"content"`                // 消息内容（详细描述）
	RelatedID   *uuid.UUID `json:"related_id,omitempty"`   // 关联对象ID（如审批ID）
	RelatedType string     `json:"related_type,omitempty"` // 关联对象类型（如approval）
	SenderID    *uuid.UUID `json:"sender_id,omitempty"`    // 发送者ID（为空表示系统消息）
}

// MessageBatchCreateRequest 批量创建消息请求
type MessageBatchCreateRequest struct {
	Messages []*MessageCreateRequest `json:"messages"` // 消息列表
}

// ==================== 用户相关结构 ====================

// UsercenterUser Usercenter用户信息（简化版）
type UsercenterUser struct {
	ID       uuid.UUID `json:"id"`
	Username string    `json:"username"`
	Name     string    `json:"name"`
	Email    string    `json:"email"`
	Mobile   string    `json:"mobile"`
	IsActive *bool     `json:"is_active"`
}

// ==================== Usercenter Service 接口 ====================

// UsercenterService Usercenter服务接口
//
// 提供与Usercenter系统交互的统一接口，包括：
//   - 消息发送（站内信）
//   - 用户信息查询
//   - 团队成员查询
type UsercenterService interface {
	// CreateMessage 创建单个消息（发送站内信）
	//
	// 参数:
	//   - req: 消息创建请求
	//
	// 返回值:
	//   - error: 创建失败时返回错误，成功返回nil
	CreateMessage(req *MessageCreateRequest) error

	// BatchCreateMessages 批量创建消息
	//
	// 参数:
	//   - reqs: 消息创建请求列表
	//
	// 返回值:
	//   - error: 创建失败时返回错误，成功返回nil
	//
	// 说明:
	//   - 批量发送时，单个消息失败不会影响其他消息
	//   - 只要有一个消息发送成功，就返回nil
	BatchCreateMessages(reqs []*MessageCreateRequest) error

	// GetUser 获取用户信息
	//
	// 参数:
	//   - userID: 用户ID
	//
	// 返回值:
	//   - *UsercenterUser: 用户信息
	//   - error: 查询失败时返回错误
	GetUser(userID uuid.UUID) (*UsercenterUser, error)

	// GetTeamMembers 获取团队成员列表
	//
	// 参数:
	//   - teamID: 团队ID
	//
	// 返回值:
	//   - []*UsercenterUser: 团队成员列表
	//   - error: 查询失败时返回错误
	GetTeamMembers(teamID uuid.UUID) ([]*UsercenterUser, error)
}
