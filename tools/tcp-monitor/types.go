package tcp_monitor

type TcpMonitor struct {
	Name     string   `json:"name"`     // 这个服务的名称
	Hosts    []string `json:"host"`     // 主机地址:以逗号分隔,\;分隔
	Port     int      `json:"port"`     // 服务端口
	Times    int      `json:"times"`    // 成功或者失败检查次数：默认2次
	Duration int      `json:"duration"` // 间隔秒数：默认30秒
	Timemout int      `json:"timemout"` // 超时时间秒数：默认5秒
	Users    []string `json:"users"`    // 告警通知人员
	Status   string   `json:"status"`   // 当前状态: Ready, Running, IsError, IsSuccess, Done
}

type tcpMonitorInfo struct {
	address string `json:"address"` // tcp监控的地址
}
