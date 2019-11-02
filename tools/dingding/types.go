package dingding

// DingDing 注册应用后可获取到的信息
// 企业内部应用/小程序 --> 创建应用-->应用首页 --> 查看信息： 可看到相关字段
type DingDing struct {
	AgentId     string `json:"agent_id"`     // 注册钉钉应用的时候的应用ID，发送工作通知消息的时候会用到agent_id
	AppKey      string `json:"app_key"`      // App Key：应用的唯一表示Key
	AppSecret   string `json:"app_secret"`   // App Secret：应用的秘钥
	AccessToken string `json:"access_token"` // Access Token
}

// DingDing Api Response
type ApiResponse struct {
	Errcode     int           `json:"errcode"`                // 错误代码，无错误代码是0
	Errmsg      string        `json:"errmsg"`                 // 错误消息
	AccessToken string        `json:"access_token,omitempty"` // Access Token
	Department  []*Department `json:"department,omitempty"`   // 部门列表
}

// DingDing Department
type Department struct {
	ID              int    `json:"id"`              // 部门ID
	Name            string `json:"name"`            // 部门名称
	ParentID        int    `json:"parentid"`        // 父部门ID
	CreateDeptGroup bool   `json:"createDeptGroup"` // 是否同步创建一个关联此部门的企业群
	AutoAddUser     bool   `json:"autoAddUser"`     // 当群创建好之后，是否有新人加入部门会自动加入该群
}
