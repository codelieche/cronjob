package dingding

// DingDing 注册应用后可获取到的信息
// 企业内部应用/小程序 --> 创建应用-->应用首页 --> 查看信息： 可看到相关字段
type DingDing struct {
	AgentId     string `json:"agent_id"`     // 注册钉钉应用的时候的应用ID，发送工作通知消息的时候会用到agent_id
	AppKey      string `json:"app_key"`      // App Key：应用的唯一表示Key
	AppSecret   string `json:"app_secret"`   // App Secret：应用的秘钥
	AccessToken string `json:"access_token"` // Access Token
}
