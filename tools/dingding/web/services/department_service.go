package services

import (
	"github.com/codelieche/cronjob/tools/dingding/datamodels"
	"github.com/codelieche/cronjob/tools/dingding/repositories"
)

// 部门Service接口
type DepartmentService interface {
	// 获取部门详情
	GetById(id int64) (department *datamodels.Department, err error)
	// 获取部门列表
	GetList(offset int, limit int) (departments []*datamodels.Department, err error)
	// 获取部门用户列表
	GetUserList(department *datamodels.Department, offset int, limit int) (users []*datamodels.User, err error)
}

func NewDepartmentService(repo repositories.DepartmentRepository) DepartmentService {
	return &departmentService{repo: repo}
}

// 部门Service
type departmentService struct {
	repo repositories.DepartmentRepository
}

// 通过ID获取部门详情
func (s *departmentService) GetById(id int64) (department *datamodels.Department, err error) {
	return s.repo.GetById(id)
}

// 获取部门列表
func (s *departmentService) GetList(offset int, limit int) (departments []*datamodels.Department, err error) {
	return s.repo.List(offset, limit)
}

// 获取部门用户列表
func (s *departmentService) GetUserList(
	department *datamodels.Department, offset int, limit int) (users []*datamodels.User, err error) {
	return s.repo.GetDepartmentUsers(department)
}
