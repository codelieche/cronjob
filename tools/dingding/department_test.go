package dingding

import (
	"log"
	"os"
	"testing"
)

// 获取部门列表
func TestDingDing_ListDepartment(t *testing.T) {

	ding := DingDing{
		AgentId:     "",
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
