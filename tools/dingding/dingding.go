package dingding

import "os"

func NewDing() (ding *DingDing) {

	ding = &DingDing{
		AgentId:     270677909,
		AppKey:      os.Getenv("DINGDING_APP_KEY"),
		AppSecret:   os.Getenv("DINGDING_APP_SECRET"),
		AccessToken: "",
	}
	return ding
}
