package core

import (
	"context"
	"strings"

	"github.com/codelieche/cronjob/apiserver/pkg/utils/filters"
	"github.com/codelieche/cronjob/apiserver/pkg/utils/types"
	"gorm.io/gorm"
)

// Category 分类
type Category struct {
	types.BaseModel
	Code                string `gorm:"size:128;unique;not null" json:"code"` // 分类编码，唯一且不为空
	Name                string `gorm:"size:128" json:"name"`                 // 分类名称
	Setup               string `gorm:"size:512" json:"setup"`                // 初始化脚本
	Teardown            string `gorm:"size:512" json:"teardown"`             // 销毁脚本
	Check               string `gorm:"size:512" json:"check"`                // 检查脚本
	Description         string `gorm:"text" json:"description"`              // 分类描述
	DeleteTimeFieldName string `gorm:"-" json:"-"`                           // 删除时自动记录时间的字段名
}

// TableName 分类表名
func (Category) TableName() string {
	return "categories"
}

// BeforeCreate 分类创建前
func (c *Category) BeforeCreate() error {
	return nil
}

// BeforeUpdate 分类更新前
func (c *Category) BeforeUpdate() error {
	return nil
}

func (c *Category) GetDeleteTasks() []string {
	return []string{"DeleteChangeCode"}
}

func (c *Category) BeforeDelete(tx *gorm.DB) (err error) {
	if c.ID == 0 {
		return
	}
	// 设置Deleted字段为true
	trueValue := true
	c.Deleted = &trueValue

	// 1. 修改Code字段，添加_del_前缀和时间戳
	// 确保Code不为空再进行修改
	if c.Code != "" && !strings.Contains(c.Code, "_del_") {
		newCode := c.Code + "_del_" + c.Strftime("20060102150405")
		c.Code = newCode
	}
	// 修改
	tx.Model(c).Update("code", c.Code)
	tx.Model(c).Update("deleted", c.Deleted)

	return nil
}

// CategoryStore 分类存储接口
type CategoryStore interface {
	// FindByID 根据ID获取分类
	FindByID(ctx context.Context, id uint) (*Category, error)

	// FindByCode 根据编码获取分类
	FindByCode(ctx context.Context, code string) (*Category, error)

	// FindByIDOrCode 根据ID或Code获取分类
	FindByIDOrCode(ctx context.Context, idOrCode string) (*Category, error)

	// Create 创建分类
	Create(ctx context.Context, obj *Category) (*Category, error)

	// Update 更新分类信息
	Update(ctx context.Context, obj *Category) (*Category, error)

	// Delete 删除分类
	Delete(ctx context.Context, obj *Category) error

	// DeleteByID 根据ID删除分类
	DeleteByID(ctx context.Context, id uint) error

	// List 获取分类列表
	List(ctx context.Context, offset int, limit int, filterActions ...filters.Filter) (categories []*Category, err error)

	// Count 统计分类数量
	Count(ctx context.Context, filterActions ...filters.Filter) (int64, error)
}

// CategoryService 分类服务接口
type CategoryService interface {
	// FindByID 根据ID获取分类
	FindByID(ctx context.Context, id uint) (*Category, error)

	// FindByCode 根据编码获取分类
	FindByCode(ctx context.Context, code string) (*Category, error)

	// FindByIDOrCode 根据ID或Code获取分类
	FindByIDOrCode(ctx context.Context, idOrCode string) (*Category, error)

	// Create 创建分类
	Create(ctx context.Context, obj *Category) (*Category, error)

	// Update 更新分类信息
	Update(ctx context.Context, obj *Category) (*Category, error)

	// Delete 删除分类
	Delete(ctx context.Context, obj *Category) error

	// DeleteByID 根据ID删除分类
	DeleteByID(ctx context.Context, id uint) error

	// List 获取分类列表
	List(ctx context.Context, offset int, limit int, filterActions ...filters.Filter) (categories []*Category, err error)

	// Count 统计分类数量
	Count(ctx context.Context, filterActions ...filters.Filter) (int64, error)
}
