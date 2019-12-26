package sockets

import (
	"context"
	"fmt"
	"log"

	"github.com/kataras/iris/v12/websocket"
)

// etcdLock处理函数
func etcdLockEventHandler(nsConn *websocket.NSConn, msg websocket.Message) error {
	ctx := websocket.GetContext(nsConn.Conn)
	log.Printf("收到message消息: %s from [%s]-[%s]", msg.Body, nsConn.Conn.ID(), ctx.RemoteAddr())
	//log.Println(msg)
	data := []byte(fmt.Sprintf("我收到了消息:%s", msg.Body))
	nsConn.Ask(context.Background(), "message", data)
	return nil
}
