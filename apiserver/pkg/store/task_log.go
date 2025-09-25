package store

import (
	"context"
	"errors"

	"github.com/codelieche/cronjob/apiserver/pkg/core"
	"github.com/codelieche/cronjob/apiserver/pkg/utils/filters"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// NewTaskLogStore 创建任务日志存储实例
func NewTaskLogStore(db *gorm.DB) core.TaskLogStore {
	return &TaskLogStore{db: db}
}

// TaskLogStore 任务日志存储实现
type TaskLogStore struct {
	db *gorm.DB
}

// FindByTaskID 根据任务ID获取任务日志
func (s *TaskLogStore) FindByTaskID(ctx context.Context, taskID uuid.UUID) (*core.TaskLog, error) {
	var taskLog = &core.TaskLog{}
	if err := s.db.Where("task_id = ?", taskID).First(taskLog).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, core.ErrNotFound
		}
		return nil, err
	}
	return taskLog, nil
}

// Create 创建任务日志
func (s *TaskLogStore) Create(ctx context.Context, taskLog *core.TaskLog) (*core.TaskLog, error) {
	// 检查是否已存在该任务的日志
	existingLog, err := s.FindByTaskID(ctx, taskLog.TaskID)
	if err == nil && existingLog != nil {
		return nil, core.ErrConflict
	} else if err != core.ErrNotFound {
		return nil, err
	}

	// 在事务中执行
	tx := s.db.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	if err := tx.Create(taskLog).Error; err != nil {
		tx.Rollback()
		return nil, err
	} else {
		tx.Commit()
		// 返回创建后的对象
		return taskLog, nil
	}
}

// Update 更新任务日志信息
func (s *TaskLogStore) Update(ctx context.Context, taskLog *core.TaskLog) (*core.TaskLog, error) {
	// Check if task log exists
	_, err := s.FindByTaskID(ctx, taskLog.TaskID)
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

	if err := tx.Model(taskLog).Updates(taskLog).Error; err != nil {
		tx.Rollback()
		return nil, err
	} else {
		tx.Commit()
		updatedTaskLog, err := s.FindByTaskID(ctx, taskLog.TaskID)
		if err != nil {
			return nil, err
		}
		return updatedTaskLog, nil
	}
}

// Delete 删除任务日志
func (s *TaskLogStore) Delete(ctx context.Context, taskLog *core.TaskLog) error {
	// Check if task log exists
	_, err := s.FindByTaskID(ctx, taskLog.TaskID)
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

	if err := tx.Delete(taskLog).Error; err != nil {
		tx.Rollback()
		return err
	} else {
		tx.Commit()
		return nil
	}
}

// DeleteByTaskID 根据任务ID删除任务日志
func (s *TaskLogStore) DeleteByTaskID(ctx context.Context, taskID uuid.UUID) error {
	// Check if task log exists
	_, err := s.FindByTaskID(ctx, taskID)
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

	if err := tx.Where("task_id = ?", taskID).Delete(&core.TaskLog{}).Error; err != nil {
		tx.Rollback()
		return err
	} else {
		tx.Commit()
		return nil
	}
}

// List 获取任务日志列表
func (s *TaskLogStore) List(ctx context.Context, offset int, limit int, filterActions ...filters.Filter) (logs []*core.TaskLog, err error) {
	query := s.db.Model(&core.TaskLog{})

	// 应用过滤器
	for _, filter := range filterActions {
		query = filter.Filter(query)
	}

	// 分页
	if limit > 0 {
		query = query.Limit(limit)
	}
	if offset > 0 {
		query = query.Offset(offset)
	}

	// 排序
	query = query.Order("created_at DESC")

	// 执行查询
	if err := query.Find(&logs).Error; err != nil {
		return nil, err
	}

	return logs, nil
}

// Count 统计任务日志数量
func (s *TaskLogStore) Count(ctx context.Context, filterActions ...filters.Filter) (int64, error) {
	query := s.db.Model(&core.TaskLog{})

	// 应用过滤器
	for _, filter := range filterActions {
		query = filter.Filter(query)
	}

	var count int64
	if err := query.Count(&count).Error; err != nil {
		return 0, err
	}

	return count, nil
}