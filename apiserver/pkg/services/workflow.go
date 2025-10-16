package services

import (
	"context"
	"fmt"

	"github.com/codelieche/cronjob/apiserver/pkg/core"
	"github.com/codelieche/cronjob/apiserver/pkg/utils/filters"
	"github.com/codelieche/cronjob/apiserver/pkg/utils/logger"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// NewWorkflowService 创建 WorkflowService 实例
func NewWorkflowService(store core.WorkflowStore) core.WorkflowService {
	return &WorkflowService{
		store: store,
	}
}

// WorkflowService 工作流服务实现
type WorkflowService struct {
	store core.WorkflowStore
}

// FindByID 根据ID获取工作流
func (s *WorkflowService) FindByID(ctx context.Context, id string) (*core.Workflow, error) {
	// 解析UUID
	uuidID, err := uuid.Parse(id)
	if err != nil {
		logger.Error("parse workflow id error", zap.Error(err), zap.String("id", id))
		return nil, core.ErrBadRequest
	}

	return s.store.FindByID(ctx, uuidID)
}

// FindByCode 根据Code获取工作流（团队内唯一）
func (s *WorkflowService) FindByCode(ctx context.Context, teamID uuid.UUID, code string) (*core.Workflow, error) {
	if code == "" {
		logger.Error("workflow code is required")
		return nil, core.ErrBadRequest
	}

	workflow, err := s.store.FindByCode(ctx, teamID, code)
	if err != nil {
		if err != core.ErrNotFound {
			logger.Error("find workflow by code error", zap.Error(err), zap.String("code", code))
		}
	}
	return workflow, err
}

// Create 创建工作流
func (s *WorkflowService) Create(ctx context.Context, workflow *core.Workflow) error {
	// 验证参数
	if workflow.Name == "" {
		logger.Error("workflow name is required")
		return core.ErrBadRequest
	}

	if workflow.Code == "" {
		logger.Error("workflow code is required")
		return core.ErrBadRequest
	}

	// 验证步骤列表
	steps, err := workflow.GetSteps()
	if err != nil {
		logger.Error("parse workflow steps error", zap.Error(err))
		return core.ErrBadRequest
	}

	if len(steps) == 0 {
		logger.Error("workflow steps cannot be empty")
		return core.ErrBadRequest
	}

	// 验证步骤顺序和必需字段
	for i, step := range steps {
		if step.Order <= 0 {
			logger.Error("step order must be positive", zap.Int("index", i), zap.Int("order", step.Order))
			return core.ErrBadRequest
		}
		if step.Name == "" {
			logger.Error("step name is required", zap.Int("index", i))
			return core.ErrBadRequest
		}
		if step.Category == "" {
			logger.Error("step category is required", zap.Int("index", i))
			return core.ErrBadRequest
		}
	}

	// 检查 team_id + code 是否已存在
	if workflow.TeamID != nil && workflow.Code != "" {
		existingWorkflow, err := s.store.FindByCode(ctx, *workflow.TeamID, workflow.Code)
		if err == nil && existingWorkflow != nil {
			logger.Error("workflow code already exists",
				zap.String("code", workflow.Code),
				zap.String("team_id", workflow.TeamID.String()))
			return core.ErrConflict
		} else if err != core.ErrNotFound {
			return err
		}
	}

	// 如果指定了id，检查id是否已经存在
	if workflow.ID != uuid.Nil {
		_, err := s.FindByID(ctx, workflow.ID.String())
		if err == nil {
			logger.Error("workflow id already exists", zap.String("id", workflow.ID.String()))
			return core.ErrConflict
		} else if err != core.ErrNotFound {
			return err
		}
	} else {
		// 🔥 如果没有指定 ID，先生成一个新的 UUID（用于设置 WorkingDir）
		workflow.ID = uuid.New()
		logger.Debug("为 workflow 生成新的 UUID",
			zap.String("workflow_id", workflow.ID.String()),
			zap.String("name", workflow.Name))
	}

	// 设置默认值
	if workflow.Project == "" {
		workflow.Project = "default"
	}

	// ⭐ 设置默认 WorkingDir（如果 Metadata 中没有设置）
	metadata, err := workflow.GetMetadata()
	if err != nil {
		metadata = &core.Metadata{}
	}
	if metadata.WorkingDir == "" {
		// 默认工作目录：./workflow/:workflowID
		// 🔥 此时 workflow.ID 已经是有效的 UUID，不会是零值
		metadata.WorkingDir = fmt.Sprintf("./workflow/%s", workflow.ID.String())
		if err := workflow.SetMetadata(metadata); err != nil {
			logger.Error("设置默认 WorkingDir 失败", zap.Error(err))
			// 不影响创建流程，只记录日志
		}
		logger.Info("自动设置默认工作目录",
			zap.String("workflow_id", workflow.ID.String()),
			zap.String("working_dir", metadata.WorkingDir))
	}

	// 创建工作流
	if err := s.store.Create(ctx, workflow); err != nil {
		logger.Error("create workflow error", zap.Error(err))
		return err
	}

	logger.Info("workflow created successfully",
		zap.String("id", workflow.ID.String()),
		zap.String("code", workflow.Code),
		zap.String("name", workflow.Name))

	return nil
}

// Update 更新工作流
func (s *WorkflowService) Update(ctx context.Context, workflow *core.Workflow) error {
	if workflow.ID == uuid.Nil {
		logger.Error("workflow id is required")
		return core.ErrBadRequest
	}

	// 检查工作流是否存在
	existingWorkflow, err := s.FindByID(ctx, workflow.ID.String())
	if err != nil {
		return err
	}

	// 验证步骤列表（如果有更新）
	if len(workflow.Steps) > 0 {
		steps, err := workflow.GetSteps()
		if err != nil {
			logger.Error("parse workflow steps error", zap.Error(err))
			return core.ErrBadRequest
		}

		if len(steps) == 0 {
			logger.Error("workflow steps cannot be empty")
			return core.ErrBadRequest
		}

		// 验证步骤顺序和必需字段
		for i, step := range steps {
			if step.Order <= 0 {
				logger.Error("step order must be positive", zap.Int("index", i), zap.Int("order", step.Order))
				return core.ErrBadRequest
			}
			if step.Name == "" {
				logger.Error("step name is required", zap.Int("index", i))
				return core.ErrBadRequest
			}
			if step.Category == "" {
				logger.Error("step category is required", zap.Int("index", i))
				return core.ErrBadRequest
			}
		}
	}

	// 检查 Code 是否冲突
	if workflow.Code != "" && workflow.Code != existingWorkflow.Code {
		if workflow.TeamID != nil {
			existingByCode, err := s.store.FindByCode(ctx, *workflow.TeamID, workflow.Code)
			if err == nil && existingByCode != nil && existingByCode.ID != workflow.ID {
				logger.Error("workflow code already exists",
					zap.String("code", workflow.Code),
					zap.String("team_id", workflow.TeamID.String()))
				return core.ErrConflict
			} else if err != nil && err != core.ErrNotFound {
				return err
			}
		}
	}

	// 更新工作流
	if err := s.store.Update(ctx, workflow); err != nil {
		logger.Error("update workflow error", zap.Error(err), zap.String("id", workflow.ID.String()))
		return err
	}

	logger.Info("workflow updated successfully",
		zap.String("id", workflow.ID.String()),
		zap.String("name", workflow.Name))

	return nil
}

// Delete 删除工作流
func (s *WorkflowService) Delete(ctx context.Context, id string) error {
	// 解析UUID
	uuidID, err := uuid.Parse(id)
	if err != nil {
		logger.Error("parse workflow id error", zap.Error(err), zap.String("id", id))
		return core.ErrBadRequest
	}

	// 检查工作流是否存在
	workflow, err := s.store.FindByID(ctx, uuidID)
	if err != nil {
		return err
	}

	// 删除工作流
	if err := s.store.Delete(ctx, uuidID); err != nil {
		logger.Error("delete workflow error", zap.Error(err), zap.String("id", id))
		return err
	}

	logger.Info("workflow deleted successfully",
		zap.String("id", id),
		zap.String("name", workflow.Name))

	return nil
}

// List 查询工作流列表
func (s *WorkflowService) List(ctx context.Context, offset, limit int, actions ...filters.Filter) ([]*core.Workflow, error) {
	workflows, err := s.store.List(ctx, offset, limit, actions...)
	if err != nil {
		logger.Error("list workflows error", zap.Error(err), zap.Int("offset", offset), zap.Int("limit", limit))
		return nil, err
	}
	return workflows, nil
}

// Count 统计工作流数量
func (s *WorkflowService) Count(ctx context.Context, actions ...filters.Filter) (int64, error) {
	count, err := s.store.Count(ctx, actions...)
	if err != nil {
		logger.Error("count workflows error", zap.Error(err))
		return 0, err
	}
	return count, nil
}

// ToggleActive 切换激活状态
func (s *WorkflowService) ToggleActive(ctx context.Context, id string) (*core.Workflow, error) {
	// 解析UUID
	uuidID, err := uuid.Parse(id)
	if err != nil {
		logger.Error("parse workflow id error", zap.Error(err), zap.String("id", id))
		return nil, core.ErrBadRequest
	}

	// 获取工作流
	workflow, err := s.store.FindByID(ctx, uuidID)
	if err != nil {
		return nil, err
	}

	// 切换激活状态
	if workflow.IsActive == nil {
		falseValue := false
		workflow.IsActive = &falseValue
	} else {
		newValue := !*workflow.IsActive
		workflow.IsActive = &newValue
	}

	// 更新工作流
	if err := s.store.Update(ctx, workflow); err != nil {
		logger.Error("toggle workflow active error", zap.Error(err), zap.String("id", id))
		return nil, err
	}

	logger.Info("workflow active toggled",
		zap.String("id", id),
		zap.Bool("is_active", *workflow.IsActive))

	return workflow, nil
}

// GetStatistics 获取工作流统计信息
func (s *WorkflowService) GetStatistics(ctx context.Context, id string) (map[string]interface{}, error) {
	// 解析UUID
	uuidID, err := uuid.Parse(id)
	if err != nil {
		logger.Error("parse workflow id error", zap.Error(err), zap.String("id", id))
		return nil, core.ErrBadRequest
	}

	// 获取工作流
	workflow, err := s.store.FindByID(ctx, uuidID)
	if err != nil {
		return nil, err
	}

	// 构建统计信息
	stats := map[string]interface{}{
		"execute_count":   workflow.ExecuteCount,
		"success_count":   workflow.SuccessCount,
		"failed_count":    workflow.FailedCount,
		"last_execute_at": workflow.LastExecuteAt,
		"last_status":     workflow.LastStatus,
	}

	// 计算成功率
	if workflow.ExecuteCount > 0 {
		successRate := float64(workflow.SuccessCount) / float64(workflow.ExecuteCount) * 100
		stats["success_rate"] = fmt.Sprintf("%.2f%%", successRate)
	} else {
		stats["success_rate"] = "0.00%"
	}

	return stats, nil
}

// WorkflowService 接口定义（需要在 core/workflow.go 中定义）
// 这里只是实现，接口定义应该在 core 包中
