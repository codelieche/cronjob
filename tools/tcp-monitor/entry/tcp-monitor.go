package main

import (
	"log"

	tcp_monitor "github.com/codelieche/cronjob/tools/tcp-monitor"
)

func init() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
}

func main() {
	log.Println("start tcp monitor.")

	tcpMonitor := tcp_monitor.TcpMonitor{
		Name:     "测试监控程序",
		Hosts:    []string{"127.0.0.1"},
		Port:     8080,
		Times:    2,
		Duration: 30,
		Timemout: 5,
		Users:    []string{"admin"},
		Status:   "Start",
	}

	// 执行监控程序
	tcpMonitor.ExecuteMonitorLoop()
}
