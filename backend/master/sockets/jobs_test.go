package sockets

import (
	"log"
	"testing"
	"time"

	"github.com/codelieche/cronjob/backend/common"

	"github.com/gorilla/websocket"
)

// 连接socket并获取jobs

func TestClient_PushJobs(t *testing.T) {
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
					// 对消息解包
				}
			}
			log.Println("跳出读取消息的循环")
		}()

		// 发送获取getJobs
		messageEvent := MessageEvent{
			Category: "getJobs",
			Data:     "0",
		}
		data := common.PacketInterfaceData(messageEvent)

		if err := conn.WriteMessage(1, data); err != nil {
			log.Println(err)
			t.Error(err)
			return
		}

		time.Sleep(time.Second)
		log.Println("睡眠1一分钟")
		time.Sleep(time.Minute)
		log.Println("Done")
	}
}

func TestSocketWatchEvent(t *testing.T) {
	// 连接客户端
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
					// 对消息解包
				}
			}
			log.Println("跳出读取消息的循环")
		}()
		log.Println("延时5分钟后退出")
		time.Sleep(time.Minute * 5)
		log.Println("=== Done ===")
	}
}
