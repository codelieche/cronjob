package sockets

import (
	"fmt"
	"log"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

func TestTryLock(t *testing.T) {

	//	1. 先连接服务端
	if conn, response, err := websocket.DefaultDialer.Dial("ws://127.0.0.1:9000/websocket", nil); err != nil {
		t.Error(err.Error())
	} else {
		log.Println(response)

		// 读取返回的数据
		go readLoopDemo(conn)

		i := 0
		for i < 200 {
			i++
			lockName := fmt.Sprintf("jobs/default/%d", i)
			messageEvent := MessageEvent{
				Category: "tryLock",
				Data:     lockName,
			}
			if err := conn.WriteJSON(messageEvent); err != nil {
				t.Error(err.Error())
			} else {
				// tryLock成功
				log.Println(lockName)
			}
		}
		conn.Close()
		time.Sleep(5 * time.Second)
		log.Println("===== Done =====")
	}
}

// 测试抢锁并续租
func TestTryLockAndLease(t *testing.T) {

	//	1. 先连接服务端
	if conn, response, err := websocket.DefaultDialer.Dial("ws://127.0.0.1:9000/websocket", nil); err != nil {
		t.Error(err.Error())
	} else {
		log.Println(response)

		// 读取返回的数据
		go readLoopDemo(conn)

		i := 0
		for i < 10 {
			i++
			lockName := fmt.Sprintf("jobs/default/%d", i)
			messageEvent := MessageEvent{
				Category: "tryLock",
				Data:     lockName,
			}
			if err := conn.WriteJSON(messageEvent); err != nil {
				t.Error(err.Error())
				return
			} else {
				// tryLock成功
				log.Println(lockName)
				// 开始执行续租
				x := 0
				for x < 10 {
					x++
					messageEvent = MessageEvent{
						Category: "leaseLock",
						Data:     lockName,
					}
					// 发送续租信息
					if err = conn.WriteJSON(messageEvent); err != nil {
						t.Error(err.Error())
						return
					} else {
						log.Printf("第%d次续租成功", x)
						time.Sleep(time.Second * 5)
					}
				}
			}
		}
		conn.Close()
		time.Sleep(5 * time.Second)
		log.Println("===== Done =====")
	}
}
