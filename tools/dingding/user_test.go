package dingding

import (
	"log"
	"testing"
)

// 测试获取用户：根据用户id
func TestGetUserByid(t *testing.T) {
	userId := "1"
	var (
		user *User
		err  error
	)

	// 通过ID获取用户
	if user, err = GetUserByid(userId); err != nil {
		t.Error(err)
		return
	} else {
		//log.Println(user)
		log.Println(user.ID, user.Username, user.DingID, user.Position, user.Mobile)
		// 获取用户的部门
		if departments, err := GetUserDepartments(user); err != nil {
			t.Error(err.Error())
			return
		} else {
			// 输出部门
			for _, department := range departments {
				log.Println(department.Name, department.DingID)
			}
		}
	}
}

// 根据用户名称获取相关信息
func TestGetUserByName(t *testing.T) {
	name := "Alex.Zhou"
	var (
		user *User
		err  error
	)

	// 通过ID获取用户
	if user, err = GetUserByName(name); err != nil {
		t.Error(err)
		return
	} else {
		// log.Println(user)
		log.Println(user.ID, user.Username, user.DingID, user.Position, user.Mobile)
		// 获取用户的部门
		if departments, err := GetUserDepartments(user); err != nil {
			t.Error(err.Error())
			return
		} else {
			// 输出部门
			for _, department := range departments {
				log.Println(department.Name, department.DingID)
			}
		}
	}
}
