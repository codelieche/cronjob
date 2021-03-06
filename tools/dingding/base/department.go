package base

import (
	"strings"

	"github.com/juju/errors"
)

// 根据部门ID或者dingId获取到用户
func GetDepartmentByid(departmentId int) (department *Department, err error) {
	//departmentId = strings.TrimSpace(departmentId)
	if departmentId <= 0 {
		err = errors.New("传入的ID不可为空")
		return nil, err
	}

	department = &Department{}

	db.First(department, "id=? or ding_id=?", departmentId, departmentId)
	if department.ID > 0 {
		// 获取到了用户
		return department, nil
	} else {
		// 未获取到
		return nil, NotFountError
	}
}

// 根据部门名字获取到用户
func GetDepartmentByName(name string) (department *Department, err error) {
	name = strings.TrimSpace(name)
	if name == "" {
		err = errors.New("传入的名字不可为空")
		return nil, err
	}
	department = &Department{}

	db.First(department, "name=?", name)
	if department.ID > 0 {
		// 获取到了用户
		return department, nil
	} else {
		// 未获取到
		return nil, NotFountError
	}
}

// 获取部门的用户
func GetDepartmentUsers(department *Department) (users []User, err error) {
	//log.Println("Get Department User")
	query := db.Model(department).Related(&users, "Users")
	if query.Error != nil {
		return nil, query.Error
	} else {
		return users, nil
	}
}

// 获取部门列表
func GetDepartmentList(offset int, limit int) (departments []*Department, err error) {
	//users = []User{}
	query := db.Model(&Department{}).Offset(offset).Limit(limit).Find(&departments)
	if query.Error != nil {
		return nil, err
	} else {
		return departments, err
	}
}
