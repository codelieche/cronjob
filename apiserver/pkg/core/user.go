package core

import (
	"context"
	"encoding/json"
	"time"

	"github.com/codelieche/cronjob/apiserver/pkg/utils/filters"
	"github.com/codelieche/cronjob/apiserver/pkg/utils/types"
)

type User struct {
	types.BaseModel
	Nickname    string          `gorm:"size:256" json:"nickname"`
	Username    string          `gorm:"size:80;unique" json:"name"`
	Phone       string          `gorm:"size:16;index" json:"phone"`
	Email       string          `gorm:"size:80" json:"email"`
	Description string          `gorm:"text" json:"description"`
	Comment     string          `gorm:"text" json:"comment"`
	WechatID    string          `gorm:"size:128" json:"wechat_id"`
	Metadata    json.RawMessage `gorm:"type:json" json:"metadata"`
	IsActive    *bool           `gorm:"type:boolean;default:false" json:"is_active" form:"is_active"`
	LastLogin   time.Time       `gorm:"column:last_login" json:"last_login,omitempty"`
}

type UserStore interface {
	// Find 根据用户名获取用户
	Find(ctx context.Context, username string) (*User, error)

	// FindByID 根据ID获取用户
	FindByID(ctx context.Context, id int64) (*User, error)

	// Create 创建用户
	Create(ctx context.Context, obj *User) (*User, error)

	// Update 更新用户信息
	Update(ctx context.Context, obj *User) (*User, error)

	// Delete 删除用户
	Delete(ctx context.Context, obj *User) error

	// DeleteByID 根据ID删除用户
	DeleteByID(ctx context.Context, id int64) error

	// List 获取用户列表
	List(ctx context.Context, offset int, limit int, filterActions ...filters.Filter) (users []*User, err error)

	// Count 统计用户数量
	Count(ctx context.Context, filterActions ...filters.Filter) (int64, error)

	// GetOrCreate 获取或者创建用户
	GetOrCreate(ctx context.Context, obj *User) (*User, error)
}

type UserService interface {
	// Find 根据用户名获取用户
	Find(ctx context.Context, username string) (*User, error)

	// FindByID 根据ID获取用户
	FindByID(ctx context.Context, id string) (*User, error)

	// Create 创建用户
	Create(ctx context.Context, obj *User) (*User, error)

	// Update 更新用户信息
	Update(ctx context.Context, obj *User) (*User, error)

	// Delete 删除用户
	Delete(ctx context.Context, obj *User) error

	// DeleteByID 根据ID删除用户
	DeleteByID(ctx context.Context, id string) error

	// List 获取用户列表
	List(ctx context.Context, offset int, limit int, filterActions ...filters.Filter) (users []*User, err error)

	// Count 统计用户数量
	Count(ctx context.Context, filterActions ...filters.Filter) (int64, error)

	// GetOrCreate 获取或者创建用户
	GetOrCreate(ctx context.Context, obj *User) (*User, error)
}
