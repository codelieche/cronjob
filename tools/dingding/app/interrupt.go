package app

import "log"

// 处理control/cmd + c关闭的时候
func handleAppInterupt() {
	log.Println("程序即将退出！")
}
