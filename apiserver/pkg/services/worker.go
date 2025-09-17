package services

import (
	"context"

	"github.com/codelieche/cronjob/apiserver/pkg/core"
	"github.com/codelieche/cronjob/apiserver/pkg/utils/filters"
	"github.com/codelieche/cronjob/apiserver/pkg/utils/logger"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// NewWorkerService 创建WorkerService实例
func NewWorkerService(store core.WorkerStore) core.WorkerService {
	return &WorkerService{
		store: store,
	}
}

// WorkerService 工作节点服务实现

type WorkerService struct {
	store core.WorkerStore
}

// FindByID 根据ID获取工作节点
func (s *WorkerService) FindByID(ctx context.Context, id string) (*core.Worker, error) {
	// 解析UUID
	uuidID, err := uuid.Parse(id)
	if err != nil {
		logger.Error("parse id error", zap.Error(err), zap.String("id", id))
		return nil, core.ErrBadRequest
	}

	return s.store.FindByID(ctx, uuidID)
}

// FindByName 根据名称获取工作节点
func (s *WorkerService) FindByName(ctx context.Context, name string) (*core.Worker, error) {
	worker, err := s.store.FindByName(ctx, name)
	if err != nil {
		if err != core.ErrNotFound {
			logger.Error("find worker by name error", zap.Error(err), zap.String("name", name))
		}
	}
	return worker, err
}

// Create 创建工作节点
func (s *WorkerService) Create(ctx context.Context, worker *core.Worker) (*core.Worker, error) {
	// 验证参数
	if worker.Name == "" {
		logger.Error("worker name is required")
		return nil, core.ErrBadRequest
	}

	// 检查工作节点是否已存在
	existingWorker, err := s.FindByName(ctx, worker.Name)
	if err == nil && existingWorker != nil {
		logger.Error("worker already exists", zap.String("name", worker.Name))
		return nil, core.ErrConflict
	} else if err != core.ErrNotFound {
		return nil, err
	}

	// 如果指定了id，还需要判断id是否已经存在
	if worker.ID != uuid.Nil {
		_, err := s.FindByID(ctx, worker.ID.String())
		if err == nil {
			logger.Error("worker id already exists", zap.String("id", worker.ID.String()))
			return nil, core.ErrConflict
		} else if err != core.ErrNotFound {
			return nil, err
		}
	}

	result, err := s.store.Create(ctx, worker)
	if err != nil {
		logger.Error("create worker error", zap.Error(err))
	}
	return result, err
}

// Update 更新工作节点信息
func (s *WorkerService) Update(ctx context.Context, worker *core.Worker) (*core.Worker, error) {
	// 验证参数
	if worker.ID == uuid.Nil {
		logger.Error("worker id is required")
		return nil, core.ErrBadRequest
	}

	// 检查工作节点是否存在
	existingWorker, err := s.store.FindByID(ctx, worker.ID)
	if err != nil {
		logger.Error("find worker by id error", zap.Error(err), zap.String("id", worker.ID.String()))
		return nil, err
	}

	// 如果名称有变化，检查新名称是否已存在
	if worker.Name != "" && worker.Name != existingWorker.Name {
		_, err := s.FindByName(ctx, worker.Name)
		if err == nil {
			logger.Error("worker name already exists", zap.String("name", worker.Name))
			return nil, core.ErrConflict
		} else if err != core.ErrNotFound {
			return nil, err
		}
	}

	result, err := s.store.Update(ctx, worker)
	if err != nil {
		logger.Error("update worker error", zap.Error(err), zap.String("id", worker.ID.String()))
	}
	return result, err
}

// Delete 删除工作节点
func (s *WorkerService) Delete(ctx context.Context, worker *core.Worker) error {
	// 验证参数
	if worker.ID == uuid.Nil {
		logger.Error("worker id is required")
		return core.ErrBadRequest
	}

	// 检查工作节点是否存在
	existingWorker, err := s.store.FindByID(ctx, worker.ID)
	if err != nil {
		logger.Error("find worker by id error", zap.Error(err), zap.String("id", worker.ID.String()))
		return err
	}

	err = s.store.Delete(ctx, existingWorker)
	if err != nil {
		logger.Error("delete worker error", zap.Error(err), zap.String("id", worker.ID.String()))
	}
	return err
}

// DeleteByID 根据ID删除工作节点
func (s *WorkerService) DeleteByID(ctx context.Context, id string) error {
	// 解析UUID
	uuidID, err := uuid.Parse(id)
	if err != nil {
		logger.Error("parse id error", zap.Error(err), zap.String("id", id))
		return core.ErrBadRequest
	}

	// 检查工作节点是否存在
	_, err = s.store.FindByID(ctx, uuidID)
	if err != nil {
		logger.Error("find worker by id error", zap.Error(err), zap.String("id", id))
		return err
	}

	return s.store.DeleteByID(ctx, uuidID)
}

// List 获取工作节点列表
func (s *WorkerService) List(ctx context.Context, offset int, limit int, filterActions ...filters.Filter) (workers []*core.Worker, err error) {
	workers, err = s.store.List(ctx, offset, limit, filterActions...)
	if err != nil {
		logger.Error("list workers error", zap.Error(err), zap.Int("offset", offset), zap.Int("limit", limit))
	}
	return workers, err
}

// Count 统计工作节点数量
func (s *WorkerService) Count(ctx context.Context, filterActions ...filters.Filter) (int64, error) {
	count, err := s.store.Count(ctx, filterActions...)
	if err != nil {
		logger.Error("count workers error", zap.Error(err))
	}
	return count, err
}

// GetOrCreate 获取或者创建工作节点
func (s *WorkerService) GetOrCreate(ctx context.Context, worker *core.Worker) (*core.Worker, error) {
	// 验证参数
	if worker.Name == "" {
		logger.Error("worker name is required")
		return nil, core.ErrBadRequest
	}

	result, err := s.store.GetOrCreate(ctx, worker)
	if err != nil {
		logger.Error("get or create worker error", zap.Error(err))
	}
	return result, err
}
