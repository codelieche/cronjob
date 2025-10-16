package store

import (
	"context"
	"errors"

	"github.com/codelieche/cronjob/apiserver/pkg/core"
	"github.com/codelieche/cronjob/apiserver/pkg/utils/filters"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// NewWorkflowExecuteStore 创建 WorkflowExecuteStore 实例
func NewWorkflowExecuteStore(db *gorm.DB) core.WorkflowExecuteStore {
	return &WorkflowExecuteStore{
		db: db,
	}
}

// WorkflowExecuteStore 工作流执行存储实现
type WorkflowExecuteStore struct {
	db *gorm.DB
}

// Create 创建工作流执行实例
func (s *WorkflowExecuteStore) Create(ctx context.Context, execute *core.WorkflowExecute) error {
	// 生成UUID
	if execute.ID == uuid.Nil {
		execute.ID = uuid.New()
	}

	// 设置默认状态
	if execute.Status == "" {
		execute.Status = core.WorkflowExecuteStatusPending
	}

	// 在事务中执行
	tx := s.db.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	if err := tx.Create(execute).Error; err != nil {
		tx.Rollback()
		return err
	}

	tx.Commit()
	return nil
}

// Update 更新工作流执行实例
func (s *WorkflowExecuteStore) Update(ctx context.Context, execute *core.WorkflowExecute) error {
	if execute.ID == uuid.Nil {
		return errors.New("无效的工作流执行实例ID")
	}

	// 检查执行实例是否存在
	_, err := s.FindByID(ctx, execute.ID)
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

	// 明确指定要更新的字段
	updateFields := []string{
		"status", "time_start", "time_end",
		"total_steps", "completed_steps", "success_steps", "failed_steps", "current_step",
		"locked_worker_id", "locked_worker_name", "locked_working_dir",
		"variables", "metadata", "error_message",
	}

	if err := tx.Model(execute).Select(updateFields).Updates(execute).Error; err != nil {
		tx.Rollback()
		return err
	}

	tx.Commit()
	return nil
}

// Delete 删除工作流执行实例（软删除）
func (s *WorkflowExecuteStore) Delete(ctx context.Context, id uuid.UUID) error {
	execute, err := s.FindByID(ctx, id)
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
	if err := tx.Delete(execute).Error; err != nil {
		tx.Rollback()
		return err
	}

	tx.Commit()
	return nil
}

// FindByID 根据ID查询工作流执行实例
func (s *WorkflowExecuteStore) FindByID(ctx context.Context, id uuid.UUID) (*core.WorkflowExecute, error) {
	var execute = &core.WorkflowExecute{}
	if err := s.db.Find(execute, "id=?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, core.ErrNotFound
		}
		return nil, err
	}

	if execute.ID == uuid.Nil {
		return nil, core.ErrNotFound
	}

	return execute, nil
}

// List 查询工作流执行列表
// 支持过滤、搜索、排序、分页
func (s *WorkflowExecuteStore) List(ctx context.Context, offset, limit int, filterActions ...filters.Filter) ([]*core.WorkflowExecute, error) {
	var executes []*core.WorkflowExecute
	query := s.db.Model(&core.WorkflowExecute{}).
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
	if err := query.Find(&executes).Error; err != nil {
		return nil, err
	}

	return executes, nil
}

// Count 统计工作流执行数量
func (s *WorkflowExecuteStore) Count(ctx context.Context, filterActions ...filters.Filter) (int64, error) {
	var count int64
	query := s.db.Model(&core.WorkflowExecute{})

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

// ListByWorkflowID 根据WorkflowID查询执行列表
// 用于Workflow详情页的执行历史Tab
func (s *WorkflowExecuteStore) ListByWorkflowID(ctx context.Context, workflowID uuid.UUID, limit, offset int) ([]*core.WorkflowExecute, error) {
	var executes []*core.WorkflowExecute
	query := s.db.Model(&core.WorkflowExecute{}).Where("workflow_id = ?", workflowID)

	// 排序（按创建时间倒序）
	query = query.Order("created_at DESC")

	// 分页
	if limit > 0 {
		query = query.Limit(limit)
	}
	if offset > 0 {
		query = query.Offset(offset)
	}

	if err := query.Find(&executes).Error; err != nil {
		return nil, err
	}

	return executes, nil
}

// CountByWorkflowID 统计Workflow的执行次数
func (s *WorkflowExecuteStore) CountByWorkflowID(ctx context.Context, workflowID uuid.UUID) (int64, error) {
	var count int64
	query := s.db.Model(&core.WorkflowExecute{}).Where("workflow_id = ?", workflowID)

	if err := query.Count(&count).Error; err != nil {
		return 0, err
	}

	return count, nil
}
