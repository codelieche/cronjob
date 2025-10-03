package services

import (
	"context"
	"time"

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

// CheckAndUpdateInactiveWorkers 检查并更新失活的worker
// 如果worker的last_active超过指定时间（默认5分钟），将其is_active设置为false
func (s *WorkerService) CheckAndUpdateInactiveWorkers(ctx context.Context, inactiveDuration time.Duration) (int, error) {
	// 计算截止时间：当前时间 - inactiveDuration
	cutoffTime := time.Now().Add(-inactiveDuration)

	// 查询所有is_active=true且last_active < cutoffTime的worker
	filterActions := []filters.Filter{
		&filters.FilterOption{
			Column: "is_active",
			Value:  1, // true
			Op:     filters.FILTER_EQ,
		},
		&filters.FilterOption{
			Column: "last_active",
			Value:  cutoffTime.Format("2006-01-02 15:04:05"),
			Op:     filters.FILTER_LT,
		},
	}

	// 获取需要更新的worker列表
	workers, err := s.store.List(ctx, 0, 1000, filterActions...)
	if err != nil {
		logger.Error("查询失活worker失败", zap.Error(err))
		return 0, err
	}

	// 更新每个失活worker的状态
	updatedCount := 0
	isActive := false
	for _, worker := range workers {
		worker.IsActive = &isActive
		_, err := s.store.Update(ctx, worker)
		if err != nil {
			logger.Error("更新worker状态失败",
				zap.Error(err),
				zap.String("worker_id", worker.ID.String()),
				zap.String("worker_name", worker.Name))
		} else {
			updatedCount++
			logger.Info("Worker已标记为失活",
				zap.String("worker_id", worker.ID.String()),
				zap.String("worker_name", worker.Name),
				zap.Time("last_active", *worker.LastActive))
		}
	}

	return updatedCount, nil
}

// CheckWorkerStatusLoop 循环检查worker状态的后台任务
// 定期检查worker的last_active，将超时的worker标记为失活
func (s *WorkerService) CheckWorkerStatusLoop(ctx context.Context, checkInterval time.Duration, inactiveDuration time.Duration) {
	logger.Info("开始运行Worker状态检查循环",
		zap.Duration("check_interval", checkInterval),
		zap.Duration("inactive_duration", inactiveDuration))

	ticker := time.NewTicker(checkInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			logger.Info("Worker状态检查循环被取消")
			return
		case <-ticker.C:
			// 执行检查并更新
			updatedCount, err := s.CheckAndUpdateInactiveWorkers(ctx, inactiveDuration)
			if err != nil {
				logger.Error("检查失活worker时发生错误", zap.Error(err))
			} else if updatedCount > 0 {
				logger.Info("已更新失活worker状态", zap.Int("count", updatedCount))
			}
		}
	}
}
