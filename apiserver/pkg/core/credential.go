package core

import (
	"context"
	"time"

	"github.com/codelieche/cronjob/apiserver/pkg/utils/filters"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Credential 凭证模型
type Credential struct {
	ID          uuid.UUID      `gorm:"size:256;primaryKey" json:"id"`                                    // 凭证唯一标识（UUID）
	TeamID      *uuid.UUID     `gorm:"size:256;index:idx_team_category" json:"team_id"`                  // 团队ID，用于多租户隔离
	Category    string         `gorm:"size:32;not null;index:idx_team_category" json:"category"`         // 凭证类型：username_password, api_token等
	Name        string         `gorm:"size:128;not null" json:"name"`                                    // 凭证名称
	Description string         `gorm:"size:512" json:"description"`                                      // 凭证描述
	Project     string         `gorm:"size:128;index:idx_project" json:"project"`                        // 项目名称（可选）
	Value       string         `gorm:"type:text;not null" json:"value"`                                  // 凭证内容（JSON格式，敏感字段加密）
	Version     int            `gorm:"not null;default:1" json:"version"`                                // 版本号
	IsActive    *bool          `gorm:"type:boolean;default:true;index:idx_team_active" json:"is_active"` // 是否启用
	Metadata    string         `gorm:"type:json" json:"metadata,omitempty"`                              // 元数据（标签、环境等）
	CreatedBy   *uuid.UUID     `gorm:"size:256" json:"created_by,omitempty"`                             // 创建人ID
	UpdatedBy   *uuid.UUID     `gorm:"size:256" json:"updated_by,omitempty"`                             // 更新人ID
	CreatedAt   time.Time      `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt   time.Time      `gorm:"autoUpdateTime" json:"updated_at"`
	DeletedAt   gorm.DeletedAt `gorm:"index" json:"-"`
	Deleted     bool           `gorm:"not null;default:false;index" json:"deleted"`
}

// TableName 指定表名
func (Credential) TableName() string {
	return "credentials"
}

// BeforeCreate 创建前生成UUID（与项目其他模型保持一致）
func (c *Credential) BeforeCreate(tx *gorm.DB) error {
	if c.ID == uuid.Nil {
		c.ID = uuid.New()
	}
	return nil
}

// CredentialStore 凭证存储接口
type CredentialStore interface {
	// FindByID 根据ID查找凭证
	FindByID(ctx context.Context, id uuid.UUID) (*Credential, error)

	// Create 创建凭证
	Create(ctx context.Context, credential *Credential) (*Credential, error)

	// Update 更新凭证
	Update(ctx context.Context, credential *Credential) (*Credential, error)

	// DeleteByID 删除凭证（软删除）
	DeleteByID(ctx context.Context, id uuid.UUID) error

	// List 获取凭证列表（带过滤和分页）
	List(ctx context.Context, offset, limit int, filterActions ...filters.Filter) ([]*Credential, error)

	// Count 获取凭证总数（带过滤）
	Count(ctx context.Context, filterActions ...filters.Filter) (int64, error)

	// Patch 动态更新凭证字段
	Patch(ctx context.Context, id uuid.UUID, updates map[string]interface{}) error
}

// CredentialService 凭证服务接口
type CredentialService interface {
	// FindByID 根据ID查找凭证
	FindByID(ctx context.Context, id string) (*Credential, error)

	// Create 创建凭证
	Create(ctx context.Context, credential *Credential) (*Credential, error)

	// Update 更新凭证
	Update(ctx context.Context, credential *Credential) (*Credential, error)

	// DeleteByID 删除凭证（软删除）
	DeleteByID(ctx context.Context, id string) error

	// List 获取凭证列表（带过滤和分页）
	List(ctx context.Context, offset, limit int, filterActions ...filters.Filter) ([]*Credential, error)

	// Count 获取凭证总数（带过滤）
	Count(ctx context.Context, filterActions ...filters.Filter) (int64, error)

	// Patch 动态更新凭证字段
	Patch(ctx context.Context, id string, updates map[string]interface{}) error

	// Decrypt 解密凭证（返回解密后的值）
	Decrypt(ctx context.Context, id string) (map[string]interface{}, error)

	// DecryptWithMetadata 解密凭证并返回完整信息（包括元数据）
	DecryptWithMetadata(ctx context.Context, id string) (map[string]interface{}, error)
}
