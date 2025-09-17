package services

import (
	"context"

	"github.com/codelieche/cronjob/apiserver/pkg/core"
	"github.com/codelieche/cronjob/apiserver/pkg/utils/filters"
	"github.com/codelieche/cronjob/apiserver/pkg/utils/logger"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// NewCronJobService 创建CronJobService实例
func NewCronJobService(store core.CronJobStore) core.CronJobService {
	return &CronJobService{
		store: store,
	}
}

// CronJobService 定时任务服务实现
type CronJobService struct {
	store core.CronJobStore
}

// FindByID 根据ID获取定时任务
func (s *CronJobService) FindByID(ctx context.Context, id string) (*core.CronJob, error) {
	// 解析UUID
	uuidID, err := uuid.Parse(id)
	if err != nil {
		logger.Error("parse id error", zap.Error(err), zap.String("id", id))
		return nil, core.ErrBadRequest
	}

	return s.store.FindByID(ctx, uuidID)
}

// FindByName 根据名称获取定时任务
func (s *CronJobService) FindByName(ctx context.Context, name string) (*core.CronJob, error) {
	CronJob, err := s.store.FindByName(ctx, name)
	if err != nil {
		if err != core.ErrNotFound {
			logger.Error("find CronJob by name error", zap.Error(err), zap.String("name", name))
		}
	}
	return CronJob, err
}

// FindByProjectAndName 根据项目和名称获取定时任务
func (s *CronJobService) FindByProjectAndName(ctx context.Context, project string, name string) (*core.CronJob, error) {
	CronJob, err := s.store.FindByProjectAndName(ctx, project, name)
	if err != nil {
		if err != core.ErrNotFound {
			logger.Error("find CronJob by project and name error", zap.Error(err), zap.String("project", project), zap.String("name", name))
		}
	}
	return CronJob, err
}

// Create 创建定时任务
func (s *CronJobService) Create(ctx context.Context, cronJob *core.CronJob) (*core.CronJob, error) {
	// 验证参数
	if cronJob.Name == "" {
		logger.Error("CronJob name is required")
		return nil, core.ErrBadRequest
	}

	// 验证时间表达式
	if cronJob.Time == "" {
		logger.Error("CronJob time expression is required")
		return nil, core.ErrBadRequest
	}

	// 验证命令
	if cronJob.Command == "" {
		logger.Error("CronJob command is required")
		return nil, core.ErrBadRequest
	}

	// 检查定时任务是否已存在
	existingCronJob, err := s.FindByProjectAndName(ctx, cronJob.Project, cronJob.Name)
	// 相同项目之间的定时任务名称不能重复
	if err == nil && existingCronJob != nil {
		logger.Error("CronJob already exists", zap.String("name", cronJob.Name))
		return nil, core.ErrConflict
	} else if err != core.ErrNotFound {
		return nil, err
	}

	// 如果指定了id，还需要判断id是否已经存在
	if cronJob.ID != uuid.Nil {
		_, err := s.FindByID(ctx, cronJob.ID.String())
		if err == nil {
			logger.Error("CronJob id already exists", zap.String("id", cronJob.ID.String()))
			return nil, core.ErrConflict
		} else if err != core.ErrNotFound {
			return nil, err
		}
	}

	// 确保Category不为空，默认为default
	if cronJob.Category == "" {
		cronJob.Category = "default"
	}

	result, err := s.store.Create(ctx, cronJob)
	if err != nil {
		logger.Error("create CronJob error", zap.Error(err))
	}
	return result, err
}

// Update 更新定时任务信息
func (s *CronJobService) Update(ctx context.Context, cronJob *core.CronJob) (*core.CronJob, error) {
	// 验证参数
	if cronJob.ID == uuid.Nil {
		logger.Error("CronJob id is required")
		return nil, core.ErrBadRequest
	}

	// 检查定时任务是否存在
	existingCronJob, err := s.store.FindByID(ctx, cronJob.ID)
	if err != nil {
		logger.Error("find CronJob by id error", zap.Error(err), zap.String("id", cronJob.ID.String()))
		return nil, err
	}

	// 如果名称有变化，检查新名称是否已存在
	if cronJob.Name != "" && cronJob.Name != existingCronJob.Name {
		_, err := s.FindByName(ctx, cronJob.Name)
		if err == nil {
			logger.Error("CronJob name already exists", zap.String("name", cronJob.Name))
			return nil, core.ErrConflict
		} else if err != core.ErrNotFound {
			return nil, err
		}
	}

	// 确保Category不为空
	if cronJob.Category == "" {
		cronJob.Category = "default"
	}

	result, err := s.store.Update(ctx, cronJob)
	if err != nil {
		logger.Error("update CronJob error", zap.Error(err), zap.String("id", cronJob.ID.String()))
	}
	return result, err
}

// Delete 删除定时任务
func (s *CronJobService) Delete(ctx context.Context, cronJob *core.CronJob) error {
	// 验证参数
	if cronJob.ID == uuid.Nil {
		logger.Error("CronJob id is required")
		return core.ErrBadRequest
	}

	// 检查定时任务是否存在
	existingCronJob, err := s.store.FindByID(ctx, cronJob.ID)
	if err != nil {
		logger.Error("find CronJob by id error", zap.Error(err), zap.String("id", cronJob.ID.String()))
		return err
	}

	err = s.store.Delete(ctx, existingCronJob)
	if err != nil {
		logger.Error("delete CronJob error", zap.Error(err), zap.String("id", cronJob.ID.String()))
	}
	return err
}

// DeleteByID 根据ID删除定时任务
func (s *CronJobService) DeleteByID(ctx context.Context, id string) error {
	// 解析UUID
	uuidID, err := uuid.Parse(id)
	if err != nil {
		logger.Error("parse id error", zap.Error(err), zap.String("id", id))
		return core.ErrBadRequest
	}

	// 检查定时任务是否存在
	_, err = s.store.FindByID(ctx, uuidID)
	if err != nil {
		logger.Error("find CronJob by id error", zap.Error(err), zap.String("id", id))
		return err
	}

	return s.store.DeleteByID(ctx, uuidID)
}

// List 获取定时任务列表
func (s *CronJobService) List(ctx context.Context, offset int, limit int, filterActions ...filters.Filter) (cronJobs []*core.CronJob, err error) {
	cronJobs, err = s.store.List(ctx, offset, limit, filterActions...)
	if err != nil {
		logger.Error("list CronJobs error", zap.Error(err), zap.Int("offset", offset), zap.Int("limit", limit))
	}
	return cronJobs, err
}

// Count 统计定时任务数量
func (s *CronJobService) Count(ctx context.Context, filterActions ...filters.Filter) (int64, error) {
	count, err := s.store.Count(ctx, filterActions...)
	if err != nil {
		logger.Error("count CronJobs error", zap.Error(err))
	}
	return count, err
}

// GetOrCreate 获取或者创建定时任务
func (s *CronJobService) GetOrCreate(ctx context.Context, cronJob *core.CronJob) (*core.CronJob, error) {
	// 验证参数
	if cronJob.Name == "" {
		logger.Error("CronJob name is required")
		return nil, core.ErrBadRequest
	}

	// 确保Category不为空，默认为default
	if cronJob.Category == "" {
		cronJob.Category = "default"
	}

	result, err := s.store.GetOrCreate(ctx, cronJob)
	if err != nil {
		logger.Error("get or create CronJob error", zap.Error(err))
	}
	return result, err
}

// Patch 动态更新定时任务字段
func (s *CronJobService) Patch(ctx context.Context, id string, updates map[string]interface{}) error {
	// 解析UUID
	uuidID, err := uuid.Parse(id)
	if err != nil {
		logger.Error("parse id error", zap.Error(err), zap.String("id", id))
		return core.ErrBadRequest
	}

	// 验证定时任务是否存在
	cronJob, err := s.store.FindByID(ctx, uuidID)
	if err != nil {
		logger.Error("find CronJob by id error", zap.Error(err), zap.String("id", id))
		return err
	}

	// 验证字段有效性 - 定义哪些字段可以被修改
	validFields := map[string]bool{
		"project":       true,
		"category":      true,
		"name":          true,
		"time":          true,
		"command":       true,
		"args":          true,
		"description":   true,
		"is_active":     true,
		"save_log":      true,
		"last_status":   true,
		"last_dispatch": true,
		"timeout":       true,
	}

	// 过滤出有效的更新字段
	var needUpdates map[string]interface{} = map[string]interface{}{}
	for field := range updates {
		if _, ok := validFields[field]; !ok {
			logger.Error("invalid cronjob field", zap.String("field", field))
			// 传递了不可更新的字段，我们跳过即可，不需要报错
		} else {
			needUpdates[field] = updates[field]
		}
	}

	// 检查名称是否有变化，如果有变化需要检查是否已存在
	if name, ok := needUpdates["name"].(string); ok && name != "" && name != cronJob.Name {
		_, err := s.FindByName(ctx, name)
		if err == nil {
			logger.Error("CronJob name already exists", zap.String("name", name))
			return core.ErrConflict
		} else if err != core.ErrNotFound {
			return err
		}
	}

	// 调用store的Patch方法进行更新
	err = s.store.Patch(ctx, uuidID, needUpdates)
	if err != nil {
		logger.Error("patch cronjob error", zap.Error(err), zap.String("id", id))
	}
	return err
}
