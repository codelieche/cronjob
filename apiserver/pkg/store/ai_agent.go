package store

import (
	"context"
	"errors"

	"github.com/codelieche/cronjob/apiserver/pkg/core"
	"github.com/codelieche/cronjob/apiserver/pkg/utils/filters"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// aiAgentStore AI Agent存储实现
type aiAgentStore struct {
	db *gorm.DB
}

// NewAIAgentStore 创建AIAgentStore实例
func NewAIAgentStore(db *gorm.DB) core.AIAgentStore {
	return &aiAgentStore{
		db: db,
	}
}

// Create 创建AI Agent
func (s *aiAgentStore) Create(ctx context.Context, agent *core.AIAgent) (*core.AIAgent, error) {
	tx := s.db.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	if err := tx.Create(agent).Error; err != nil {
		tx.Rollback()
		return nil, err
	}

	tx.Commit()
	return agent, nil
}

// Update 更新AI Agent
func (s *aiAgentStore) Update(ctx context.Context, agent *core.AIAgent) (*core.AIAgent, error) {
	// 检查是否存在
	if _, err := s.FindByID(ctx, agent.ID); err != nil {
		return nil, err
	}

	// 更新
	if err := s.db.Save(agent).Error; err != nil {
		return nil, err
	}

	return agent, nil
}

// FindByID 根据ID查找
func (s *aiAgentStore) FindByID(ctx context.Context, id uuid.UUID) (*core.AIAgent, error) {
	var agent core.AIAgent
	if err := s.db.First(&agent, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, core.ErrNotFound
		}
		return nil, err
	}
	return &agent, nil
}

// DeleteByID 删除（软删除）
func (s *aiAgentStore) DeleteByID(ctx context.Context, id uuid.UUID) error {
	// 检查是否存在
	if _, err := s.FindByID(ctx, id); err != nil {
		return err
	}

	// 硬删除
	if err := s.db.Delete(&core.AIAgent{}, "id = ?", id).Error; err != nil {
		return err
	}

	return nil
}

// List 获取列表（带过滤和分页）
func (s *aiAgentStore) List(ctx context.Context, offset, limit int, filterActions ...filters.Filter) ([]*core.AIAgent, error) {
	var agents []*core.AIAgent
	query := s.db.Model(&core.AIAgent{})

	// 应用过滤器
	for _, filterAction := range filterActions {
		query = filterAction.Filter(query)
	}

	// 排序：最新创建的在前
	query = query.Order("created_at DESC")

	// 分页查询
	if err := query.Offset(offset).Limit(limit).Find(&agents).Error; err != nil {
		return nil, err
	}

	return agents, nil
}

// Count 统计数量
func (s *aiAgentStore) Count(ctx context.Context, filterActions ...filters.Filter) (int64, error) {
	var count int64
	query := s.db.Model(&core.AIAgent{})

	// 应用过滤器
	for _, filterAction := range filterActions {
		query = filterAction.Filter(query)
	}

	if err := query.Count(&count).Error; err != nil {
		return 0, err
	}

	return count, nil
}

// FindByProviderID 根据Provider ID查找Agent列表
func (s *aiAgentStore) FindByProviderID(ctx context.Context, providerID uuid.UUID) ([]*core.AIAgent, error) {
	var agents []*core.AIAgent
	if err := s.db.Where("provider_id = ?", providerID).Order("created_at DESC").Find(&agents).Error; err != nil {
		return nil, err
	}
	return agents, nil
}
