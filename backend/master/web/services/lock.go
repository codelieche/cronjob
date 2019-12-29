package services

import (
	"github.com/codelieche/cronjob/backend/common/datamodels"
	"github.com/codelieche/cronjob/backend/common/repositories"
)

// Lock Service
type LockService interface {
	// 创建、抢锁
	Create(name string, ttl int64) (lock *datamodels.Lock, err error)
	// 对锁续租: 后续会添加密码校验
	Lease(leaseID int64, password string) error
	// 删除锁: 释放租约
	Release(leaseID int64, password string) error
}

// 实例化LockService
func NewLockService(repo repositories.LockRepository) LockService {
	return &lockService{repo: repo}
}

type lockService struct {
	repo repositories.LockRepository
}

func (s *lockService) Create(name string, ttl int64) (lock *datamodels.Lock, err error) {
	return s.repo.Create(name, ttl)
}

func (s *lockService) Lease(leaseID int64, password string) error {
	return s.repo.Lease(leaseID)
}

func (s *lockService) Release(leaseID int64, password string) error {
	return s.repo.Release(leaseID)
}
