package repositories

import (
	"errors"
	"strings"

	"github.com/codelieche/cronjob/tools/dingding/common"

	"github.com/codelieche/cronjob/tools/dingding/datamodels"
	"github.com/jinzhu/gorm"
)

// Repository：可以直接访问数据源，并可以直接操作数据的层
// 部门操作的接口
type DepartmentRepository interface {
	// 通过id获取部门
	GetById(id int64) (department *datamodels.Department, err error)
	// 通过部门名字获取部门
	GetByName(name string) (department *datamodels.Department, err error)
	// 获取部门列表
	List(offset int, limit int) (departments []*datamodels.Department, err error)
	// 获取部门用户
	GetDepartmentUsers(department *datamodels.Department) (users []*datamodels.User, err error)
}

// 实例化一个部门操作实例
func NewDepartmentRepository(db *gorm.DB) DepartmentRepository {
	return &departmentRespository{db: db}
}

// 部门操作实例
type departmentRespository struct {
	db *gorm.DB
}

// 通过ID获取部门
func (r *departmentRespository) GetById(id int64) (department *datamodels.Department, err error) {
	//departmentId = strings.TrimSpace(departmentId)
	if id <= 0 {
		err = errors.New("传入的ID不可为空")
		return nil, err
	}

	department = &datamodels.Department{}

	r.db.First(department, "id=? or ding_id=?", id, id)
	if department.ID > 0 {
		// 获取到了用户
		return department, nil
	} else {
		// 未获取到
		return nil, common.NotFountError
	}
}

// 通过名字获取部门
func (r *departmentRespository) GetByName(name string) (department *datamodels.Department, err error) {
	name = strings.TrimSpace(name)
	if name == "" {
		err = errors.New("传入的名字不可为空")
		return nil, err
	}
	department = &datamodels.Department{}

	r.db.First(department, "name=?", name)
	if department.ID > 0 {
		// 获取到了用户
		return department, nil
	} else {
		// 未获取到
		return nil, common.NotFountError
	}
}

// 获取部门列表
func (r *departmentRespository) List(offset int, limit int) (departments []*datamodels.Department, err error) {
	query := r.db.Model(&datamodels.Department{}).Offset(offset).Limit(limit).Find(&departments)
	if query.Error != nil {
		return nil, err
	} else {
		return departments, err
	}
}

// 获取部门用户
func (r *departmentRespository) GetDepartmentUsers(department *datamodels.Department) (users []*datamodels.User, err error) {
	query := r.db.Model(department).Related(&users, "Users")
	if query.Error != nil {
		return nil, query.Error
	} else {
		return users, nil
	}
}
