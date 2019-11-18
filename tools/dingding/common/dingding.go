package common

import (
	"log"
	"os"

	"github.com/codelieche/cronjob/tools/dingding/settings"
)

var config *settings.Config

// DingDing 注册应用后可获取到的信息
// 企业内部应用/小程序 --> 创建应用-->应用首页 --> 查看信息： 可看到相关字段
type DingDing struct {
	AgentId     int    `json:"agent_id"`     // 注册钉钉应用的时候的应用ID，发送工作通知消息的时候会用到agent_id
	AppKey      string `json:"app_key"`      // App Key：应用的唯一表示Key
	AppSecret   string `json:"app_secret"`   // App Secret：应用的秘钥
	AccessToken string `json:"access_token"` // Access Token
}

func NewDing() (ding *DingDing) {
	var (
	//agentID int
	//err     error
	)
	if config == nil {
		if config == nil {
			if err := settings.ParseConfig(); err != nil {
				log.Println(err.Error())
				os.Exit(1)
			}
			config = settings.GetConfig()
		}
	}

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
