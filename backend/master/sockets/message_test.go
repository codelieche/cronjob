package sockets

import (
	"fmt"
	"log"
	"testing"
	"time"

	"github.com/codelieche/cronjob/backend/common"

	"github.com/gorilla/websocket"
)

func TestSendMessage(t *testing.T) {
	if conn, response, err := websocket.DefaultDialer.Dial("ws://127.0.0.1:9000/websocket", nil); err != nil {
		t.Error(err.Error())
	} else {
		log.Println(response)
		i := 0
		for i < 100 {
			i++
			msg := fmt.Sprintf("test message %d", i)
			if err = conn.WriteMessage(1, []byte(msg)); err != nil {
				t.Error(err)
				break
			}
		}
		time.Sleep(time.Second)
		log.Println("Done")
	}
}

func TestSendJsonMessage(t *testing.T) {
	if conn, response, err := websocket.DefaultDialer.Dial("ws://127.0.0.1:9000/websocket", nil); err != nil {
		t.Error(err.Error())
	} else {
		log.Println(response)
		i := 0
		for i < 100 {
			i++
			msg := fmt.Sprintf("test message %d", i)
			data := map[string]interface{}{
				"category": "message",
				"data":     msg,
			}
			if err = conn.WriteJSON(data); err != nil {
				t.Error(err)
				break
			}
		}
		time.Sleep(time.Second)
		log.Println("Done")
	}
}

func TestSendSocketMessageToMaster(t *testing.T) {
	//	1. 先连接服务端
	if conn, response, err := websocket.DefaultDialer.Dial("ws://127.0.0.1:9000/websocket", nil); err != nil {
		t.Error(err.Error())
	} else {
		log.Println(response)
		go func() {
			log.Println("===== 读取消息 =====")
			for {
				messageType, data, err := conn.ReadMessage()
				if err != nil {
					log.Println(err)
					break
				} else {
					//log.Println(messageType, data)
					log.Printf("[%d]: %s", messageType, data)
				}
			}
			log.Println("跳出读取消息的循环")
		}()

		i := 0
		for i < 10 {
			i++
			msg := fmt.Sprintf("[这个是测试消息%d]", i)
			if err := conn.WriteMessage(websocket.TextMessage, common.PacketData([]byte(msg))); err != nil {
				t.Error(err)
				break
			} else {
				log.Println("发送消息完毕：", msg)
				time.Sleep(time.Duration(i) * time.Second)
			}
		}
	}
}
