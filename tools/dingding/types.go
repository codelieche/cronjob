package dingding

// DingDing 注册应用后可获取到的信息
type DingDing struct {
	AgentID     string `json:"agent_id"`     // 注册钉钉应用的时候的应用ID，发送工作通知消息的时候会用到
	AppKey      string `json:"app_key"`      // App Key
	AppSecret   string `json:"app_secret"`   // App Secret
	AccessToken string `json:"access_token"` // Access Token
}
