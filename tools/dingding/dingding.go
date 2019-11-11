package dingding

import (
	"log"
	"os"
)

func NewDing() (ding *DingDing) {
	var (
	//agentID int
	//err     error
	)

	//agentIDstr := os.Getenv("DINGDING_AGET_ID")
	//if agentID, err = strconv.Atoi(agentIDstr); err != nil {
	//	log.Println("请设置DINGDING_AGET_ID环境变量")
	//	os.Exit(1)
	//}

	if config.DingDing.AgentID <= 0 {
		log.Println("DingDing App Agent ID为空")
		os.Exit(1)
	}

	ding = &DingDing{
		AgentId:     config.DingDing.AgentID,
		AppKey:      config.DingDing.AppKey,
		AppSecret:   config.DingDing.SecretKey,
		AccessToken: "",
	}

	return ding
}
