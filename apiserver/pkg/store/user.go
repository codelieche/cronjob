package store

import (
	"context"
	"errors"

	"github.com/codelieche/cronjob/apiserver/pkg/core"
	"github.com/codelieche/cronjob/apiserver/pkg/utils/filters"
	"gorm.io/gorm"
)

func NewUserStore(db *gorm.DB) core.UserStore {
	return &UserStore{
		db: db,
	}
}

type UserStore struct {
	db *gorm.DB
}

// Find 根据用户名获取用户
func (a *UserStore) Find(ctx context.Context, username string) (*core.User, error) {
	var user = &core.User{}
	if err := a.db.Where("username = ?", username).First(user).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, core.ErrNotFound
		}
		return nil, err
	} else {
		return user, nil
	}
}

// FindByID 根据ID获取用户
func (a *UserStore) FindByID(ctx context.Context, id int64) (*core.User, error) {
	var user = &core.User{}
	if err := a.db.Find(user, "id=?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, core.ErrNotFound
		}
		return nil, err
	} else {
		if user.ID > 0 {
			return user, nil
		} else {
			return nil, core.ErrNotFound
		}
	}
}

// Create 创建用户
func (a *UserStore) Create(ctx context.Context, user *core.User) (*core.User, error) {
	// 在事务中执行
	tx := a.db.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	if err := tx.Create(user).Error; err != nil {
		tx.Rollback()
		return nil, err
	} else {
		tx.Commit()
		// 返回创建后的对象
		return user, nil
	}
}

// Update 更新用户信息
func (a *UserStore) Update(ctx context.Context, user *core.User) (*core.User, error) {
	if user.ID <= 0 {
		err := errors.New("传入的ID无效")
		return nil, err
	}
	// 在事务中执行
	tx := a.db.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	if err := tx.Model(user).Updates(user).Error; err != nil {
		tx.Rollback()
		return nil, err
	} else {
		tx.Commit()
		// 重新获取更新后的用户信息
		updatedUser, err := a.FindByID(ctx, int64(user.ID))
		if err != nil {
			return nil, err
		}
		return updatedUser, nil
	}
}

// Delete 删除用户
func (a *UserStore) Delete(ctx context.Context, user *core.User) error {
	if user.ID <= 0 {
		return core.ErrNotFound
	} else {
		// 在事务中执行
		tx := a.db.Begin()
		defer func() {
			if r := recover(); r != nil {
				tx.Rollback()
			}
		}()

		// 检查用户是否存在
		existingUser, err := a.FindByID(ctx, int64(user.ID))
		if err != nil {
			tx.Rollback()
			return err
		} else {
			// 使用tx.Delete直接删除对象，以触发BeforeDelete钩子
			if err := tx.Delete(existingUser).Error; err != nil {
				tx.Rollback()
				return err
			}
			tx.Commit()
			return nil
		}
	}
}

// DeleteByID 根据ID删除用户
func (a *UserStore) DeleteByID(ctx context.Context, id int64) error {
	// 检查用户是否存在
	user, err := a.FindByID(ctx, id)
	if err != nil {
		return err
	}

	// 在事务中执行
	tx := a.db.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// 使用tx.Delete直接删除对象，以触发BeforeDelete钩子
	if err := tx.Delete(user).Error; err != nil {
		tx.Rollback()
		return err
	}
	tx.Commit()
	return nil
}

// List 获取用户列表
func (a *UserStore) List(ctx context.Context, offset int, limit int, filterActions ...filters.Filter) (users []*core.User, err error) {
	// 构建查询
	query := a.db.Model(&core.User{}).
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
	if err := query.Find(&users).Error; err != nil {
		return nil, err
	} else {
		return users, nil
	}
}

// Count 统计用户数量
func (a *UserStore) Count(ctx context.Context, filterActions ...filters.Filter) (int64, error) {
	var count int64
	query := a.db.Model(&core.User{})

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

// GetOrCreate 获取或者创建用户
func (a *UserStore) GetOrCreate(ctx context.Context, user *core.User) (*core.User, error) {
	existingUser, err := a.Find(ctx, user.Username)
	if err == nil {
		// 用户已存在，返回现有用户
		return existingUser, nil
	} else if err != core.ErrNotFound {
		// 其他错误
		return nil, err
	}

	// 用户不存在，创建新用户
	return a.Create(ctx, user)
}
