package repositories

import (
	"log"
	"testing"

	"github.com/codelieche/cronjob/tools/dingding/datamodels"
	"github.com/codelieche/cronjob/tools/dingding/datasource"
)

// 根据ID获取用户
func TestUserRepository_GetById(t *testing.T) {
	userId := "1"
	var (
		user *datamodels.User
		err  error
	)

	db := datasource.DB
	r := NewUserRepository(db)

	// 通过ID获取用户
	if user, err = r.GetById(userId); err != nil {
		t.Error(err)
		return
	} else {
		//log.Println(user)
		log.Println(user.ID, user.Username, user.DingID, user.Position, user.Mobile)
		// 获取用户的部门
		if departments, err := r.GetUserDepartments(user); err != nil {
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

// 根据名字获取用户
func TestUserRepository_GetByName(t *testing.T) {
	name := "Alex.Zhou"
	var (
		user *datamodels.User
		err  error
	)

	db := datasource.DB
	r := NewUserRepository(db)

	// 通过ID获取用户
	if user, err = r.GetByName(name); err != nil {
		t.Error(err)
		return
	} else {
		// log.Println(user)
		log.Println(user.ID, user.Username, user.DingID, user.Position, user.Mobile)
		// 获取用户的部门
		if departments, err := r.GetUserDepartments(user); err != nil {
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

// 获取用户列表
func TestUserRepository_List(t *testing.T) {
	var offset int = 0
	var limit int = 1
	var haveNext = true

	// 先实例化Repository
	db := datasource.DB
	r := NewUserRepository(db)

	for haveNext {
		if users, err := r.List(offset, limit); err != nil {
			t.Error(err.Error())
		} else {
			// 判断是否有下一页
			if len(users) != limit {
				haveNext = false
			} else {
				offset += limit
			}
			// 输出获取到的用户
			for i, user := range users {
				log.Println(i, user.ID, user.Username, user.Position, user.Mobile)
			}
		}
	}
}

// 获取用户消息列表
func TestUserRepository_GetUserMessagesList(t *testing.T) {
	// 1. 先测试获取到用户
	userID := "1"
	offset := 0
	limit := 2
	haveNext := true

	db := datasource.DB
	r := NewUserRepository(db)

	if user, err := r.GetById(userID); err != nil {
		t.Error(err.Error())
	} else {
		// 2. 获取用户的消息
		for haveNext {
			if messages, err := r.GetUserMessagesList(user, offset, limit); err != nil {
				t.Error(err.Error())
				haveNext = false
			} else {
				// 判断是否还有下一页，以及修改offset
				if len(messages) == limit {
					haveNext = true
					offset += limit
				} else {
					haveNext = false
				}
				// 3. 打印出消息
				for _, message := range messages {
					log.Println(message.ID, message.MsgType, message.Success, message.Users)
					for _, u := range message.Users {
						log.Println(u.ID, u.DingID, u.Username)
					}
				}
			}

			if haveNext {
				log.Println("查找下一页：", offset, limit)
			}
		}

	}
}
