package services

import (
	"context"
	"regexp"
	"strconv"

	"github.com/codelieche/cronjob/apiserver/pkg/core"
	"github.com/codelieche/cronjob/apiserver/pkg/utils/filters"
)

func NewUserService(store core.UserStore) core.UserService {
	return &UserService{
		store: store,
	}
}

type UserService struct {
	store core.UserStore
}

// Find 根据用户名获取用户
func (s *UserService) Find(ctx context.Context, username string) (*core.User, error) {
	return s.store.Find(ctx, username)
}

// FindByID 根据ID获取用户
func (s *UserService) FindByID(ctx context.Context, id string) (*core.User, error) {
	if isDigit, err := regexp.Match("^\\d+$", []byte(id)); err != nil {
		return nil, err
	} else {
		if isDigit {
			if userID, err := strconv.Atoi(id); err != nil {
				return nil, err
			} else {
				return s.store.FindByID(ctx, int64(userID))
			}
		} else {
			return nil, core.ErrNotFound
		}
	}
}

// Create 创建用户
func (s *UserService) Create(ctx context.Context, user *core.User) (*core.User, error) {
	// 验证参数
	if user.Username == "" {
		return nil, core.ErrBadRequest
	}

	// 检查用户是否已存在
	existingUser, err := s.Find(ctx, user.Username)
	if err == nil && existingUser != nil {
		return nil, core.ErrConflict
	} else if err != core.ErrNotFound {
		return nil, err
	}

	return s.store.Create(ctx, user)
}

// Update 更新用户信息
func (s *UserService) Update(ctx context.Context, user *core.User) (*core.User, error) {
	// 验证参数
	if user.ID <= 0 {
		return nil, core.ErrBadRequest
	}

	// 检查用户是否存在
	existingUser, err := s.store.FindByID(ctx, int64(user.ID))
	if err != nil {
		return nil, err
	}

	// 如果用户名有变化，检查新用户名是否已存在
	if user.Username != "" && user.Username != existingUser.Username {
		_, err := s.Find(ctx, user.Username)
		if err == nil {
			return nil, core.ErrConflict
		} else if err != core.ErrNotFound {
			return nil, err
		}
	}

	return s.store.Update(ctx, user)
}

// Delete 删除用户
func (s *UserService) Delete(ctx context.Context, user *core.User) error {
	// 验证参数
	if user.ID <= 0 {
		return core.ErrBadRequest
	}

	// 检查用户是否存在
	existingUser, err := s.store.FindByID(ctx, int64(user.ID))
	if err != nil {
		return err
	}

	return s.store.Delete(ctx, existingUser)
}

// DeleteByID 根据ID删除用户
func (s *UserService) DeleteByID(ctx context.Context, id string) error {
	if isDigit, err := regexp.Match("^\\d+$", []byte(id)); err != nil {
		return err
	} else {
		if isDigit {
			if userID, err := strconv.Atoi(id); err != nil {
				return err
			} else {
				// 检查用户是否存在
				_, err := s.store.FindByID(ctx, int64(userID))
				if err != nil {
					return err
				}

				return s.store.DeleteByID(ctx, int64(userID))
			}
		} else {
			return core.ErrBadRequest
		}
	}
}

// List 获取用户列表
func (s *UserService) List(ctx context.Context, offset int, limit int, filterActions ...filters.Filter) (users []*core.User, err error) {
	return s.store.List(ctx, offset, limit, filterActions...)
}

// Count 统计用户数量
func (s *UserService) Count(ctx context.Context, filterActions ...filters.Filter) (int64, error) {
	return s.store.Count(ctx, filterActions...)
}

// GetOrCreate 获取或者创建用户
func (s *UserService) GetOrCreate(ctx context.Context, user *core.User) (*core.User, error) {
	// 验证参数
	if user.Username == "" {
		return nil, core.ErrBadRequest
	}

	return s.store.GetOrCreate(ctx, user)
}
