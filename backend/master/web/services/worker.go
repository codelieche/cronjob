package services

import (
	"github.com/codelieche/cronjob/backend/common/datamodels"
	"github.com/codelieche/cronjob/backend/common/repositories"
)

type WorkerService interface {
	// 创建Worker
	Create(worker *datamodels.Worker) (*datamodels.Worker, error)
	// 获取Worker
	Get(name string) (worker *datamodels.Worker, err error)
	// 删除Worker
	Delete(worker *datamodels.Worker) (success bool, err error)
	DeleteByName(name string) (success bool, err error)
	// 工作节点的列表
	List() (workersList []*datamodels.Worker, err error)
}

func NewWorkerService(repo repositories.WorkerRepository) WorkerService {
	return &workerService{repo: repo}
}

type workerService struct {
	repo repositories.WorkerRepository
}

func (s *workerService) Create(worker *datamodels.Worker) (*datamodels.Worker, error) {
	return s.repo.Create(worker)
}

func (s *workerService) Get(name string) (worker *datamodels.Worker, err error) {
	return s.repo.Get(name)
}

func (s *workerService) Delete(worker *datamodels.Worker) (success bool, err error) {
	return s.repo.Delete(worker)
}

// 根据Worker的名字删除
func (s *workerService) DeleteByName(name string) (success bool, err error) {
	return s.repo.DeleteByName(name)
}

func (s *workerService) List() (workersList []*datamodels.Worker, err error) {
	return s.repo.List()
}
