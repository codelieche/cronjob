package dingding

import (
	"os"
	"testing"
)

func TestGetAccessToken(t *testing.T) {

	ding := DingDing{
		AgentId:     "",
		AppKey:      os.Getenv("DINGDING_APP_KEY"),
		AppSecret:   os.Getenv("DINGDING_APP_SECRET"),
		AccessToken: "",
	}
	ding.GetAccessToken()
}
