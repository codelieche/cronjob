package core

import (
	"context"
	"encoding/json"
	"time"

	"github.com/codelieche/cronjob/apiserver/pkg/utils/filters"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// AIAgent AI审批实体
type AIAgent struct {
	// 基础字段
	ID          uuid.UUID `gorm:"type:char(36);primaryKey" json:"id"`
	ProviderID  uuid.UUID `gorm:"type:char(36);not null;index" json:"provider_id"`
	Name        string    `gorm:"type:varchar(100);not null" json:"name"`
	Description string    `gorm:"type:text" json:"description"`

	// Agent类型
	AgentType string `gorm:"type:varchar(50);not null;index" json:"agent_type"` // approval/code_review/security_scan

	// AI配置（覆盖Provider默认值）
	Model       *string  `gorm:"type:varchar(100)" json:"model"`
	Temperature *float64 `gorm:"type:decimal(3,2)" json:"temperature"`
	MaxTokens   *int     `json:"max_tokens"`
	Timeout     *int     `json:"timeout"`

	// 提示词配置
	SystemPrompt   string `gorm:"type:text" json:"system_prompt"`
	PromptTemplate string `gorm:"type:text" json:"prompt_template"`

	// 上下文配置
	MaxContextLength *int  `json:"max_context_length"`
	IncludeHistory   *bool `json:"include_history"`

	// 决策配置
	DecisionThreshold     *float64 `gorm:"type:decimal(3,2)" json:"decision_threshold"`
	AutoApproveConditions string   `gorm:"type:json" json:"auto_approve_conditions"`
	AutoRejectConditions  string   `gorm:"type:json" json:"auto_reject_conditions"`

	// 扩展配置
	Config string `gorm:"type:json" json:"config"`

	// 统计信息（通用）
	TotalCalls      *int       `gorm:"default:0" json:"total_calls"`
	SuccessCount    *int       `gorm:"default:0" json:"success_count"`
	FailedCount     *int       `gorm:"default:0" json:"failed_count"`
	AvgResponseTime *int       `json:"avg_response_time"` // 毫秒
	LastCalledAt    *time.Time `json:"last_called_at"`

	// 状态
	Enabled *bool `gorm:"default:true;index" json:"enabled"`

	// 团队关联
	TeamID uuid.UUID `gorm:"type:char(36);not null;index" json:"team_id"`

	// 系统字段
	CreatedBy *uuid.UUID `gorm:"type:char(36)" json:"created_by"`
	CreatedAt time.Time  `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt time.Time  `gorm:"autoUpdateTime" json:"updated_at"`
}

// TableName 指定表名
func (AIAgent) TableName() string {
	return "ai_agents"
}

// BeforeCreate 创建前生成UUID
func (a *AIAgent) BeforeCreate(tx *gorm.DB) error {
	if a.ID == uuid.Nil {
		a.ID = uuid.New()
	}
	return nil
}

// AutoApproveCondition 自动通过条件
type AutoApproveCondition struct {
	ConfidenceThreshold float64  `json:"confidence_threshold,omitempty"`
	RequiredChecks      []string `json:"required_checks,omitempty"`
}

// AutoRejectCondition 自动拒绝条件
type AutoRejectCondition struct {
	Keywords      []string `json:"keywords,omitempty"`
	SecurityLevel string   `json:"security_level,omitempty"`
	Severity      []string `json:"severity,omitempty"`
}

// GetAutoApproveConditions 解析自动通过条件
func (a *AIAgent) GetAutoApproveConditions() (*AutoApproveCondition, error) {
	if a.AutoApproveConditions == "" {
		return &AutoApproveCondition{}, nil
	}

	var cond AutoApproveCondition
	if err := json.Unmarshal([]byte(a.AutoApproveConditions), &cond); err != nil {
		return nil, err
	}
	return &cond, nil
}

// SetAutoApproveConditions 设置自动通过条件
func (a *AIAgent) SetAutoApproveConditions(cond *AutoApproveCondition) error {
	data, err := json.Marshal(cond)
	if err != nil {
		return err
	}
	a.AutoApproveConditions = string(data)
	return nil
}

// GetAutoRejectConditions 解析自动拒绝条件
func (a *AIAgent) GetAutoRejectConditions() (*AutoRejectCondition, error) {
	if a.AutoRejectConditions == "" {
		return &AutoRejectCondition{}, nil
	}

	var cond AutoRejectCondition
	if err := json.Unmarshal([]byte(a.AutoRejectConditions), &cond); err != nil {
		return nil, err
	}
	return &cond, nil
}

// SetAutoRejectConditions 设置自动拒绝条件
func (a *AIAgent) SetAutoRejectConditions(cond *AutoRejectCondition) error {
	data, err := json.Marshal(cond)
	if err != nil {
		return err
	}
	a.AutoRejectConditions = string(data)
	return nil
}

// AIAgentStore AI Agent存储接口
type AIAgentStore interface {
	// Create 创建AI Agent
	Create(ctx context.Context, agent *AIAgent) (*AIAgent, error)

	// Update 更新AI Agent
	Update(ctx context.Context, agent *AIAgent) (*AIAgent, error)

	// FindByID 根据ID查找
	FindByID(ctx context.Context, id uuid.UUID) (*AIAgent, error)

	// DeleteByID 删除（软删除）
	DeleteByID(ctx context.Context, id uuid.UUID) error

	// List 获取列表（带过滤和分页）
	List(ctx context.Context, offset, limit int, filterActions ...filters.Filter) ([]*AIAgent, error)

	// Count 统计数量
	Count(ctx context.Context, filterActions ...filters.Filter) (int64, error)

	// FindByProviderID 根据Provider ID查找Agent列表
	FindByProviderID(ctx context.Context, providerID uuid.UUID) ([]*AIAgent, error)
}
