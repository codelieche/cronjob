package app

import (
	"log"

	"github.com/codelieche/cronjob/backend/master/sockets"
)

// 处理control/CMD + c关闭的时候
func handleAppOnInterput() {
	log.Println("程序即将退出")
	// websocket关闭
	sockets.Close()

	// 关闭数据库连接等

	// 关闭session的redis数据库

}
