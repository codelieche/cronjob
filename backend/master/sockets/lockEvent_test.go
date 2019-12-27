package sockets

import (
	"encoding/json"
	"fmt"
	"log"
	"testing"
	"time"

	"github.com/codelieche/cronjob/backend/common/datamodels"

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
			lockReqest := datamodels.LockRequest{
				ID:     i,
				Name:   lockName,
				Secret: "",
			}
			messageEvent := MessageEvent{
				Category: "tryLock",
				Data:     "",
			}
			if data, err := json.Marshal(lockReqest); err != nil {

			} else {
				messageEvent.Data = string(data)
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

			lockReqest := datamodels.LockRequest{
				ID:     i,
				Name:   lockName,
				Secret: "123456",
			}
			messageEvent := MessageEvent{
				Category: "tryLock",
				Data:     "",
			}
			if data, err := json.Marshal(lockReqest); err != nil {

			} else {
				messageEvent.Data = string(data)
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
						Data:     "",
					}
					lockReqest2 := datamodels.LockRequest{
						Name:   lockName,
						Secret: lockReqest.Secret,
					}
					if data, err := json.Marshal(lockReqest2); err != nil {

					} else {
						messageEvent.Data = string(data)
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

// 测试抢锁并续租和释放
func TestTryLockAndReLease(t *testing.T) {

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
			lockReqest := datamodels.LockRequest{
				ID:     i,
				Name:   lockName,
				Secret: "",
			}

			messageEvent := MessageEvent{
				Category: "tryLock",
				Data:     "",
			}
			if data, err := json.Marshal(lockReqest); err != nil {

			} else {
				messageEvent.Data = string(data)
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
					if x == i {
						log.Println("执行释放")
						messageEvent = MessageEvent{
							Category: "releaseLock",
							Data:     lockName,
						}

						// 发送释放信息
						if err = conn.WriteJSON(messageEvent); err != nil {
							t.Error(err.Error())
							return
						} else {
							log.Println("发送释放信息成功", lockName)
							break
						}
					}
					x++

					messageEvent = MessageEvent{
						Category: "leaseLock",
						Data:     lockName,
					}
					lockReqest2 := datamodels.LockRequest{
						Name:   lockName,
						Secret: lockReqest.Secret,
					}
					if data, err := json.Marshal(lockReqest2); err != nil {

					} else {
						messageEvent.Data = string(data)
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
