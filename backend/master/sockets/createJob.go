package sockets

import (
	"context"
	"fmt"
	"log"

	"github.com/kataras/iris/v12/websocket"
)

// createJob处理函数
func createJobEventHandler(nsConn *websocket.NSConn, msg websocket.Message) error {
	ctx := websocket.GetContext(nsConn.Conn)
	log.Printf("收到createJob的消息: %s from [%s]-[%s]", msg.Body, nsConn.Conn.ID(), ctx.RemoteAddr())
	//log.Println(msg)
	data := []byte(fmt.Sprintf("我收到了消息:%s", msg.Body))
	nsConn.Ask(context.Background(), "message", data)
	//nsConn.Conn.Write(msg)
	return nil
}
