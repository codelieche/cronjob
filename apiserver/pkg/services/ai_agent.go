package services

import (
	"context"

	"github.com/codelieche/cronjob/apiserver/pkg/core"
	"github.com/codelieche/cronjob/apiserver/pkg/utils/filters"
	"github.com/codelieche/cronjob/apiserver/pkg/utils/logger"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// AIAgentService AI Agent服务实现
type AIAgentService struct {
	store core.AIAgentStore
}

// NewAIAgentService 创建AIAgentService实例
func NewAIAgentService(store core.AIAgentStore) *AIAgentService {
	return &AIAgentService{
		store: store,
	}
}

// Create 创建AI Agent
// ⭐ 自动填充team_id：如果为空，使用当前用户的team_id
func (s *AIAgentService) Create(ctx context.Context, agent *core.AIAgent, currentUserTeamID uuid.UUID) (*core.AIAgent, error) {
	// 自动填充team_id
	if agent.TeamID == uuid.Nil {
		agent.TeamID = currentUserTeamID
		logger.Info("auto fill team_id for ai_agent",
			zap.String("agent_name", agent.Name),
			zap.String("team_id", currentUserTeamID.String()))
	}

	// 创建
	created, err := s.store.Create(ctx, agent)
	if err != nil {
		logger.Error("create ai_agent error", zap.Error(err))
		return nil, err
	}

	return created, nil
}

// Update 更新AI Agent
func (s *AIAgentService) Update(ctx context.Context, agent *core.AIAgent) (*core.AIAgent, error) {
	updated, err := s.store.Update(ctx, agent)
	if err != nil {
		logger.Error("update ai_agent error", zap.Error(err))
		return nil, err
	}

	return updated, nil
}

// FindByID 根据ID查找
func (s *AIAgentService) FindByID(ctx context.Context, id string) (*core.AIAgent, error) {
	agentID, err := uuid.Parse(id)
	if err != nil {
		logger.Error("parse ai_agent id error", zap.Error(err), zap.String("id", id))
		return nil, core.ErrBadRequest
	}

	agent, err := s.store.FindByID(ctx, agentID)
	if err != nil {
		if err != core.ErrNotFound {
			logger.Error("find ai_agent by id error", zap.Error(err), zap.String("id", id))
		}
		return nil, err
	}

	return agent, nil
}

// DeleteByID 删除
func (s *AIAgentService) DeleteByID(ctx context.Context, id string) error {
	agentID, err := uuid.Parse(id)
	if err != nil {
		logger.Error("parse ai_agent id error", zap.Error(err), zap.String("id", id))
		return core.ErrBadRequest
	}

	if err := s.store.DeleteByID(ctx, agentID); err != nil {
		logger.Error("delete ai_agent error", zap.Error(err), zap.String("id", id))
		return err
	}

	return nil
}

// List 获取列表
func (s *AIAgentService) List(ctx context.Context, offset, limit int, filterActions ...filters.Filter) ([]*core.AIAgent, error) {
	agents, err := s.store.List(ctx, offset, limit, filterActions...)
	if err != nil {
		logger.Error("list ai_agents error", zap.Error(err))
		return nil, err
	}

	return agents, nil
}

// Count 统计数量
func (s *AIAgentService) Count(ctx context.Context, filterActions ...filters.Filter) (int64, error) {
	count, err := s.store.Count(ctx, filterActions...)
	if err != nil {
		logger.Error("count ai_agents error", zap.Error(err))
		return 0, err
	}
	return count, nil
}

// FindByProviderID 根据Provider ID查找Agent列表
func (s *AIAgentService) FindByProviderID(ctx context.Context, providerID string) ([]*core.AIAgent, error) {
	pid, err := uuid.Parse(providerID)
	if err != nil {
		logger.Error("parse provider id error", zap.Error(err), zap.String("provider_id", providerID))
		return nil, core.ErrBadRequest
	}

	agents, err := s.store.FindByProviderID(ctx, pid)
	if err != nil {
		logger.Error("find ai_agents by provider id error", zap.Error(err), zap.String("provider_id", providerID))
		return nil, err
	}

	return agents, nil
}
