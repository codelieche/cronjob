package worker

import (
	"encoding/json"
	"fmt"
	"log"

	"github.com/codelieche/cronjob/backend/master/sockets"

	"github.com/gorilla/websocket"
)

func recoverSocketClientError(socket *Socket) {
	//  捕获到异常
	if err := recover(); err != nil {
		log.Println("捕获到异常：", err)
		// 删除掉client
		socket.IsActive = false
		//socket.lock.Lock()
		//socket.lock.Unlock()

	}
}

// 读取客户端消息循环
func readLoop(socket *Socket) {
	// 捕获异常
	defer recoverSocketClientError(socket)

	// 不断的获取数据
	for {

		messageType, message, err := socket.conn.ReadMessage()

		//log.Println(messageType, message)
		if messageType == websocket.CloseMessage {
			log.Println("客户端要断开了")
			socket.IsActive = false
		} else {
			//log.Println(messageType)
		}

		if messageType != 1 {
			log.Println("消息类型不是1：", messageType)
			log.Println(string(message))
		}

		if err != nil {
			log.Println("读取消息出错：", err)
			//socket.IsActive = false
			//break
		}

		var event = &sockets.MessageEvent{}
		if err := json.Unmarshal(message, &event); err != nil {
			log.Println(err)
			msg := fmt.Sprintf("收到消息：%s", message)
			log.Println(msg)

		} else {
			// 判断消息类型，然后调用不通的处理器
			log.Println(event)
			switch event.Category {
			case "tryLock":
				// 尝试获取锁: {"category": "tryLock", "data":"{\"id\": 123, \"name\": \"jobs/default/abc\",\"secret\": \"123456\"}"}
				go tryLockEventHandler(event)
			case "leaseLock":
				// 释放获取到的锁: {"category": "leaseLock", "data":"{\"secret\": \"123456\", \"name\": \"jobs/default/abc\"}"}
				go leaseLockEventHandler(event)
			//case "releaseLock":
			//	// 释放获取到的锁: {"category": "releaseLock", "data":"jobs/default/abc"}
			//	go releaseLockEventHandler(event.Data, client)
			default:
				log.Println("我还暂时处理不了此类消息：", event)
			}
		}
	}
}

func readLoopDemo(conn *websocket.Conn) {
	// 不断的获取数据
	for {
		messageType, message, err := conn.ReadMessage()
		// messageType：
		//log.Println(messageType, message)
		if messageType == websocket.CloseMessage {
			log.Println("客户端要断开了")
		}
		if err != nil {
			log.Println("读取消息出错：", err)
			break
		}

		var event = &sockets.MessageEvent{}
		if err := json.Unmarshal(message, &event); err != nil {
			msg := fmt.Sprintf("收到消息：%s", message)
			log.Println(msg)

		} else {
			// 判断消息类型，然后调用不通的处理器
			log.Println(event)
		}

		// 如果想一次发送多个数据的话，需实例化个NextWriter
		//if w, err := conn.NextWriter(websocket.TextMessage); err != nil {
		//	log.Println(err)
		//} else {
		//	w.Write([]byte(msg))
		//	w.Write([]byte("<---"))
		//	w.Close()
		//}

	}
}
