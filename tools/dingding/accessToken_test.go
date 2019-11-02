package dingding

import (
	"log"
	"os"
	"testing"
)

// 测试获取Access Token
func TestGetAccessToken(t *testing.T) {

	ding := DingDing{
		AgentId:     "",
		AppKey:      os.Getenv("DINGDING_APP_KEY"),
		AppSecret:   os.Getenv("DINGDING_APP_SECRET"),
		AccessToken: "",
	}

	if token, err := ding.GetAccessToken(); err != nil {
		t.Error(err.Error())
	} else {
		log.Println("Token:", token)
	}
}
