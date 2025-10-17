package core

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/codelieche/cronjob/apiserver/pkg/utils/filters"
	"github.com/codelieche/cronjob/apiserver/pkg/utils/tools"
	"github.com/codelieche/cronjob/apiserver/pkg/utils/types"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// AIProvider AI平台配置
type AIProvider struct {
	// 基础字段
	ID          uuid.UUID `gorm:"type:char(36);primaryKey" json:"id"`
	Name        string    `gorm:"type:varchar(100);not null" json:"name"`
	Description string    `gorm:"type:text" json:"description"`

	// 平台配置
	ProviderType string `gorm:"type:varchar(50);not null;index" json:"provider_type"` // openai/anthropic/azure/custom
	APIEndpoint  string `gorm:"type:varchar(500)" json:"api_endpoint"`
	APIKey       string `gorm:"type:text" json:"api_key,omitempty"` // 对称加密存储，敏感信息前端不返回

	// 默认配置
	DefaultModel       string   `gorm:"type:varchar(100)" json:"default_model"`
	DefaultTemperature *float64 `gorm:"type:decimal(3,2)" json:"default_temperature"`
	DefaultMaxTokens   *int     `json:"default_max_tokens"`
	DefaultTimeout     *int     `json:"default_timeout"`

	// 限额配置
	RateLimitRPM *int     `json:"rate_limit_rpm"`
	RateLimitTPM *int     `json:"rate_limit_tpm"`
	DailyBudget  *float64 `gorm:"type:decimal(10,2)" json:"daily_budget"`

	// 额外配置
	Config string `gorm:"type:json" json:"config"`

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
func (AIProvider) TableName() string {
	return "ai_providers"
}

// BeforeCreate 创建前生成UUID
func (p *AIProvider) BeforeCreate(tx *gorm.DB) error {
	if p.ID == uuid.Nil {
		p.ID = uuid.New()
	}
	return nil
}

// AIProviderConfig 平台特定配置
type AIProviderConfig struct {
	OrganizationID string `json:"organization_id,omitempty"`
	APIVersion     string `json:"api_version,omitempty"`
	RetryTimes     int    `json:"retry_times,omitempty"`
	ProxyURL       string `json:"proxy_url,omitempty"`
}

// GetConfig 解析配置
func (p *AIProvider) GetConfig() (*AIProviderConfig, error) {
	if p.Config == "" {
		return &AIProviderConfig{}, nil
	}

	var config AIProviderConfig
	if err := json.Unmarshal([]byte(p.Config), &config); err != nil {
		return nil, err
	}
	return &config, nil
}

// SetConfig 设置配置
func (p *AIProvider) SetConfig(config *AIProviderConfig) error {
	data, err := json.Marshal(config)
	if err != nil {
		return err
	}
	p.Config = string(data)
	return nil
}

// GetDecryptedAPIKey 获取解密后的API Key
// 参考凭证系统的实现，使用对称加密/解密
func (p *AIProvider) GetDecryptedAPIKey() (string, error) {
	if p.APIKey == "" {
		return "", nil
	}

	// 使用系统加密服务解密
	crypto := tools.NewCryptography(types.EncryptionKey)
	decrypted, err := crypto.Decrypt(p.APIKey)
	if err != nil {
		return "", fmt.Errorf("解密API Key失败: %w", err)
	}

	return decrypted, nil
}

// EncryptAPIKey 加密API Key（静态方法）
// 在创建/更新时使用
func EncryptAPIKey(plainKey string) (string, error) {
	if plainKey == "" {
		return "", nil
	}

	crypto := tools.NewCryptography(types.EncryptionKey)
	encrypted, err := crypto.Encrypt(plainKey)
	if err != nil {
		return "", fmt.Errorf("加密API Key失败: %w", err)
	}

	return encrypted, nil
}

// AIProviderStore AI平台配置存储接口
type AIProviderStore interface {
	// Create 创建AI平台配置
	Create(ctx context.Context, provider *AIProvider) (*AIProvider, error)

	// Update 更新AI平台配置
	Update(ctx context.Context, provider *AIProvider) (*AIProvider, error)

	// FindByID 根据ID查找
	FindByID(ctx context.Context, id uuid.UUID) (*AIProvider, error)

	// DeleteByID 删除（软删除）
	DeleteByID(ctx context.Context, id uuid.UUID) error

	// List 获取列表（带过滤和分页）
	List(ctx context.Context, offset, limit int, filterActions ...filters.Filter) ([]*AIProvider, error)

	// Count 统计数量
	Count(ctx context.Context, filterActions ...filters.Filter) (int64, error)
}
