package services

import (
	"context"
	"strconv"

	"github.com/codelieche/cronjob/apiserver/pkg/core"
	"github.com/codelieche/cronjob/apiserver/pkg/utils/filters"
	"github.com/codelieche/cronjob/apiserver/pkg/utils/logger"
	"go.uber.org/zap"
)

// NewCategoryService 创建CategoryService实例
func NewCategoryService(store core.CategoryStore) core.CategoryService {
	return &CategoryService{
		store: store,
	}
}

// CategoryService 分类服务实现
type CategoryService struct {
	store core.CategoryStore
}

// FindByID 根据ID获取分类
func (s *CategoryService) FindByID(ctx context.Context, id uint) (*core.Category, error) {
	category, err := s.store.FindByID(ctx, id)
	if err != nil {
		if err != core.ErrNotFound {
			logger.Error("find category by id error", zap.Error(err), zap.Uint("id", id))
		}
	}
	return category, err
}

// FindByCode 根据编码获取分类
func (s *CategoryService) FindByCode(ctx context.Context, code string) (*core.Category, error) {
	category, err := s.store.FindByCode(ctx, code)
	if err != nil {
		if err != core.ErrNotFound {
			logger.Error("find category by code error", zap.Error(err), zap.String("code", code))
		}
	}
	return category, err
}

// Create 创建分类
func (s *CategoryService) Create(ctx context.Context, category *core.Category) (*core.Category, error) {
	// 验证参数
	if category.Code == "" {
		logger.Error("category code is required")
		return nil, core.ErrBadRequest
	}

	// 检查分类是否已存在
	existingCategory, err := s.FindByCode(ctx, category.Code)
	if err == nil && existingCategory != nil {
		logger.Error("category already exists", zap.String("code", category.Code))
		return nil, core.ErrConflict
	} else if err != core.ErrNotFound {
		return nil, err
	}

	result, err := s.store.Create(ctx, category)
	if err != nil {
		logger.Error("create category error", zap.Error(err))
	}
	return result, err
}

// Update 更新分类信息
func (s *CategoryService) Update(ctx context.Context, category *core.Category) (*core.Category, error) {
	// 验证参数
	if category.ID <= 0 {
		logger.Error("category id is required")
		return nil, core.ErrBadRequest
	}

	// 检查分类是否存在
	existingCategory, err := s.store.FindByID(ctx, category.ID)
	if err != nil {
		logger.Error("find category by id error", zap.Error(err), zap.Uint("id", category.ID))
		return nil, err
	}

	// 如果编码有变化，检查新编码是否已存在
	if category.Code != "" && category.Code != existingCategory.Code {
		_, err := s.FindByCode(ctx, category.Code)
		if err == nil {
			logger.Error("category code already exists", zap.String("code", category.Code))
			return nil, core.ErrConflict
		} else if err != core.ErrNotFound {
			return nil, err
		}
	}

	result, err := s.store.Update(ctx, category)
	if err != nil {
		logger.Error("update category error", zap.Error(err), zap.Uint("id", category.ID))
	}
	return result, err
}

// Delete 删除分类
func (s *CategoryService) Delete(ctx context.Context, category *core.Category) error {
	// 验证参数
	if category.ID <= 0 {
		logger.Error("category id is required")
		return core.ErrBadRequest
	}

	// 检查分类是否存在
	existingCategory, err := s.store.FindByID(ctx, category.ID)
	if err != nil {
		logger.Error("find category by id error", zap.Error(err), zap.Uint("id", category.ID))
		return err
	}

	err = s.store.Delete(ctx, existingCategory)
	if err != nil {
		logger.Error("delete category error", zap.Error(err), zap.Uint("id", category.ID))
	}
	return err
}

// DeleteByID 根据ID删除分类
func (s *CategoryService) DeleteByID(ctx context.Context, id uint) error {
	// 检查分类是否存在
	_, err := s.store.FindByID(ctx, id)
	if err != nil {
		logger.Error("find category by id error", zap.Error(err), zap.Uint("id", id))
		return err
	}

	return s.store.DeleteByID(ctx, id)
}

// List 获取分类列表
func (s *CategoryService) List(ctx context.Context, offset int, limit int, filterActions ...filters.Filter) (categories []*core.Category, err error) {
	categories, err = s.store.List(ctx, offset, limit, filterActions...)
	if err != nil {
		logger.Error("list categories error", zap.Error(err), zap.Int("offset", offset), zap.Int("limit", limit))
	}
	return categories, err
}

// Count 统计分类数量
func (s *CategoryService) Count(ctx context.Context, filterActions ...filters.Filter) (int64, error) {
	count, err := s.store.Count(ctx, filterActions...)
	if err != nil {
		logger.Error("count categories error", zap.Error(err))
	}
	return count, err
}

// DeleteByIDString 将字符串ID转换为uint并删除分类
func (s *CategoryService) DeleteByIDString(ctx context.Context, idStr string) error {
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		logger.Error("parse id error", zap.Error(err), zap.String("id", idStr))
		return core.ErrBadRequest
	}

	return s.DeleteByID(ctx, uint(id))
}

// FindByIDOrCode 根据ID或Code获取分类
func (s *CategoryService) FindByIDOrCode(ctx context.Context, idOrCode string) (*core.Category, error) {
	category, err := s.store.FindByIDOrCode(ctx, idOrCode)
	if err != nil {
		if err != core.ErrNotFound {
			logger.Error("find category by id or code error", zap.Error(err), zap.String("idOrCode", idOrCode))
		}
	}
	return category, err
}
