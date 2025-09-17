package services

import (
	"context"

	"github.com/codelieche/cronjob/apiserver/pkg/core"
	"github.com/codelieche/cronjob/apiserver/pkg/utils/filters"
	"github.com/codelieche/cronjob/apiserver/pkg/utils/logger"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// NewTaskService 创建TaskService实例
func NewTaskService(store core.TaskStore) core.TaskService {
	return &TaskService{
		store: store,
	}
}

// TaskService 任务服务实现
type TaskService struct {
	store core.TaskStore
}

// FindByID 根据ID获取任务
func (s *TaskService) FindByID(ctx context.Context, id string) (*core.Task, error) {
	// 解析UUID
	uuidID, err := uuid.Parse(id)
	if err != nil {
		logger.Error("parse id error", zap.Error(err), zap.String("id", id))
		return nil, core.ErrBadRequest
	}

	return s.store.FindByID(ctx, uuidID)
}

// Create 创建任务
func (s *TaskService) Create(ctx context.Context, task *core.Task) (*core.Task, error) {
	// 验证参数
	if task.Name == "" {
		logger.Error("task name is required")
		return nil, core.ErrBadRequest
	}

	// 设置默认值
	if task.Project == "" {
		task.Project = "default"
	}

	if task.Category == "" {
		task.Category = "default"
	}

	if task.Status == "" {
		task.Status = core.TaskStatusPending
	}

	// 生成UUID
	if task.ID == uuid.Nil {
		task.ID = uuid.New()
	} else {
		// 如果指定了id，还需要判断id是否已经存在
		_, err := s.FindByID(ctx, task.ID.String())
		if err == nil {
			logger.Error("task id already exists", zap.String("id", task.ID.String()))
			return nil, core.ErrConflict
		} else if err != core.ErrNotFound {
			return nil, err
		}
	}

	result, err := s.store.Create(ctx, task)
	if err != nil {
		logger.Error("create task error", zap.Error(err))
	}
	return result, err
}

// Update 更新任务信息
func (s *TaskService) Update(ctx context.Context, task *core.Task) (*core.Task, error) {
	// 验证参数
	if task.ID == uuid.Nil {
		logger.Error("task id is required")
		return nil, core.ErrBadRequest
	}

	// 检查任务是否存在
	existingTask, err := s.store.FindByID(ctx, task.ID)
	if err != nil || existingTask.ID != task.ID {
		logger.Error("find task by id error", zap.Error(err), zap.String("id", task.ID.String()))
		return nil, err
	}

	result, err := s.store.Update(ctx, task)
	if err != nil {
		logger.Error("update task error", zap.Error(err), zap.String("id", task.ID.String()))
	}
	return result, err
}

// Delete 删除任务
func (s *TaskService) Delete(ctx context.Context, task *core.Task) error {
	if task.ID == uuid.Nil {
		logger.Error("task id is required")
		return core.ErrBadRequest
	}

	err := s.store.Delete(ctx, task)
	if err != nil {
		logger.Error("delete task error", zap.Error(err), zap.String("id", task.ID.String()))
	}
	return err
}

// DeleteByID 根据ID删除任务
func (s *TaskService) DeleteByID(ctx context.Context, id string) error {
	// 解析UUID
	uuidID, err := uuid.Parse(id)
	if err != nil {
		logger.Error("parse id error", zap.Error(err), zap.String("id", id))
		return core.ErrBadRequest
	}

	err = s.store.DeleteByID(ctx, uuidID)
	if err != nil {
		logger.Error("delete task by id error", zap.Error(err), zap.String("id", id))
	}
	return err
}

// List 获取任务列表
func (s *TaskService) List(ctx context.Context, offset int, limit int, filterActions ...filters.Filter) (tasks []*core.Task, err error) {
	tasks, err = s.store.List(ctx, offset, limit, filterActions...)
	if err != nil {
		logger.Error("list tasks error", zap.Error(err))
	}
	return tasks, err
}

// Count 统计任务数量
func (s *TaskService) Count(ctx context.Context, filterActions ...filters.Filter) (int64, error) {
	count, err := s.store.Count(ctx, filterActions...)
	if err != nil {
		logger.Error("count tasks error", zap.Error(err))
	}
	return count, err
}

// UpdateStatus 更新任务状态
func (s *TaskService) UpdateStatus(ctx context.Context, id string, status string) error {
	// 验证状态是否有效
	validStatus := map[string]bool{
		core.TaskStatusPending:  true,
		core.TaskStatusRunning:  true,
		core.TaskStatusSuccess:  true,
		core.TaskStatusFailed:   true,
		core.TaskStatusError:    true,
		core.TaskStatusTimeout:  true,
		core.TaskStatusCanceled: true,
		core.TaskStatusRetrying: true,
	}

	if _, ok := validStatus[status]; !ok {
		logger.Error("invalid task status", zap.String("status", status))
		return core.ErrBadRequest
	}

	// 解析UUID
	uuidID, err := uuid.Parse(id)
	if err != nil {
		logger.Error("parse id error", zap.Error(err), zap.String("id", id))
		return core.ErrBadRequest
	}

	err = s.store.UpdateStatus(ctx, uuidID, status)
	if err != nil {
		logger.Error("update task status error", zap.Error(err), zap.String("id", id), zap.String("status", status))
	}
	return err
}

// UpdateOutput 更新任务输出
func (s *TaskService) UpdateOutput(ctx context.Context, id string, output string) error {
	// 解析UUID
	uuidID, err := uuid.Parse(id)
	if err != nil {
		logger.Error("parse id error", zap.Error(err), zap.String("id", id))
		return core.ErrBadRequest
	}

	err = s.store.UpdateOutput(ctx, uuidID, output)
	if err != nil {
		logger.Error("update task output error", zap.Error(err), zap.String("id", id))
	}
	return err
}

// Patch 动态更新任务字段
func (s *TaskService) Patch(ctx context.Context, id string, updates map[string]interface{}) error {
	// 解析UUID
	uuidID, err := uuid.Parse(id)
	if err != nil {
		logger.Error("parse id error", zap.Error(err), zap.String("id", id))
		return core.ErrBadRequest
	}

	// 验证字段有效性
	// 我们这里只允许修改某一些字段，比如：status, worker_id, worker_name, is_standalone, output
	validFields := map[string]bool{
		"cronjob":       true,
		"next":          true,
		"status":        true,
		"worker_id":     true,
		"worker_name":   true,
		"time_start":    true,
		"time_end":      true,
		"is_standalone": true,
		"output":        true,
		"is_group":      true,
		"task_order":    true,
		"timeout":       true,
	}
	var needUpdates map[string]interface{} = map[string]interface{}{}
	for field := range updates {
		if _, ok := validFields[field]; !ok {
			logger.Error("invalid task field", zap.String("field", field))
			// return core.ErrBadRequest
			// 传递了不可更新的字段，我们跳过即可，不需要报错，反正不会更新不可更新的字段
		} else {
			needUpdates[field] = updates[field]
		}
	}

	// 可以在这里添加对updates中字段的验证逻辑
	// 例如，检查状态字段的有效性、字段长度等

	err = s.store.Patch(ctx, uuidID, needUpdates)
	if err != nil {
		logger.Error("patch task error", zap.Error(err), zap.String("id", id))
	}
	return err
}
