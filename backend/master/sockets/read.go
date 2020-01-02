package sockets

import (
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/codelieche/cronjob/backend/common"

	"github.com/gorilla/websocket"
)

func recoverSocketClientError(client *Client) {
	//  捕获到异常
	if err := recover(); err != nil {
		log.Println("捕获到异常：", err)
		// 删除掉client
		client.IsActive = false
		app.clientMux.Lock()
		defer app.clientMux.Unlock()
		if _, isExist := app.clients[client.RemoteAddr]; isExist {
			log.Println("删除客户端信息：", client.RemoteAddr)
			delete(app.clients, client.RemoteAddr)
		}
	}
}

// 读取客户端消息循环
func readLoop(client *Client) {
	// 捕获异常
	defer recoverSocketClientError(client)

	// 不断的获取数据
	for {
		messageType, message, err := client.conn.ReadMessage()
		// messageType：
		log.Println(messageType, string(message))
		if messageType == websocket.CloseMessage {
			log.Println("客户端要断开了")
			client.IsActive = false
		}
		if err != nil {
			log.Println("读取消息出错：", err)
			client.IsActive = false
			break
		}

		var event = &MessageEvent{}
		//d, _ := json.Marshal(event)
		//log.Println(string(d))
		if err := json.Unmarshal(message, &event); err != nil {
			log.Println(err)
			msg := fmt.Sprintf("收到消息：%s", message)
			log.Println(msg)
			err = client.conn.WriteMessage(messageType, []byte(msg))
			if err != nil {
				log.Println("发送消息失败：", err)
				client.IsActive = false
				break
			}

		} else {
			// 判断消息类型，然后调用不通的处理器
			//log.Println(event)
			switch event.Category {
			case "tryLock":
				// 尝试获取锁: {"category": "tryLock", "data":"{\"id\": 123, \"name\": \"jobs/default/abc\",\"secret\": \"123456\"}"}
				go tryLockEventHandler(event, client)
			case "leaseLock":
				// 释放获取到的锁: {"category": "leaseLock", "data":"{\"secret\": \"123456\", \"name\": \"jobs/default/abc\"}"}
				go leaseLockEventHandler(event, client)
			case "releaseLock":
				// 释放获取到的锁: {"category": "releaseLock", "data":"jobs/default/abc"}
				go releaseLockEventHandler(event.Data, client)
			default:
				log.Println("我还暂时处理不了此类消息：", event)
			}
		}
	}
}

func readeLoop02(client *Client) {

	tmpBuffer := make([]byte, 0)
	messageChan := make(chan []byte, 100)
	go consuleMessage(messageChan)
	for {
		messageType, data, err := client.conn.ReadMessage()
		if err != nil {
			log.Println(err)
			if messageType == -1 {
				log.Println("messageType == -1：退出")
				break
			}
		}
		if len(data) > 0 {
			common.UnPacketData(append(tmpBuffer, data...), messageChan)
		} else {
			log.Println("消息为空：", messageType, data)
		}
	}
}

func consuleMessage(c <-chan []byte) {

	for {
		select {
		case message := <-c:
			log.Println(message)
			log.Printf("%s", message)
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

		var event = &MessageEvent{}
		if err := json.Unmarshal(message, &event); err != nil {
			msg := fmt.Sprintf("收到消息：%s", message)
			log.Println(msg)

		} else {
			// 判断消息类型，然后调用不通的处理器
			log.Println(event)
		}

		conn.SetReadDeadline(time.Now())

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
