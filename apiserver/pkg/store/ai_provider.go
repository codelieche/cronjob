package store

import (
	"context"
	"errors"

	"github.com/codelieche/cronjob/apiserver/pkg/core"
	"github.com/codelieche/cronjob/apiserver/pkg/utils/filters"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// aiProviderStore AI平台配置存储实现
type aiProviderStore struct {
	db *gorm.DB
}

// NewAIProviderStore 创建AIProviderStore实例
func NewAIProviderStore(db *gorm.DB) core.AIProviderStore {
	return &aiProviderStore{
		db: db,
	}
}

// Create 创建AI平台配置
func (s *aiProviderStore) Create(ctx context.Context, provider *core.AIProvider) (*core.AIProvider, error) {
	tx := s.db.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	if err := tx.Create(provider).Error; err != nil {
		tx.Rollback()
		return nil, err
	}

	tx.Commit()
	return provider, nil
}

// Update 更新AI平台配置
func (s *aiProviderStore) Update(ctx context.Context, provider *core.AIProvider) (*core.AIProvider, error) {
	// 检查是否存在
	if _, err := s.FindByID(ctx, provider.ID); err != nil {
		return nil, err
	}

	// 更新
	if err := s.db.Save(provider).Error; err != nil {
		return nil, err
	}

	return provider, nil
}

// FindByID 根据ID查找
func (s *aiProviderStore) FindByID(ctx context.Context, id uuid.UUID) (*core.AIProvider, error) {
	var provider core.AIProvider
	if err := s.db.First(&provider, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, core.ErrNotFound
		}
		return nil, err
	}
	return &provider, nil
}

// DeleteByID 删除（软删除）
func (s *aiProviderStore) DeleteByID(ctx context.Context, id uuid.UUID) error {
	// 检查是否存在
	if _, err := s.FindByID(ctx, id); err != nil {
		return err
	}

	// 软删除（实际是硬删除，因为没有DeletedAt字段）
	if err := s.db.Delete(&core.AIProvider{}, "id = ?", id).Error; err != nil {
		return err
	}

	return nil
}

// List 获取列表（带过滤和分页）
func (s *aiProviderStore) List(ctx context.Context, offset, limit int, filterActions ...filters.Filter) ([]*core.AIProvider, error) {
	var providers []*core.AIProvider
	query := s.db.Model(&core.AIProvider{})

	// 应用过滤器
	for _, filterAction := range filterActions {
		query = filterAction.Filter(query)
	}

	// 排序：最新创建的在前
	query = query.Order("created_at DESC")

	// 分页查询
	if err := query.Offset(offset).Limit(limit).Find(&providers).Error; err != nil {
		return nil, err
	}

	return providers, nil
}

// Count 统计数量
func (s *aiProviderStore) Count(ctx context.Context, filterActions ...filters.Filter) (int64, error) {
	var count int64
	query := s.db.Model(&core.AIProvider{})

	// 应用过滤器
	for _, filterAction := range filterActions {
		query = filterAction.Filter(query)
	}

	if err := query.Count(&count).Error; err != nil {
		return 0, err
	}

	return count, nil
}
