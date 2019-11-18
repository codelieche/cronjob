package services

import (
	"github.com/codelieche/cronjob/tools/dingding/datamodels"
	"github.com/codelieche/cronjob/tools/dingding/repositories"
)

type UserService interface {
	GetById(idOrName string) (user *datamodels.User, success bool)
	List(offset int, limit int) (users []*datamodels.User, success bool)
}

func NewUserService(repo repositories.UserRepository) UserService {
	return &userService{repo: repo}
}

// User Service
type userService struct {
	repo repositories.UserRepository
}

// 通过ID获取用户名获取到用户
func (s *userService) GetById(idOrName string) (user *datamodels.User, success bool) {
	var (
		err error
	)
	if user, err = s.repo.GetById(idOrName); err != nil {
		// 再次通过名字获取一下
		if user, err = s.repo.GetByName(idOrName); err != nil {
			return nil, false
		} else {
			return user, true
		}
	} else {
		return user, true
	}
}

// 获取用户列表
func (s *userService) List(offset int, limit int) (users []*datamodels.User, success bool) {
	if users, err := s.repo.List(offset, limit); err != nil {
		return nil, false
	} else {
		return users, true
	}
}
