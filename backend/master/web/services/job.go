package services

import (
	"github.com/codelieche/cronjob/backend/common/datamodels"
	"github.com/codelieche/cronjob/backend/common/repositories"
)

// Job Service Interface
type JobService interface {
	// 创建Job
	Create(job *datamodels.Job) (*datamodels.Job, error)
	// 保存Job
	Save(job *datamodels.Job) (*datamodels.Job, error)
	// 根据ID获取Job
	GetByID(id int64) (job *datamodels.Job, err error)
	// 根据分类和ID获取分类
	// 获取Job的列表
	List(offset int, limit int) (jobs []*datamodels.Job, err error)
	// 删除Job
	Delete(job *datamodels.Job) (err error)
	// 更新Job
	Update(job *datamodels.Job, fields map[string]interface{}) (*datamodels.Job, error)
	// 更新Job
	UpdateByID(id int64, fields map[string]interface{}) (*datamodels.Job, error)
	// 根据ID或者Name获取分类
	GetCategoryByIDOrName(idOrName string) (category *datamodels.Category, err error)
}

// 实例化Job Service
func NewJobService(repo repositories.JobRepository) JobService {
	return &jobService{repo: repo}
}

// job Service
type jobService struct {
	repo repositories.JobRepository
}

// 创建Job
func (s *jobService) Create(job *datamodels.Job) (*datamodels.Job, error) {
	return s.repo.Save(job)
}

// 保存Job
func (s *jobService) Save(job *datamodels.Job) (*datamodels.Job, error) {
	return s.repo.Save(job)
}

// 根据ID获取Job
func (s *jobService) GetByID(id int64) (job *datamodels.Job, err error) {
	return s.repo.Get(id)
}

// 获取Job的列表
func (s *jobService) List(offset int, limit int) (jobs []*datamodels.Job, err error) {
	return s.repo.List(offset, limit)
}

// 删除Job
func (s *jobService) Delete(job *datamodels.Job) (err error) {
	return s.repo.Delete(job)
}

// 更新Job
func (s *jobService) Update(job *datamodels.Job, fields map[string]interface{}) (*datamodels.Job, error) {
	return s.repo.Update(job, fields)
}

// 跟新Job
func (s *jobService) UpdateByID(id int64, fields map[string]interface{}) (*datamodels.Job, error) {
	return s.repo.UpdateByID(id, fields)
}

func (s *jobService) GetCategoryByIDOrName(idOrName string) (category *datamodels.Category, err error) {
	return s.repo.GetCategoryByIDOrName(idOrName)
}
