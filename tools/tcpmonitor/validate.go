package tcpmonitor

import "errors"

// 对TCPMonitor需要传递的基本参数做校验
func (tcpMonitor *TCPMonitor) validate() (err error) {
	// 对hosts校验
	if len(tcpMonitor.Hosts) < 1 {
		err = errors.New("hosts不可为空")
		return
	}

	// 对端口号做校验
	if tcpMonitor.Port <= 0 {
		err = errors.New("port不可为0")
		return
	}

	// 对部分值做校验
	if tcpMonitor.Interval <= 0 {
		// 默认是30秒的间隔
		tcpMonitor.Interval = 30
	}
	if tcpMonitor.Timemout <= 0 {
		// 尝试连接的超时时间
		tcpMonitor.Timemout = 5
	}
	if tcpMonitor.Times <= 0 {
		// 设置检查：成功/失败的次数
		tcpMonitor.Times = 2
	}

	// 对users/phones/emails做校验
	if len(tcpMonitor.Users) == 0 && len(tcpMonitor.Emails) == 0 && len(tcpMonitor.Phones) == 0 {
		err = errors.New("请传入接收告警信息的：users/photos/emails")
		return
	}

	return nil
}
