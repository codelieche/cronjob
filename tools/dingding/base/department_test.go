package dingding

import (
	"log"
	"testing"
)

// 测试根据部门ID获取部门相关信息
func TestGetDepartmentByid(t *testing.T) {
	departmentId := 2
	// departmentId := 118434421
	var (
		department *Department
		err        error
	)

	// 通过ID获取用户
	if department, err = GetDepartmentByid(departmentId); err != nil {
		t.Error(err)
		return
	} else {
		log.Println(department.DingID, department.Name)
		// 获取用户的部门
		if users, err := GetDepartmentUsers(department); err != nil {
			t.Error(err.Error())
			return
		} else {
			// 输出用户
			log.Println("获取到用户个数：", len(users))
			for _, user := range users {
				log.Println(user.Username, user.DingID, user.Mobile)
			}
		}
	}
}

// 测试根据部门名字获取部门
func TestGetDepartmentByName(t *testing.T) {
	name := "技术部"
	// departmentId := "118434421"
	var (
		department *Department
		err        error
	)

	// 通过ID获取用户
	if department, err = GetDepartmentByName(name); err != nil {
		t.Error(err)
		return
	} else {
		log.Println(department.DingID, department.Name)
		// 获取用户的部门
		if users, err := GetDepartmentUsers(department); err != nil {
			t.Error(err.Error())
			return
		} else {
			// 输出用户
			log.Println("获取到用户个数：", len(users))
			for _, user := range users {
				log.Println(user.ID, user.Username, user.DingID, user.Position, user.Mobile)
			}
		}
	}
}
