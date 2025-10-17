package forms

import "github.com/google/uuid"

// AIProviderForm AI平台配置表单
type AIProviderForm struct {
	Name               string    `json:"name" binding:"required"`          // 平台名称
	Description        string    `json:"description"`                      // 描述
	ProviderType       string    `json:"provider_type" binding:"required"` // 平台类型：openai/anthropic/azure/custom
	APIEndpoint        string    `json:"api_endpoint"`                     // API端点URL
	APIKey             string    `json:"api_key" binding:"required"`       // API密钥（明文传入，后端加密）
	DefaultModel       string    `json:"default_model"`                    // 默认模型
	DefaultTemperature *float64  `json:"default_temperature"`              // 默认温度
	DefaultMaxTokens   *int      `json:"default_max_tokens"`               // 默认最大Token数
	DefaultTimeout     *int      `json:"default_timeout"`                  // 默认超时时间（秒）
	RateLimitRPM       *int      `json:"rate_limit_rpm"`                   // 每分钟请求限制
	RateLimitTPM       *int      `json:"rate_limit_tpm"`                   // 每分钟Token限制
	DailyBudget        *float64  `json:"daily_budget"`                     // 每日预算
	Config             string    `json:"config"`                           // 额外配置（JSON字符串）
	Enabled            *bool     `json:"enabled"`                          // 是否启用
	TeamID             uuid.UUID `json:"team_id"`                          // 团队ID（可选，不传则使用当前用户的team_id）
}

// AIProviderUpdateForm AI平台配置更新表单
type AIProviderUpdateForm struct {
	Name               string   `json:"name"`
	Description        string   `json:"description"`
	APIEndpoint        string   `json:"api_endpoint"`
	APIKey             string   `json:"api_key"` // 为空表示不更新
	DefaultModel       string   `json:"default_model"`
	DefaultTemperature *float64 `json:"default_temperature"`
	DefaultMaxTokens   *int     `json:"default_max_tokens"`
	DefaultTimeout     *int     `json:"default_timeout"`
	RateLimitRPM       *int     `json:"rate_limit_rpm"`
	RateLimitTPM       *int     `json:"rate_limit_tpm"`
	DailyBudget        *float64 `json:"daily_budget"`
	Config             string   `json:"config"`
	Enabled            *bool    `json:"enabled"`
}
