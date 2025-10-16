package store

import (
	"context"
	"errors"

	"github.com/codelieche/cronjob/apiserver/pkg/core"
	"github.com/codelieche/cronjob/apiserver/pkg/utils/filters"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// NewWorkerStore 创建WorkerStore实例
func NewWorkerStore(db *gorm.DB) core.WorkerStore {
	return &WorkerStore{
		db: db,
	}
}

// WorkerStore 工作节点存储实现

type WorkerStore struct {
	db *gorm.DB
}

// FindByID 根据ID获取工作节点
func (s *WorkerStore) FindByID(ctx context.Context, id uuid.UUID) (*core.Worker, error) {
	var worker = &core.Worker{}
	if err := s.db.Find(worker, "id=?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, core.ErrNotFound
		}
		return nil, err
	} else {
		if worker.ID != uuid.Nil {
			return worker, nil
		} else {
			return nil, core.ErrNotFound
		}
	}
}

// FindByName 根据名称获取工作节点
func (s *WorkerStore) FindByName(ctx context.Context, name string) (*core.Worker, error) {
	var worker = &core.Worker{}
	if err := s.db.Where("name = ?", name).First(worker).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, core.ErrNotFound
		}
		return nil, err
	} else {
		return worker, nil
	}
}

// Create 创建工作节点
func (s *WorkerStore) Create(ctx context.Context, worker *core.Worker) (*core.Worker, error) {
	// 检查是否已存在同名工作节点
	existingWorker, err := s.FindByName(ctx, worker.Name)
	if err == nil && existingWorker != nil {
		return nil, core.ErrConflict
	} else if err != core.ErrNotFound {
		return nil, err
	}

	// 生成UUID
	if worker.ID == uuid.Nil {
		worker.ID = uuid.New()
	}

	// 在事务中执行
	tx := s.db.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	if err := tx.Create(worker).Error; err != nil {
		tx.Rollback()
		return nil, err
	} else {
		tx.Commit()
		// 返回创建后的对象
		return worker, nil
	}
}

// Update 更新工作节点信息
func (s *WorkerStore) Update(ctx context.Context, worker *core.Worker) (*core.Worker, error) {
	if worker.ID == uuid.Nil {
		err := errors.New("传入的ID无效")
		return nil, err
	}

	// 检查工作节点是否存在
	existingWorker, err := s.FindByID(ctx, worker.ID)
	if err != nil {
		return nil, err
	}

	// 如果名称有变化，检查新名称是否已存在
	if worker.Name != "" && worker.Name != existingWorker.Name {
		_, err := s.FindByName(ctx, worker.Name)
		if err == nil {
			return nil, core.ErrConflict
		} else if err != core.ErrNotFound {
			return nil, err
		}
	}

	// 在事务中执行
	tx := s.db.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	if err := tx.Model(worker).Updates(worker).Error; err != nil {
		tx.Rollback()
		return nil, err
	} else {
		tx.Commit()
		// 重新获取更新后的工作节点信息
		updatedWorker, err := s.FindByID(ctx, worker.ID)
		if err != nil {
			return nil, err
		}
		return updatedWorker, nil
	}
}

// Delete 删除工作节点
func (s *WorkerStore) Delete(ctx context.Context, worker *core.Worker) error {
	if worker.ID == uuid.Nil {
		return core.ErrNotFound
	}

	// 检查工作节点是否存在
	_, err := s.FindByID(ctx, worker.ID)
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

	// 🔥 使用Model().Where().Delete()方式，明确指定WHERE条件
	if err := tx.Model(&core.Worker{}).Where("id = ?", worker.ID).Delete(&core.Worker{}).Error; err != nil {
		tx.Rollback()
		return err
	}
	tx.Commit()
	return nil
}

// DeleteByID 根据ID删除工作节点
func (s *WorkerStore) DeleteByID(ctx context.Context, id uuid.UUID) error {
	// 检查工作节点是否存在
	_, err := s.FindByID(ctx, id)
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

	// 🔥 使用Model().Where().Delete()方式，明确指定WHERE条件
	if err := tx.Model(&core.Worker{}).Where("id = ?", id).Delete(&core.Worker{}).Error; err != nil {
		tx.Rollback()
		return err
	}
	tx.Commit()
	return nil
}

// List 获取工作节点列表
func (s *WorkerStore) List(ctx context.Context, offset int, limit int, filterActions ...filters.Filter) (workers []*core.Worker, err error) {
	// 构建查询
	query := s.db.Model(&core.Worker{}).
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
	if err := query.Find(&workers).Error; err != nil {
		return nil, err
	} else {
		return workers, nil
	}
}

// Count 统计工作节点数量
func (s *WorkerStore) Count(ctx context.Context, filterActions ...filters.Filter) (int64, error) {
	var count int64
	query := s.db.Model(&core.Worker{})

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

// GetOrCreate 获取或者创建工作节点
func (s *WorkerStore) GetOrCreate(ctx context.Context, worker *core.Worker) (*core.Worker, error) {
	existingWorker, err := s.FindByName(ctx, worker.Name)
	if err == nil {
		// 工作节点已存在，返回现有工作节点
		return existingWorker, nil
	} else if err != core.ErrNotFound {
		// 其他错误
		return nil, err
	}

	// 工作节点不存在，创建新工作节点
	return s.Create(ctx, worker)
}
