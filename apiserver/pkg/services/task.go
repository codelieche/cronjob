package services

import (
	"context"
	"fmt"
	"time"

	"github.com/codelieche/cronjob/apiserver/pkg/config"
	"github.com/codelieche/cronjob/apiserver/pkg/core"
	"github.com/codelieche/cronjob/apiserver/pkg/utils/filters"
	"github.com/codelieche/cronjob/apiserver/pkg/utils/logger"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// NewTaskService 创建TaskService实例
func NewTaskService(store core.TaskStore, locker core.Locker) core.TaskService {
	return &TaskService{
		store:  store,
		locker: locker,
	}
}

// TaskService 任务服务实现
type TaskService struct {
	store  core.TaskStore
	locker core.Locker
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

// Cancel 取消待执行任务
//
// 🔒 使用分布式锁确保并发安全，防止与任务分发、超时检查等操作冲突
//
// 取消条件：
//  1. 任务状态必须是 pending
//  2. 成功获取任务锁
//
// 取消操作：
//  1. 更新任务状态为 canceled
//  2. 设置任务结束时间为当前时间
//
// 参数:
//   - ctx: 上下文对象
//   - id: 任务ID
//
// 返回:
//   - *core.Task: 取消后的任务信息
//   - error: 错误信息
func (s *TaskService) Cancel(ctx context.Context, id string) (*core.Task, error) {
	// 1. 解析UUID
	uuidID, err := uuid.Parse(id)
	if err != nil {
		logger.Error("解析任务ID失败", zap.Error(err), zap.String("id", id))
		return nil, core.ErrBadRequest
	}

	// 2. 🔒 获取任务锁（确保并发安全）
	lockKey := fmt.Sprintf(config.TaskLockerKeyFormat, uuidID.String())
	lockd, err := s.locker.Acquire(ctx, lockKey, 10*time.Second)
	if err != nil {
		logger.Warn("获取任务锁失败，无法取消任务",
			zap.String("task_id", uuidID.String()),
			zap.Error(err))
		return nil, fmt.Errorf("获取任务锁失败: %w", err)
	}
	defer lockd.Release(ctx)

	// 3. 重新查询任务（确保状态一致）
	task, err := s.store.FindByID(ctx, uuidID)
	if err != nil {
		if err == core.ErrNotFound {
			logger.Error("任务不存在", zap.String("id", id))
			return nil, core.ErrNotFound
		}
		logger.Error("查询任务失败", zap.Error(err), zap.String("id", id))
		return nil, err
	}

	// 4. 验证任务状态（只能取消 pending 状态的任务）
	if task.Status != core.TaskStatusPending {
		logger.Warn("只能取消pending状态的任务",
			zap.String("task_id", uuidID.String()),
			zap.String("task_name", task.Name),
			zap.String("current_status", task.Status))
		return nil, fmt.Errorf("任务状态为 %s，只能取消pending状态的任务", task.Status)
	}

	// 5. 更新任务状态
	now := time.Now()
	task.Status = core.TaskStatusCanceled
	task.TimeEnd = &now

	// 6. 保存更新
	updatedTask, err := s.store.Update(ctx, task)
	if err != nil {
		logger.Error("更新任务失败",
			zap.Error(err),
			zap.String("task_id", uuidID.String()))
		return nil, err
	}

	logger.Info("任务已取消",
		zap.String("task_id", uuidID.String()),
		zap.String("task_name", task.Name),
		zap.Time("cancel_time", now))

	return updatedTask, nil
}
