package main

import (
	"log"

	"github.com/codelieche/cronjob/tools/tcpmonitor"
)

func init() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
}

// tcpMonitor的程序入口
func main() {
	log.Println("start tcp monitor.")

	tcpMonitor := tcpmonitor.TCPMonitor{
		Name:     "测试监控程序",
		Hosts:    []string{"127.0.0.1"},
		Port:     8080,
		Times:    2,
		Interval: 30,
		Timemout: 5,
		Phones:   []string{},
		Emails:   []string{},
		Users:    []string{"admin"},
		Status:   "Start",
	}

	// 执行监控程序
	tcpMonitor.ExecuteMonitorLoop()
}
