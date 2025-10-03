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

// NewTaskStore 创建TaskStore实例
func NewTaskStore(db *gorm.DB) core.TaskStore {
	return &TaskStore{
		db: db,
	}
}

// TaskStore 任务存储实现
type TaskStore struct {
	db *gorm.DB
}

// FindByID 根据ID获取任务
func (s *TaskStore) FindByID(ctx context.Context, id uuid.UUID) (*core.Task, error) {
	var task = &core.Task{}
	if err := s.db.Find(task, "id=?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, core.ErrNotFound
		}
		return nil, err
	} else {
		if task.ID != uuid.Nil {
			return task, nil
		} else {
			return nil, core.ErrNotFound
		}
	}
}

// Create 创建任务
func (s *TaskStore) Create(ctx context.Context, task *core.Task) (*core.Task, error) {
	// 生成UUID
	if task.ID == uuid.Nil {
		task.ID = uuid.New()
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

	// 在事务中执行
	tx := s.db.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	if err := tx.Create(task).Error; err != nil {
		tx.Rollback()
		return nil, err
	} else {
		tx.Commit()
		// 返回创建后的对象
		return task, nil
	}
}

// Update 更新任务信息
func (s *TaskStore) Update(ctx context.Context, task *core.Task) (*core.Task, error) {
	if task.ID == uuid.Nil {
		err := errors.New("传入的ID无效")
		return nil, err
	}

	// 检查任务是否存在
	existingTask, err := s.FindByID(ctx, task.ID)
	if err != nil {
		return nil, err
	}

	// 在事务中执行
	tx := s.db.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// 🔥 使用 Select() 强制更新所有字段，包括零值字段（如 retry_count=0, retryable=false）
	// 明确指定要更新的字段列表
	updateFields := []string{
		"project", "category", "cronjob", "name", "is_group", "task_order",
		"previous", "next", "command", "args", "description",
		"time_plan", "timeout_at", "time_start", "time_end",
		"status", "output", "save_log",
		"retry_count", "max_retry", "retryable", "failure_reason", "next_retry_time", // 🔥 包含重试配置字段
		"worker_id", "worker_name", "is_standalone", "timeout", "metadata",
	}

	// 更新任务信息
	if err := tx.Model(existingTask).Select(updateFields).Updates(task).Error; err != nil {
		tx.Rollback()
		return nil, err
	} else {
		tx.Commit()
		// 返回更新后的对象
		return existingTask, nil
	}
}

// Delete 删除任务
func (s *TaskStore) Delete(ctx context.Context, task *core.Task) error {
	if task.ID == uuid.Nil {
		return errors.New("传入的任务ID无效")
	}

	// 在事务中执行
	tx := s.db.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	if err := tx.Delete(task).Error; err != nil {
		tx.Rollback()
		return err
	} else {
		tx.Commit()
		return nil
	}
}

// DeleteByID 根据ID删除任务
func (s *TaskStore) DeleteByID(ctx context.Context, id uuid.UUID) error {
	// 先获取任务
	task, err := s.FindByID(ctx, id)
	if err != nil {
		return err
	}

	// 删除任务
	return s.Delete(ctx, task)
}

// List 获取任务列表
func (s *TaskStore) List(ctx context.Context, offset int, limit int, filterActions ...filters.Filter) (tasks []*core.Task, err error) {
	tx := s.db.Model(&core.Task{})

	// 应用过滤器
	for _, action := range filterActions {
		tx = action.Filter(tx)
	}

	// 分页
	tx = tx.Offset(offset).Limit(limit)

	// 获取列表
	if err = tx.Find(&tasks).Error; err != nil {
		return nil, err
	}

	return tasks, nil
}

// Count 统计任务数量
func (s *TaskStore) Count(ctx context.Context, filterActions ...filters.Filter) (int64, error) {
	var count int64
	tx := s.db.Model(&core.Task{})

	// 应用过滤器
	for _, action := range filterActions {
		tx = action.Filter(tx)
	}

	// 统计数量
	if err := tx.Count(&count).Error; err != nil {
		return 0, err
	}

	return count, nil
}

// UpdateStatus 更新任务状态
func (s *TaskStore) UpdateStatus(ctx context.Context, id uuid.UUID, status string) error {
	// 先获取任务
	task, err := s.FindByID(ctx, id)
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

	// 更新状态
	if err := tx.Model(task).Update("status", status).Error; err != nil {
		tx.Rollback()
		return err
	} else {
		tx.Commit()
		return nil
	}
}

// UpdateOutput 更新任务输出
func (s *TaskStore) UpdateOutput(ctx context.Context, id uuid.UUID, output string) error {
	// 先获取任务
	task, err := s.FindByID(ctx, id)
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

	// 更新输出
	if err := tx.Model(task).Update("output", output).Error; err != nil {
		tx.Rollback()
		return err
	} else {
		tx.Commit()
		return nil
	}
}

// Patch 动态更新任务字段
func (s *TaskStore) Patch(ctx context.Context, id uuid.UUID, updates map[string]interface{}) error {
	// 检查ID是否有效
	if id == uuid.Nil {
		return errors.New("传入的ID无效")
	}

	// 检查任务是否存在
	task, err := s.FindByID(ctx, id)
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

	// 🔥 使用 Select() 明确指定要更新的字段，避免 GORM 忽略零值（如 false, 0）
	// 提取 updates 中的所有字段名
	var fields []string
	for field := range updates {
		fields = append(fields, field)
	}

	// 使用 Select() 指定更新字段，然后用 Updates() 批量更新
	if err := tx.Model(task).Select(fields).Updates(updates).Error; err != nil {
		tx.Rollback()
		return err
	}

	// 🔥 如果任务状态更新为完成状态，且任务关联了CronJob，则同步更新CronJob的LastDispatch和LastStatus
	if status, ok := updates["status"].(string); ok && task.CronJob != nil && *task.CronJob != uuid.Nil {
		// 判断是否为完成状态
		completedStatus := map[string]bool{
			core.TaskStatusSuccess:  true,
			core.TaskStatusFailed:   true,
			core.TaskStatusError:    true,
			core.TaskStatusTimeout:  true,
			core.TaskStatusCanceled: true,
		}

		if completedStatus[status] {
			// 更新CronJob的LastDispatch和LastStatus
			cronJobUpdates := map[string]interface{}{
				"last_status": status,
			}

			// 优先使用任务的结束时间，如果没有则使用当前时间
			// 1. 优先使用本次更新中的 time_end（如果有）
			// 2. 其次使用任务原有的 time_end（如果有）
			// 3. 最后使用当前时间
			if timeEnd, ok := updates["time_end"]; ok {
				cronJobUpdates["last_dispatch"] = timeEnd
			} else if task.TimeEnd != nil {
				cronJobUpdates["last_dispatch"] = task.TimeEnd
			} else {
				cronJobUpdates["last_dispatch"] = time.Now()
			}

			// 更新CronJob
			// 注意：last_status 和 last_dispatch 不会是零值，所以不需要使用 Select()
			if err := tx.Table("cronjobs").Where("id = ?", task.CronJob).Updates(cronJobUpdates).Error; err != nil {
				tx.Rollback()
				return err
			}
		}
	}

	tx.Commit()
	return nil
}
