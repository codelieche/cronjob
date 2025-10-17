package core

import (
	"context"
	"encoding/json"
	"time"

	"github.com/codelieche/cronjob/apiserver/pkg/utils/filters"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Approval 审批记录
type Approval struct {
	// 基础字段
	ID uuid.UUID `gorm:"type:char(36);primaryKey" json:"id"`

	// 关联对象（可选）
	TaskID         *uuid.UUID `gorm:"type:char(36);index" json:"task_id,omitempty"`
	WorkflowExecID *uuid.UUID `gorm:"type:char(36);index" json:"workflow_exec_id,omitempty"`

	// 审批内容
	Title   string          `gorm:"type:varchar(200);not null" json:"title"`
	Content string          `gorm:"type:text" json:"content"`
	Context json.RawMessage `gorm:"type:json" json:"context"` // 使用json.RawMessage保证类型安全

	// 审批人配置
	UserIDs    json.RawMessage `gorm:"type:json" json:"user_ids"`     // 使用json.RawMessage保证类型安全
	AIAgentIDs json.RawMessage `gorm:"type:json" json:"ai_agent_ids"` // 使用json.RawMessage保证类型安全
	RequireAll *bool           `gorm:"default:false" json:"require_all"`

	// 审批状态
	Status          string     `gorm:"type:varchar(20);default:'pending';index" json:"status"` // pending/approved/rejected/timeout/cancelled
	ApprovedBy      string     `gorm:"type:varchar(100)" json:"approved_by"`
	ApprovedAt      *time.Time `json:"approved_at"`
	ApprovalComment string     `gorm:"type:text" json:"approval_comment"`

	// AI审批结果
	AIDecision json.RawMessage `gorm:"type:json" json:"ai_decision,omitempty"` // 使用json.RawMessage保证类型安全

	// 超时配置
	Timeout   int        `gorm:"default:3600" json:"timeout"`
	StartedAt *time.Time `json:"started_at"`
	TimeoutAt *time.Time `gorm:"index" json:"timeout_at"`

	// 扩展字段
	Metadata json.RawMessage `gorm:"type:json" json:"metadata"` // 使用json.RawMessage保证类型安全

	// 团队关联
	TeamID uuid.UUID `gorm:"type:char(36);not null;index" json:"team_id"`

	// 系统字段
	CreatedBy *uuid.UUID `gorm:"type:char(36)" json:"created_by"`
	CreatedAt time.Time  `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt time.Time  `gorm:"autoUpdateTime" json:"updated_at"`
}

// TableName 指定表名
func (Approval) TableName() string {
	return "approvals"
}

// BeforeCreate 创建前生成UUID
func (a *Approval) BeforeCreate(tx *gorm.DB) error {
	if a.ID == uuid.Nil {
		a.ID = uuid.New()
	}
	return nil
}

// ApprovalContext 审批上下文
type ApprovalContext struct {
	// Task审批场景
	TaskID       string  `json:"task_id,omitempty"`
	TaskName     string  `json:"task_name,omitempty"`
	Version      string  `json:"version,omitempty"`
	TestStatus   string  `json:"test_status,omitempty"`
	TestCoverage float64 `json:"test_coverage,omitempty"`

	// 资源删除场景
	ResourceType string `json:"resource_type,omitempty"`
	ResourceID   string `json:"resource_id,omitempty"`
	ResourceName string `json:"resource_name,omitempty"`
	Action       string `json:"action,omitempty"`
	Reason       string `json:"reason,omitempty"`

	// 扩展字段
	Extra map[string]interface{} `json:"extra,omitempty"`
}

// GetContext 解析审批上下文
func (a *Approval) GetContext() (*ApprovalContext, error) {
	if len(a.Context) == 0 {
		return &ApprovalContext{}, nil
	}

	var context ApprovalContext
	if err := json.Unmarshal(a.Context, &context); err != nil {
		return nil, err
	}
	return &context, nil
}

// SetContext 设置审批上下文
func (a *Approval) SetContext(context *ApprovalContext) error {
	data, err := json.Marshal(context)
	if err != nil {
		return err
	}
	a.Context = data
	return nil
}

// AIDecisionResult AI审批决策结果
type AIDecisionResult struct {
	AgentID    string                 `json:"agent_id"`
	Decision   string                 `json:"decision"` // approve/reject
	Confidence float64                `json:"confidence"`
	Reason     string                 `json:"reason"`
	Analysis   map[string]interface{} `json:"analysis,omitempty"`
}

// GetAIDecision 解析AI审批结果
func (a *Approval) GetAIDecision() (*AIDecisionResult, error) {
	if len(a.AIDecision) == 0 {
		return nil, nil
	}

	var decision AIDecisionResult
	if err := json.Unmarshal(a.AIDecision, &decision); err != nil {
		return nil, err
	}
	return &decision, nil
}

// SetAIDecision 设置AI审批结果
func (a *Approval) SetAIDecision(decision *AIDecisionResult) error {
	data, err := json.Marshal(decision)
	if err != nil {
		return err
	}
	a.AIDecision = data
	return nil
}

// GetUserIDs 解析审批人ID列表
func (a *Approval) GetUserIDs() ([]string, error) {
	if len(a.UserIDs) == 0 {
		return []string{}, nil
	}

	var ids []string
	if err := json.Unmarshal(a.UserIDs, &ids); err != nil {
		return nil, err
	}
	return ids, nil
}

// SetUserIDs 设置审批人ID列表
func (a *Approval) SetUserIDs(ids []string) error {
	data, err := json.Marshal(ids)
	if err != nil {
		return err
	}
	a.UserIDs = data
	return nil
}

// GetAIAgentIDs 解析AI Agent ID列表
func (a *Approval) GetAIAgentIDs() ([]string, error) {
	if len(a.AIAgentIDs) == 0 {
		return []string{}, nil
	}

	var ids []string
	if err := json.Unmarshal(a.AIAgentIDs, &ids); err != nil {
		return nil, err
	}
	return ids, nil
}

// SetAIAgentIDs 设置AI Agent ID列表
func (a *Approval) SetAIAgentIDs(ids []string) error {
	data, err := json.Marshal(ids)
	if err != nil {
		return err
	}
	a.AIAgentIDs = data
	return nil
}

// ApprovalStore 审批存储接口
type ApprovalStore interface {
	// Create 创建审批
	Create(ctx context.Context, approval *Approval) (*Approval, error)

	// Update 更新审批
	Update(ctx context.Context, approval *Approval) (*Approval, error)

	// FindByID 根据ID查找
	FindByID(ctx context.Context, id uuid.UUID) (*Approval, error)

	// DeleteByID 删除（软删除）
	DeleteByID(ctx context.Context, id uuid.UUID) error

	// List 获取列表（带过滤和分页）
	List(ctx context.Context, offset, limit int, filterActions ...filters.Filter) ([]*Approval, error)

	// Count 统计数量
	Count(ctx context.Context, filterActions ...filters.Filter) (int64, error)

	// FindByTaskID 根据Task ID查找
	FindByTaskID(ctx context.Context, taskID uuid.UUID) (*Approval, error)

	// FindTimeoutApprovals 查找超时的审批
	FindTimeoutApprovals(ctx context.Context, now time.Time) ([]*Approval, error)

	// FindMyPending 查找我的待审批
	FindMyPending(ctx context.Context, userID uuid.UUID, offset, limit int) ([]*Approval, error)

	// FindMyCreated 查找我发起的审批
	FindMyCreated(ctx context.Context, userID uuid.UUID, offset, limit int) ([]*Approval, error)
}
