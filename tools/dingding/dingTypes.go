package dingding

// DingDing 注册应用后可获取到的信息
// 企业内部应用/小程序 --> 创建应用-->应用首页 --> 查看信息： 可看到相关字段
type DingDing struct {
	AgentId     int    `json:"agent_id"`     // 注册钉钉应用的时候的应用ID，发送工作通知消息的时候会用到agent_id
	AppKey      string `json:"app_key"`      // App Key：应用的唯一表示Key
	AppSecret   string `json:"app_secret"`   // App Secret：应用的秘钥
	AccessToken string `json:"access_token"` // Access Token
}

// DingDing Api Response
type ApiResponse struct {
	Errcode     int               `json:"errcode"`                // 错误代码，无错误代码是0
	Errmsg      string            `json:"errmsg"`                 // 错误消息
	AccessToken string            `json:"access_token,omitempty"` // Access Token
	Department  []*DingDepartment `json:"department,omitempty"`   // 部门列表
	UserList    []*DingUser       `json:"userlist,omitempty"`     // 用户列表
	TaskId      int64             `json:"task_id,omitempty"`      // 任务ID，比如：发送消息
	RequestID   string            `json:"request_id,omitempty"`   // 请求ID，比如：发送消息的响应
}

// DingDing Department
type DingDepartment struct {
	ID              int    `json:"id"`              // 部门ID
	Name            string `json:"name"`            // 部门名称
	ParentID        int    `json:"parentid"`        // 父部门ID
	CreateDeptGroup bool   `json:"createDeptGroup"` // 是否同步创建一个关联此部门的企业群
	AutoAddUser     bool   `json:"autoAddUser"`     // 当群创建好之后，是否有新人加入部门会自动加入该群
}

// DingDing User
type DingUser struct {
	UserId     string `json:"userid"`              // 员工在当前企业内的唯一标识，也称staffid，可由企业在创建时指定，并代表一定含义比如工号，创建后不可改变
	Unionid    string `json:"unionid"`             // 员工在当前开发者企业内的唯一表示，系统生成，固定值，不可改变
	Mobile     string `json:"mobile"`              // 手机号
	Tel        string `json:"tel"`                 // 分机号
	WorkPlace  string `json:"workPlace"`           //办公地点
	Remark     string `json:"remark"`              // 备注
	IsAdmin    bool   `json:"isAdmin"`             // 是否是企业的管理员
	IsBoss     bool   `json:"isBoss"`              // 是否未企业的老板
	IsHide     bool   `json:"isHide"`              // 是否隐藏号码
	IsLeader   bool   `json:"isLeader"`            // 是否未部门的主管
	Name       string `json:"name"`                // 成员名称
	Active     bool   `json:"active"`              // 表示该用户是否激活了钉钉
	Department []int  `json:"department"`          // 成员所属的部门ID列表
	Position   string `json:"position"`            // 职位信息
	Email      string `json:"email"`               // 员工的有些
	OrgEmail   string `json:"orgEmail, omitempty"` // 员工的企业邮箱，如果员工的企业邮箱没有开通，返回信息中不包含
	Avatar     string `json:"avatar"`              // 头像Url
	HiredDate  string `json:"hiredDate"`           // 入职时间
	StateCode  string `json:"stateCode"`           // 国家地区码
}

// DingDing Message
// 参考文档：https://ding-doc.dingtalk.com/doc#/serverapi2/ye8tup
type Message struct {
	MsgType  string       `json:"msgtype"`             // 消息类型
	Text     *TextMsg     `json:"text, omitempty"`     // msgType是text的消息内容
	Image    *ImageMsg    `json:"image, omitempty"`    // msgType是image的消息内容
	Markdown *MarkdownMsg `json:"markdown, omitempty"` // msgType是markdown的消息内容
}

// Text Message
type TextMsg struct {
	Content string `json:"content"` // 文本消息内容
}

// Markdown Message
type MarkdownMsg struct {
	Title string `json:"title"` // markdown的标题
	Text  string `json:"text"`  // 消息正文的内容
}

type ImageMsg struct {
	MediaId string `json:"media_id"` // 媒体文件Id，可以通过媒体文件接口上传图片获取
}

// 发送工作通知消息
type WorkerMessage struct {
	AgentID    int      `json:"agent_id"`               // 【必须】应用agentId
	UseridList string   `json:"userid_list,omitempty"`  // 接受者的用户userid列表，最大列表长度：100，逗号分隔
	DeptIdList string   `json:"dept_id_list,omitempty"` // 接受者的部门id列表，最大列表长度:20，接受者是部门id下(包括子部门)的所有用户
	ToAllUser  bool     `json:"to_all_user,omitempty"`  // 是否发送给企业全部用户
	Msg        *Message `json:"msg"`                    // 【必须】消息内容
}
