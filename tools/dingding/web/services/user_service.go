package services

import (
	"github.com/codelieche/cronjob/tools/dingding/datamodels"
	"github.com/codelieche/cronjob/tools/dingding/repositories"
)

type UserService interface {
	GetById(idOrName string) (user *datamodels.User, success bool)
	GetByIdOrName(idOrName string) (user *datamodels.User, success bool)
	// 通过名字获取用户
	GetByName(name string) (user *datamodels.User, err error)
	// 通过手机号获取用户
	GetByMobile(mobile string) (user *datamodels.User, err error)
	List(offset int, limit int) (users []*datamodels.User, success bool)
	GetMessageList(user *datamodels.User, offset int, limit int) (messages []*datamodels.Message, success bool)
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

// 通过ID或者名字获取用户名获取到用户
func (s *userService) GetByIdOrName(idOrName string) (user *datamodels.User, success bool) {
	var (
		err error
	)
	// 通过ID或者名字获取到用户
	if user, err = s.repo.GetByIdOrName(idOrName); err != nil {
		return nil, false
	} else {
		return user, true
	}
}

// 通过名字获取用户名获取到用户
func (s *userService) GetByName(name string) (user *datamodels.User, err error) {

	// 通过name获取到用户
	if user, err = s.repo.GetByName(name); err != nil {
		return nil, err
	} else {
		return user, nil
	}
}

// 通过mobile获取用户名获取到用户
func (s *userService) GetByMobile(mobile string) (user *datamodels.User, err error) {

	// 通过mobile获取到用户
	if user, err = s.repo.GetByMobile(mobile); err != nil {
		return nil, err
	} else {
		return user, nil
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

// 获取用户的消息列表
func (s *userService) GetMessageList(
	user *datamodels.User, offset int, limit int) (messages []*datamodels.Message, success bool) {
	// 获取用户消息列表
	if messages, err := s.repo.GetUserMessagesList(user, offset, limit); err != nil {
		return nil, false
	} else {
		return messages, true
	}
}
