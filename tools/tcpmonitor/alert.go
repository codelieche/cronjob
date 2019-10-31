package tcpmonitor

import "log"

// SendErrorMessage 发送错误消息
func (tcpMonitor *TCPMonitor) SendErrorMessage(message string) (success bool, err error) {
	log.Println(message)
	return true, nil
}

// SendRecoverMessage 发送恢复消息
// 开始出现了异常，后面恢复了，需发送恢复消息
func (tcpMonitor *TCPMonitor) SendRecoverMessage(message string) (success bool, err error) {
	log.Println(message)
	return true, nil
}
