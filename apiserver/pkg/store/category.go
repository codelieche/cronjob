package store

import (
	"context"
	"errors"
	"strconv"

	"github.com/codelieche/cronjob/apiserver/pkg/core"
	"github.com/codelieche/cronjob/apiserver/pkg/utils/filters"
	"gorm.io/gorm"
)

// NewCategoryStore 创建CategoryStore实例
func NewCategoryStore(db *gorm.DB) core.CategoryStore {
	return &CategoryStore{
		db: db,
	}
}

// CategoryStore 分类存储实现
type CategoryStore struct {
	db *gorm.DB
}

// FindByID 根据ID获取分类
func (s *CategoryStore) FindByID(ctx context.Context, id uint) (*core.Category, error) {
	var category = &core.Category{}
	if err := s.db.Find(category, "id=?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, core.ErrNotFound
		}
		return nil, err
	} else {
		if category.ID > 0 {
			return category, nil
		} else {
			return nil, core.ErrNotFound
		}
	}
}

// FindByCode 根据编码获取分类
func (s *CategoryStore) FindByCode(ctx context.Context, code string) (*core.Category, error) {
	var category = &core.Category{}
	if err := s.db.Where("code = ?", code).First(category).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, core.ErrNotFound
		}
		return nil, err
	} else {
		return category, nil
	}
}

// Create 创建分类
func (s *CategoryStore) Create(ctx context.Context, category *core.Category) (*core.Category, error) {
	// 检查是否已存在相同编码的分类
	existingCategory, err := s.FindByCode(ctx, category.Code)
	if err == nil && existingCategory != nil {
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

	if err := tx.Create(category).Error; err != nil {
		tx.Rollback()
		return nil, err
	} else {
		tx.Commit()
		// 返回创建后的对象
		return category, nil
	}
}

// Update 更新分类信息
func (s *CategoryStore) Update(ctx context.Context, category *core.Category) (*core.Category, error) {
	if category.ID <= 0 {
		err := errors.New("传入的ID无效")
		return nil, err
	}

	// 检查分类是否存在
	existingCategory, err := s.FindByID(ctx, category.ID)
	if err != nil {
		return nil, err
	}

	// 如果编码有变化，检查新编码是否已存在
	if category.Code != "" && category.Code != existingCategory.Code {
		_, err := s.FindByCode(ctx, category.Code)
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

	if err := tx.Model(category).Updates(category).Error; err != nil {
		tx.Rollback()
		return nil, err
	} else {
		tx.Commit()
		// 重新获取更新后的分类信息
		updatedCategory, err := s.FindByID(ctx, category.ID)
		if err != nil {
			return nil, err
		}
		return updatedCategory, nil
	}
}

// Delete 删除分类
func (s *CategoryStore) Delete(ctx context.Context, category *core.Category) error {
	if category.ID <= 0 {
		return core.ErrNotFound
	}

	// 检查分类是否存在
	_, err := s.FindByID(ctx, category.ID)
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
	if err := tx.Model(&core.Category{}).Where("id = ?", category.ID).Delete(&core.Category{}).Error; err != nil {
		tx.Rollback()
		return err
	}
	tx.Commit()
	return nil
}

// DeleteByID 根据ID删除分类
func (s *CategoryStore) DeleteByID(ctx context.Context, id uint) error {
	// 检查分类是否存在
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
	if err := tx.Model(&core.Category{}).Where("id = ?", id).Delete(&core.Category{}).Error; err != nil {
		tx.Rollback()
		return err
	}
	tx.Commit()
	return nil
}

// FindByIDOrCode 根据ID或Code获取分类
func (s *CategoryStore) FindByIDOrCode(ctx context.Context, idOrCode string) (*core.Category, error) {
	// 尝试将idOrCode解析为uint类型的ID
	id, err := strconv.ParseUint(idOrCode, 10, 32)
	if err == nil {
		// 如果解析成功，尝试通过ID查找
		category, err := s.FindByID(ctx, uint(id))
		if err == nil {
			return category, nil
		} else if err != core.ErrNotFound {
			// 如果不是未找到的错误，直接返回
			return nil, err
		}
	}

	// 如果ID解析失败或未找到，尝试通过Code查找
	return s.FindByCode(ctx, idOrCode)
}

// List 获取分类列表
func (s *CategoryStore) List(ctx context.Context, offset int, limit int, filterActions ...filters.Filter) (categories []*core.Category, err error) {
	// 构建查询
	query := s.db.Model(&core.Category{}).
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
	if err := query.Find(&categories).Error; err != nil {
		return nil, err
	}

	return categories, nil
}

// Count 统计分类数量
func (s *CategoryStore) Count(ctx context.Context, filterActions ...filters.Filter) (int64, error) {
	var count int64

	// 构建查询
	query := s.db.Model(&core.Category{})

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
	}

	return count, nil
}
