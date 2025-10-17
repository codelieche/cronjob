package forms

import "github.com/google/uuid"

// AIAgentForm AI Agent表单
type AIAgentForm struct {
	ProviderID            uuid.UUID `json:"provider_id" binding:"required"` // 关联的Provider ID
	Name                  string    `json:"name" binding:"required"`        // Agent名称
	Description           string    `json:"description"`                    // 描述
	AgentType             string    `json:"agent_type" binding:"required"`  // Agent类型：approval/code_review/security_scan
	Model                 *string   `json:"model"`                          // 模型（覆盖Provider默认值）
	Temperature           *float64  `json:"temperature"`                    // 温度
	MaxTokens             *int      `json:"max_tokens"`                     // 最大Token数
	Timeout               *int      `json:"timeout"`                        // 超时时间（秒）
	SystemPrompt          string    `json:"system_prompt"`                  // 系统提示词
	PromptTemplate        string    `json:"prompt_template"`                // 提示词模板
	MaxContextLength      *int      `json:"max_context_length"`             // 最大上下文长度
	IncludeHistory        *bool     `json:"include_history"`                // 是否包含历史记录
	DecisionThreshold     *float64  `json:"decision_threshold"`             // 决策阈值
	AutoApproveConditions string    `json:"auto_approve_conditions"`        // 自动通过条件（JSON字符串）
	AutoRejectConditions  string    `json:"auto_reject_conditions"`         // 自动拒绝条件（JSON字符串）
	Config                string    `json:"config"`                         // 额外配置（JSON字符串）
	Enabled               *bool     `json:"enabled"`                        // 是否启用
	TeamID                uuid.UUID `json:"team_id"`                        // 团队ID（可选，不传则使用当前用户的team_id）
}

// AIAgentUpdateForm AI Agent更新表单
type AIAgentUpdateForm struct {
	Name                  string   `json:"name"`
	Description           string   `json:"description"`
	Model                 *string  `json:"model"`
	Temperature           *float64 `json:"temperature"`
	MaxTokens             *int     `json:"max_tokens"`
	Timeout               *int     `json:"timeout"`
	SystemPrompt          string   `json:"system_prompt"`
	PromptTemplate        string   `json:"prompt_template"`
	MaxContextLength      *int     `json:"max_context_length"`
	IncludeHistory        *bool    `json:"include_history"`
	DecisionThreshold     *float64 `json:"decision_threshold"`
	AutoApproveConditions string   `json:"auto_approve_conditions"`
	AutoRejectConditions  string   `json:"auto_reject_conditions"`
	Config                string   `json:"config"`
	Enabled               *bool    `json:"enabled"`
}
