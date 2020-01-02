package sockets

import (
	"fmt"
	"log"
	"testing"
	"time"

	"github.com/codelieche/cronjob/backend/common"
	"github.com/gorilla/websocket"
)

func Test01(t *testing.T) {
	//	1. 先连接服务端
	if conn, response, err := websocket.DefaultDialer.Dial("ws://127.0.0.1:9000/websocket", nil); err != nil {
		t.Error(err.Error())
	} else {
		log.Println(response)
		i := 0
		for i < 10 {
			i++
			msg := fmt.Sprintf("[这个是测试消息%d]", i)
			if err := conn.WriteMessage(websocket.TextMessage, common.PacketData([]byte(msg))); err != nil {
				t.Error(err)
				break
			} else {
				log.Println("发送消息完毕")
				time.Sleep(time.Duration(i) * time.Second)
			}
		}
	}
}
