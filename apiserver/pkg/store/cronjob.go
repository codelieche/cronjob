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

// NewCronJobStore 创建CronJobStore实例
func NewCronJobStore(db *gorm.DB) core.CronJobStore {
	return &CronJobStore{
		db: db,
	}
}

// CronJobStore 定时任务存储实现
type CronJobStore struct {
	db *gorm.DB
}

// FindByID 根据ID获取定时任务
func (s *CronJobStore) FindByID(ctx context.Context, id uuid.UUID) (*core.CronJob, error) {
	var cronJob = &core.CronJob{}
	if err := s.db.Find(cronJob, "id=?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, core.ErrNotFound
		}
		return nil, err
	} else {
		if cronJob.ID != uuid.Nil {
			return cronJob, nil
		} else {
			return nil, core.ErrNotFound
		}
	}
}

// FindByName 根据名称获取定时任务
func (s *CronJobStore) FindByName(ctx context.Context, name string) (*core.CronJob, error) {
	var cronJob = &core.CronJob{}
	if err := s.db.Where("name = ?", name).First(cronJob).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, core.ErrNotFound
		}
		return nil, err
	} else {
		return cronJob, nil
	}
}

// Create 创建定时任务
func (s *CronJobStore) Create(ctx context.Context, cronJob *core.CronJob) (*core.CronJob, error) {
	// 检查是否已存在同名定时任务
	existingCronJob, err := s.FindByProjectAndName(ctx, cronJob.Project, cronJob.Name)
	if err == nil && existingCronJob != nil {
		return nil, core.ErrConflict
	} else if err != core.ErrNotFound {
		return nil, err
	}

	// 生成UUID
	if cronJob.ID == uuid.Nil {
		cronJob.ID = uuid.New()
	}

	// 确保Category不为空，默认为default
	if cronJob.Category == "" {
		cronJob.Category = "default"
	}

	// 在事务中执行
	tx := s.db.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	if err := tx.Create(cronJob).Error; err != nil {
		tx.Rollback()
		return nil, err
	} else {
		tx.Commit()
		// 返回创建后的对象
		return cronJob, nil
	}
}

// Update 更新定时任务信息
func (s *CronJobStore) Update(ctx context.Context, cronJob *core.CronJob) (*core.CronJob, error) {
	if cronJob.ID == uuid.Nil {
		err := errors.New("传入的ID无效")
		return nil, err
	}

	// 检查定时任务是否存在
	existingCronJob, err := s.FindByID(ctx, cronJob.ID)
	if err != nil {
		return nil, err
	}

	// 如果名称有变化，检查新名称是否已存在
	if cronJob.Name != "" && cronJob.Name != existingCronJob.Name {
		_, err := s.FindByName(ctx, cronJob.Name)
		if err == nil {
			return nil, core.ErrConflict
		} else if err != core.ErrNotFound {
			return nil, err
		}
	}

	// 确保Category不为空
	if cronJob.Category == "" {
		cronJob.Category = "default"
	}

	// 在事务中执行
	tx := s.db.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// 🔥 使用 Select() 强制更新所有字段，包括零值字段（如 max_retry=0, is_active=false）
	// 明确指定要更新的字段列表
	updateFields := []string{
		"project", "category", "name", "time", "command", "args", "description",
		"is_active", "save_log", "timeout", "metadata",
		"max_retry", "retryable", // 🔥 包含重试配置字段
	}

	if err := tx.Model(cronJob).Select(updateFields).Updates(cronJob).Error; err != nil {
		tx.Rollback()
		return nil, err
	} else {
		tx.Commit()
		// 重新获取更新后的定时任务信息
		updatedCronJob, err := s.FindByID(ctx, cronJob.ID)
		if err != nil {
			return nil, err
		}
		return updatedCronJob, nil
	}
}

// Delete 删除定时任务
func (s *CronJobStore) Delete(ctx context.Context, cronJob *core.CronJob) error {
	if cronJob.ID == uuid.Nil {
		return core.ErrNotFound
	} else {
		// 在事务中执行
		tx := s.db.Begin()
		defer func() {
			if r := recover(); r != nil {
				tx.Rollback()
			}
		}()

		// 检查定时任务是否存在
		existingCronJob, err := s.FindByID(ctx, cronJob.ID)
		if err != nil {
			tx.Rollback()
			return err
		} else {
			// 使用tx.Delete直接删除对象，以触发BeforeDelete钩子
			if err := tx.Delete(existingCronJob).Error; err != nil {
				tx.Rollback()
				return err
			}
			tx.Commit()
			return nil
		}
	}
}

// DeleteByID 根据ID删除定时任务
func (s *CronJobStore) DeleteByID(ctx context.Context, id uuid.UUID) error {
	// 检查定时任务是否存在
	CronJob, err := s.FindByID(ctx, id)
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

	// 使用tx.Delete直接删除对象，以触发BeforeDelete钩子
	if err := tx.Delete(CronJob).Error; err != nil {
		tx.Rollback()
		return err
	}
	tx.Commit()
	return nil
}

// List 获取定时任务列表
func (s *CronJobStore) List(ctx context.Context, offset int, limit int, filterActions ...filters.Filter) (CronJobs []*core.CronJob, err error) {
	// 构建查询
	query := s.db.Model(&core.CronJob{}).
		Offset(offset).Limit(limit)

	// 应用过滤条件
	if filterActions != nil && len(filterActions) > 0 {
		for _, action := range filterActions {
			if action == nil {
				continue
			}
			query = action.Filter(query)
		}
	}

	// 执行查询
	if err = query.Find(&CronJobs).Error; err != nil {
		return nil, err
	} else {
		return CronJobs, nil
	}
}

// FindByProjectAndName 根据项目和名称获取定时任务
func (s *CronJobStore) FindByProjectAndName(ctx context.Context, project string, name string) (*core.CronJob, error) {
	var cronJob = &core.CronJob{}
	if err := s.db.Where("project = ? AND name = ?", project, name).First(cronJob).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, core.ErrNotFound
		}
		return nil, err
	} else {
		return cronJob, nil
	}
}

// Count 统计定时任务数量
func (s *CronJobStore) Count(ctx context.Context, filterActions ...filters.Filter) (int64, error) {
	var count int64

	// 构建查询
	query := s.db.Model(&core.CronJob{})

	// 应用过滤条件
	if filterActions != nil && len(filterActions) > 0 {
		for _, action := range filterActions {
			if action == nil {
				continue
			}
			query = action.Filter(query)
		}
	}

	// 执行统计
	if err := query.Count(&count).Error; err != nil {
		return 0, err
	} else {
		return count, nil
	}
}

// GetOrCreate 获取或者创建定时任务
func (s *CronJobStore) GetOrCreate(ctx context.Context, cronJob *core.CronJob) (*core.CronJob, error) {
	// 检查定时任务是否已存在
	existingCronJob, err := s.FindByName(ctx, cronJob.Name)
	if err == nil && existingCronJob != nil {
		return existingCronJob, nil
	} else if err != core.ErrNotFound {
		return nil, err
	}

	// 如果不存在，创建新的定时任务
	return s.Create(ctx, cronJob)
}

// Patch 动态更新定时任务字段
func (s *CronJobStore) Patch(ctx context.Context, id uuid.UUID, updates map[string]interface{}) error {
	// 检查ID是否有效
	if id == uuid.Nil {
		return errors.New("传入的ID无效")
	}

	// 检查定时任务是否存在
	cronJob, err := s.FindByID(ctx, id)
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
	if err := tx.Model(cronJob).Select(fields).Updates(updates).Error; err != nil {
		tx.Rollback()
		return err
	} else {
		tx.Commit()
		return nil
	}
}

// BatchUpdateNullLastPlan 批量更新is_active=true且last_plan为NULL的CronJob
// 用于初始化新建CronJob的last_plan字段，避免无法调度
//
// 参数:
//   - ctx: 上下文
//   - lastPlan: 要设置的last_plan时间
//
// 返回值:
//   - affectedRows: 更新的行数
//   - error: 操作错误
func (s *CronJobStore) BatchUpdateNullLastPlan(ctx context.Context, lastPlan time.Time) (int64, error) {
	result := s.db.Model(&core.CronJob{}).
		Where("is_active = ?", true).
		Where("last_plan IS NULL").
		Where("deleted_at IS NULL").
		Update("last_plan", lastPlan)

	if result.Error != nil {
		return 0, result.Error
	}

	return result.RowsAffected, nil
}
