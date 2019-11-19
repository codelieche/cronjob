package common

import (
	"log"
	"os"
	"testing"
)

// 获取部门列表
func TestDingDing_ListDepartment(t *testing.T) {

	ding := DingDing{
		AgentId:     0,
		AppKey:      os.Getenv("DINGDING_APP_KEY"),
		AppSecret:   os.Getenv("DINGDING_APP_SECRET"),
		AccessToken: "",
	}

	if departments, err := ding.ListDepartment(); err != nil {
		t.Error(err.Error())
		return
	} else {
		log.Println("获取到部门的长度是：", len(departments))
		for _, department := range departments {
			log.Println(department)
		}
	}
}

func TestDingDing_GetDepartmentUserList(t *testing.T) {
	ding := DingDing{
		AgentId:     0,
		AppKey:      os.Getenv("DINGDING_APP_KEY"),
		AppSecret:   os.Getenv("DINGDING_APP_SECRET"),
		AccessToken: "",
	}
	//departmentId := 118434421
	if departments, err := ding.ListDepartment(); err != nil {
		t.Error(err.Error())
		return
	} else {
		log.Println("获取到部门的长度是：", len(departments))
		for _, department := range departments {
			log.Println(department)
			//	获取部门用户
			if userList, err := ding.GetDepartmentUserList(department.ID, 0, 10); err != nil {
				t.Error(err.Error())
				return
			} else {
				log.Printf("部门：%s，获取到用户%d个\n", department.Name, len(userList))
				for _, user := range userList {
					log.Println(user)
				}
			}
		}
	}

}
