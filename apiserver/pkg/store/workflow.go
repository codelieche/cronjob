package store

import (
	"context"
	"errors"
	"time"

	"github.com/codelieche/cronjob/apiserver/pkg/core"
	"github.com/codelieche/cronjob/apiserver/pkg/utils/filters"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// NewWorkflowStore 创建 WorkflowStore 实例
func NewWorkflowStore(db *gorm.DB) core.WorkflowStore {
	return &WorkflowStore{
		db: db,
	}
}

// WorkflowStore 工作流存储实现
type WorkflowStore struct {
	db *gorm.DB
}

// Create 创建工作流
func (s *WorkflowStore) Create(ctx context.Context, workflow *core.Workflow) error {
	// 检查 team_id + code 是否已存在
	if workflow.TeamID != nil && workflow.Code != "" {
		existing, err := s.FindByCode(ctx, *workflow.TeamID, workflow.Code)
		if err == nil && existing != nil {
			return core.ErrConflict
		} else if err != nil && err != core.ErrNotFound {
			return err
		}
	}

	// 生成UUID
	if workflow.ID == uuid.Nil {
		workflow.ID = uuid.New()
	}

	// 设置默认值
	if workflow.Project == "" {
		workflow.Project = "default"
	}

	// 在事务中执行
	tx := s.db.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	if err := tx.Create(workflow).Error; err != nil {
		tx.Rollback()
		return err
	}

	tx.Commit()
	return nil
}

// Update 更新工作流
func (s *WorkflowStore) Update(ctx context.Context, workflow *core.Workflow) error {
	if workflow.ID == uuid.Nil {
		return errors.New("无效的工作流ID")
	}

	// 检查工作流是否存在
	existing, err := s.FindByID(ctx, workflow.ID)
	if err != nil {
		return err
	}

	// 如果 Code 有变化，检查新 Code 是否已存在
	if workflow.Code != "" && workflow.Code != existing.Code {
		if workflow.TeamID != nil {
			existingByCode, err := s.FindByCode(ctx, *workflow.TeamID, workflow.Code)
			if err == nil && existingByCode != nil && existingByCode.ID != workflow.ID {
				return core.ErrConflict
			} else if err != nil && err != core.ErrNotFound {
				return err
			}
		}
	}

	// 在事务中执行
	tx := s.db.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// 明确指定要更新的字段
	// 🔥 包含Webhook相关字段
	updateFields := []string{
		"project", "code", "name", "description", "steps", "default_variables", "metadata", "is_active", "timeout",
		"webhook_enabled", "webhook_token", "webhook_ip_whitelist", "webhook_metadata",
	}

	if err := tx.Model(workflow).Select(updateFields).Updates(workflow).Error; err != nil {
		tx.Rollback()
		return err
	}

	tx.Commit()
	return nil
}

// Delete 删除工作流（软删除）
func (s *WorkflowStore) Delete(ctx context.Context, id uuid.UUID) error {
	workflow, err := s.FindByID(ctx, id)
	if err != nil {
		return err
	}

	// 在事务中执行
	tx := s.db.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// 软删除
	if err := tx.Delete(workflow).Error; err != nil {
		tx.Rollback()
		return err
	}

	tx.Commit()
	return nil
}

// FindByID 根据ID查询工作流
func (s *WorkflowStore) FindByID(ctx context.Context, id uuid.UUID) (*core.Workflow, error) {
	var workflow = &core.Workflow{}
	if err := s.db.Find(workflow, "id=?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, core.ErrNotFound
		}
		return nil, err
	}

	if workflow.ID == uuid.Nil {
		return nil, core.ErrNotFound
	}

	return workflow, nil
}

// FindByCode 根据Code查询工作流（团队内唯一）
func (s *WorkflowStore) FindByCode(ctx context.Context, teamID uuid.UUID, code string) (*core.Workflow, error) {
	var workflow = &core.Workflow{}
	query := s.db.Where("team_id = ? AND code = ?", teamID, code)

	if err := query.First(workflow).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, core.ErrNotFound
		}
		return nil, err
	}

	return workflow, nil
}

// List 查询工作流列表
// 支持过滤、搜索、排序、分页
func (s *WorkflowStore) List(ctx context.Context, offset, limit int, filterActions ...filters.Filter) ([]*core.Workflow, error) {
	var workflows []*core.Workflow
	query := s.db.Model(&core.Workflow{}).
		Offset(offset).Limit(limit)

	// 应用过滤条件
	if len(filterActions) > 0 {
		for _, action := range filterActions {
			if action == nil {
				continue
			}
			query = action.Filter(query)
		}
	}

	// 执行查询
	if err := query.Find(&workflows).Error; err != nil {
		return nil, err
	}

	return workflows, nil
}

// Count 统计工作流数量
func (s *WorkflowStore) Count(ctx context.Context, filterActions ...filters.Filter) (int64, error) {
	var count int64
	query := s.db.Model(&core.Workflow{})

	// 应用过滤条件
	if len(filterActions) > 0 {
		for _, action := range filterActions {
			if action == nil {
				continue
			}
			query = action.Filter(query)
		}
	}

	if err := query.Count(&count).Error; err != nil {
		return 0, err
	}

	return count, nil
}

// UpdateStats 更新统计信息
// 在WorkflowExecute状态变化时调用，更新执行次数和最后执行状态
//
// 调用规则：
// 1. Execute() 创建时：传 status="pending"，execute_count +1
// 2. 完成时：传 status="success/failed"，只更新 success_count/failed_count（不再更新 execute_count）
func (s *WorkflowStore) UpdateStats(ctx context.Context, id uuid.UUID, status string) error {
	workflow, err := s.FindByID(ctx, id)
	if err != nil {
		return err
	}

	// 在事务中执行
	tx := s.db.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// 更新统计信息
	now := time.Now()
	updates := map[string]interface{}{
		"last_execute_at": now,
		"last_status":     status,
	}

	// 🔥 根据状态决定更新逻辑：
	// - pending: 发起执行，execute_count +1
	// - success/failed/canceled: 执行完成，只更新 success_count/failed_count（不更新 execute_count）
	if status == core.WorkflowExecuteStatusPending {
		// 发起执行时：execute_count +1
		updates["execute_count"] = workflow.ExecuteCount + 1
	} else if status == core.WorkflowExecuteStatusSuccess {
		// 执行成功：success_count +1（不更新 execute_count）
		updates["success_count"] = workflow.SuccessCount + 1
	} else if status == core.WorkflowExecuteStatusFailed {
		// 执行失败：failed_count +1（不更新 execute_count）
		updates["failed_count"] = workflow.FailedCount + 1
	}
	// 其他状态（running, canceled）：只更新 last_status，不更新计数

	if err := tx.Model(&core.Workflow{}).Where("id = ?", id).Updates(updates).Error; err != nil {
		tx.Rollback()
		return err
	}

	tx.Commit()
	return nil
}
