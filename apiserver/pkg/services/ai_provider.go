package services

import (
	"context"

	"github.com/codelieche/cronjob/apiserver/pkg/core"
	"github.com/codelieche/cronjob/apiserver/pkg/utils/filters"
	"github.com/codelieche/cronjob/apiserver/pkg/utils/logger"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// AIProviderService AI平台配置服务实现
type AIProviderService struct {
	store core.AIProviderStore
}

// NewAIProviderService 创建AIProviderService实例
func NewAIProviderService(store core.AIProviderStore) *AIProviderService {
	return &AIProviderService{
		store: store,
	}
}

// Create 创建AI平台配置
// ⭐ 自动填充team_id：如果为空，使用当前用户的team_id
func (s *AIProviderService) Create(ctx context.Context, provider *core.AIProvider, currentUserTeamID uuid.UUID) (*core.AIProvider, error) {
	// 自动填充team_id
	if provider.TeamID == uuid.Nil {
		provider.TeamID = currentUserTeamID
		logger.Info("auto fill team_id for ai_provider",
			zap.String("provider_name", provider.Name),
			zap.String("team_id", currentUserTeamID.String()))
	}

	// 加密API Key
	if provider.APIKey != "" {
		encrypted, err := core.EncryptAPIKey(provider.APIKey)
		if err != nil {
			logger.Error("encrypt api key error", zap.Error(err))
			return nil, err
		}
		provider.APIKey = encrypted
	}

	// 创建
	created, err := s.store.Create(ctx, provider)
	if err != nil {
		logger.Error("create ai_provider error", zap.Error(err))
		return nil, err
	}

	// 返回时清空敏感字段
	created.APIKey = ""

	return created, nil
}

// Update 更新AI平台配置
func (s *AIProviderService) Update(ctx context.Context, provider *core.AIProvider) (*core.AIProvider, error) {
	// 如果APIKey不为空，说明是新传入的，需要加密
	if provider.APIKey != "" {
		encrypted, err := core.EncryptAPIKey(provider.APIKey)
		if err != nil {
			logger.Error("encrypt api key error", zap.Error(err))
			return nil, err
		}
		provider.APIKey = encrypted
	} else {
		// 如果为空，保持原有的（从数据库读取）
		existing, err := s.store.FindByID(ctx, provider.ID)
		if err != nil {
			return nil, err
		}
		provider.APIKey = existing.APIKey
	}

	// 更新
	updated, err := s.store.Update(ctx, provider)
	if err != nil {
		logger.Error("update ai_provider error", zap.Error(err))
		return nil, err
	}

	// 返回时清空敏感字段
	updated.APIKey = ""

	return updated, nil
}

// FindByID 根据ID查找
func (s *AIProviderService) FindByID(ctx context.Context, id string) (*core.AIProvider, error) {
	providerID, err := uuid.Parse(id)
	if err != nil {
		logger.Error("parse ai_provider id error", zap.Error(err), zap.String("id", id))
		return nil, core.ErrBadRequest
	}

	provider, err := s.store.FindByID(ctx, providerID)
	if err != nil {
		if err != core.ErrNotFound {
			logger.Error("find ai_provider by id error", zap.Error(err), zap.String("id", id))
		}
		return nil, err
	}

	// 返回时清空敏感字段
	provider.APIKey = ""

	return provider, nil
}

// DeleteByID 删除
func (s *AIProviderService) DeleteByID(ctx context.Context, id string) error {
	providerID, err := uuid.Parse(id)
	if err != nil {
		logger.Error("parse ai_provider id error", zap.Error(err), zap.String("id", id))
		return core.ErrBadRequest
	}

	if err := s.store.DeleteByID(ctx, providerID); err != nil {
		logger.Error("delete ai_provider error", zap.Error(err), zap.String("id", id))
		return err
	}

	return nil
}

// List 获取列表
func (s *AIProviderService) List(ctx context.Context, offset, limit int, filterActions ...filters.Filter) ([]*core.AIProvider, error) {
	providers, err := s.store.List(ctx, offset, limit, filterActions...)
	if err != nil {
		logger.Error("list ai_providers error", zap.Error(err))
		return nil, err
	}

	// 返回时清空所有敏感字段
	for _, provider := range providers {
		provider.APIKey = ""
	}

	return providers, nil
}

// Count 统计数量
func (s *AIProviderService) Count(ctx context.Context, filterActions ...filters.Filter) (int64, error) {
	count, err := s.store.Count(ctx, filterActions...)
	if err != nil {
		logger.Error("count ai_providers error", zap.Error(err))
		return 0, err
	}
	return count, nil
}
