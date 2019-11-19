package services

import (
	"log"

	"github.com/codelieche/cronjob/tools/dingding/common"
	"github.com/codelieche/cronjob/tools/dingding/datamodels"
	"github.com/codelieche/cronjob/tools/dingding/repositories"
)

// 消息Service 接口
type MessageService interface {
	// 通过ID获取消息详情
	GetById(id int64) (message *datamodels.Message, err error)
	// 获取消息列表
	GetList(offset int, limit int) (messages []*datamodels.Message, err error)
	// 发送工作消息
	SendWorkMessage(workerMessage *common.WorkerMessage, message *datamodels.Message) (*datamodels.Message, error)

	//	获取用户
	GetUserByName(name string) (user *datamodels.User, err error)
	GetUserByMobile(mobile string) (user *datamodels.User, err error)
}

// 实例化消息Service
func NewMessageService(repo repositories.MessageRepository, userRepo repositories.UserRepository) MessageService {
	if userRepo == nil {
		log.Println("请传入User Repository")
	}
	return &messageService{repo: repo, userRepo: userRepo}
}

// 消息Service
type messageService struct {
	repo     repositories.MessageRepository
	userRepo repositories.UserRepository
}

// 通过ID获取消息详情
func (s *messageService) GetById(id int64) (message *datamodels.Message, err error) {
	return s.repo.GetById(id)
}

// 获取消息列表
func (s *messageService) GetList(offset int, limit int) (messages []*datamodels.Message, err error) {
	return s.repo.List(offset, limit)
}

// 创建工作消息
func (s *messageService) SendWorkMessage(
	workerMessage *common.WorkerMessage, message *datamodels.Message) (*datamodels.Message, error) {
	var (
		success bool
		err     error
	)
	if success, err = s.repo.SendWorkerMessage(workerMessage, message); err != nil {
		return nil, err
	} else {
		success = success
		return message, nil
	}
}

// 获取用户
func (s *messageService) GetUserByName(name string) (user *datamodels.User, err error) {
	return s.userRepo.GetByName(name)
}
func (s *messageService) GetUserByMobile(mobile string) (user *datamodels.User, err error) {
	return s.userRepo.GetByMobile(mobile)
}
