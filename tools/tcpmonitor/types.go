// tcpmonitor包相关的struct

package tcpmonitor

import "sync"

// TCPMonitor Struct
type TCPMonitor struct {
	Name     string   `json:"name"`     // 这个服务的名称
	Hosts    []string `json:"host"`     // 主机地址:以逗号分隔,\;分隔
	Port     int      `json:"port"`     // 服务端口
	Times    int      `json:"times"`    // 成功或者失败检查次数：默认2次
	Interval int      `json:"interval"` // 间隔秒数：默认30秒
	Timemout int      `json:"timemout"` // 超时时间秒数：默认5秒
	Emails   []string `json:"emails"`   // 邮件告警
	Phones   []string `json:"phones"`   // 电话告警
	Users    []string `json:"users"`    // 告警通知人员: 通过wechart、DingDing等发送告警消息
	Status   string   `json:"status"`   // 当前状态: Ready, Running, IsError, IsSuccess, Done
	Count    int64    `json:"count"`    // 总共执行次数
	// 全局会用到的变量
	monitorSuccessCount int            // 监控成功的主机数
	lock                sync.RWMutex   // 读写锁
	wg                  sync.WaitGroup // sync WaitGroup
}

// monitorExecuteInfo tcp监控执行时候的信息
type monitorExecuteInfo struct {
	address                string // tcp监控的地址
	count                  int    // 总共执行尝试连接的次数
	successCount           int    // 执行成功的次数
	errorCount             int    // 执行错误的次数
	needSendErrorMessage   bool   // 是否需要发送错误信息
	errorMessageSended     bool   // 错误消息是否已经发送
	needSendRecoverMessage bool   // 是否需要发送恢复消息
	recoverMessageSended   bool   // 恢复消息是否已经发送
}
