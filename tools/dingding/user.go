package dingding

import (
	"log"
	"strings"

	"github.com/juju/errors"
)

// 根据用户ID或者dingId获取到用户
func GetUserByid(userId string) (user *User, err error) {
	userId = strings.TrimSpace(userId)
	if userId == "" {
		err = errors.New("传入的ID不可为空")
		return nil, err
	}

	user = &User{}

	db.First(user, "id=? or ding_id=?", userId, userId)
	if user.ID > 0 {
		// 获取到了用户
		return user, nil
	} else {
		// 未获取到
		return nil, NotFountError
	}
}

// 根据用户名字获取到用户
func GetUserByName(name string) (user *User, err error) {
	name = strings.TrimSpace(name)
	if name == "" {
		err = errors.New("传入的名字不可为空")
		return nil, err
	}
	user = &User{}
	log.Println(name)

	db.First(user, "username = ?", name)
	if user.ID > 0 {
		// 获取到了用户
		return user, nil
	} else {
		// 未获取到
		return nil, NotFountError
	}
}

// 获取用户的部门
func GetUserDepartments(user *User) (departments []Department, err error) {
	//log.Println("Get User Departments")
	departments = []Department{}
	query := db.Model(user).Related(&departments, "Departments")
	if query.Error != nil {
		return nil, query.Error
	} else {
		return departments, nil
	}
}