package sockets

import "log"

// 当程序要退出的时候，断开所有的连接
func Close() {
	log.Println("socket close()")

}
