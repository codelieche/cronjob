package services

import (
	"github.com/codelieche/cronjob/backend/common/datamodels"
	"github.com/codelieche/cronjob/backend/common/repositories"
)

// JobExecute Service Interface
type JobExecuteService interface {
	// 创建JobExecute
	Create(jobExecute *datamodels.JobExecute) (*datamodels.JobExecute, error)
	// 根据ID获取JobExecute
	GetByID(id int64) (jobExecute *datamodels.JobExecute, err error)
	// 获取JobExecute的列表
	List(offset int, limit int) (jobExecutes []*datamodels.JobExecute, err error)
	// 更新
	Update(jobExecute *datamodels.JobExecute, fields map[string]interface{}) (*datamodels.JobExecute, error)
	// 根据ID更新
	UpdateByID(id int64, fields map[string]interface{}) (jobExecute *datamodels.JobExecute, err error)

	// 回写执行结果信息
	SaveExecuteLog(jobExecuteResult *datamodels.JobExecuteResult) (jobExecute *datamodels.JobExecute, err error)

	// 获取JobExecute的Log
	GetExecuteLog(jobExecute *datamodels.JobExecute) (jobExecuteLog *datamodels.JobExecuteLog, err error)
	GetExecuteLogByID(id int64) (jobExecuteLog *datamodels.JobExecuteLog, err error)
}

func NewJobExecuteService(repo repositories.JobExecuteRepository) JobExecuteService {
	return &jobExecuteService{repo: repo}
}

type jobExecuteService struct {
	repo repositories.JobExecuteRepository
}

func (s *jobExecuteService) Create(jobExecute *datamodels.JobExecute) (*datamodels.JobExecute, error) {
	return s.repo.Create(jobExecute)
}

func (s *jobExecuteService) GetByID(id int64) (jobExecute *datamodels.JobExecute, err error) {
	return s.repo.Get(id)
}

func (s *jobExecuteService) List(offset int, limit int) (jobExecutes []*datamodels.JobExecute, err error) {
	return s.repo.List(offset, limit)
}

func (s *jobExecuteService) Update(jobExecute *datamodels.JobExecute, fields map[string]interface{}) (*datamodels.JobExecute, error) {
	return s.repo.Update(jobExecute, fields)
}

func (s *jobExecuteService) UpdateByID(id int64, fields map[string]interface{}) (jobExecute *datamodels.JobExecute, err error) {
	return s.repo.UpdateByID(id, fields)
}

func (s *jobExecuteService) SaveExecuteLog(jobExecuteResult *datamodels.JobExecuteResult) (jobExecute *datamodels.JobExecute, err error) {
	return s.repo.SaveExecuteLog(jobExecuteResult)
}

func (s *jobExecuteService) GetExecuteLog(jobExecute *datamodels.JobExecute) (jobExecuteLog *datamodels.JobExecuteLog, err error) {
	return s.repo.GetExecuteLog(jobExecute)
}

func (s *jobExecuteService) GetExecuteLogByID(id int64) (jobExecuteLog *datamodels.JobExecuteLog, err error) {
	return s.repo.GetExecuteLogByID(id)
}
