package services

import (
	"github.com/codelieche/cronjob/backend/common/datamodels"
	"github.com/codelieche/cronjob/backend/common/repositories"
)

type JobKillService interface {
	// 创建JobKill
	Create(jobKill *datamodels.JobKill) (*datamodels.JobKill, error)
	// 保存JobKill
	Save(jobKill *datamodels.JobKill) (*datamodels.JobKill, error)
	// 根据ID获取JobKill
	GetByID(id int64) (jobKill *datamodels.JobKill, err error)
	// 获取JobKill的列表
	List(offset int, limit int) (jobKills []*datamodels.JobKill, err error)
	// 设置JobKill为完成
	SetFinishedByID(id int64) (jobKill *datamodels.JobKill, err error)
}

func NewJobKillService(repo repositories.JobKillRepository) JobKillService {
	return &jobKillService{repo: repo}
}

type jobKillService struct {
	repo repositories.JobKillRepository
}

func (s *jobKillService) Create(jobKill *datamodels.JobKill) (*datamodels.JobKill, error) {
	return s.repo.Save(jobKill)
}

func (s *jobKillService) Save(jobKill *datamodels.JobKill) (*datamodels.JobKill, error) {
	return s.repo.Save(jobKill)
}

func (s *jobKillService) GetByID(id int64) (jobKill *datamodels.JobKill, err error) {
	return s.repo.Get(id)
}

func (s *jobKillService) List(offset int, limit int) (jobKills []*datamodels.JobKill, err error) {
	return s.repo.List(offset, limit)
}

func (s *jobKillService) SetFinishedByID(id int64) (jobKill *datamodels.JobKill, err error) {
	return s.repo.SetFinishedByID(id)
}
