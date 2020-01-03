package sockets

import (
	"encoding/json"
	"fmt"
	"log"
	"sync"

	"github.com/codelieche/cronjob/backend/common"

	"github.com/codelieche/cronjob/backend/common/datasources"
)

var app *App

type App struct {
	etcd                 *datasources.Etcd
	clients              map[string]*Client // 客户端连接
	BroadcastMessageChan chan string        // 广播消息channel
	clientMux            *sync.RWMutex      // 客户端相关信息的读写锁
	messageChan          chan *Message      // 消息Channel
	closeChan            chan bool          // 关闭通道
}

// 不断的消费
func (app *App) ConsumeMessageLoop() {

	for {
		select {
		case message := <-app.messageChan:
			// 对消息反序列化
			messageEvent := MessageEvent{}
			if err := json.Unmarshal(message.Data, &messageEvent); err != nil {
				msg := fmt.Sprintf("收到消息：%s --> %s", message.RemoteAddr, message.Data)
				log.Println(msg)
			} else {
				// 根据对消息类型做相应的处理
				// {"category": "getJobs", "data": "0"}
				if messageEvent.Category == "getJobs" {
					app.clientMux.RLock()
					if client, isExist := app.clients[message.RemoteAddr]; isExist {
						// 发送任务信息给客户端
						go pushJobsToClient(client)
					} else {
						log.Println("客户端连接不存在：", message.RemoteAddr)
					}
					app.clientMux.RUnlock()
				}
			}

		case <-app.closeChan:
			log.Println("app已经关闭")
			goto END
		}
	}
END:
	log.Println("跳出App的ConsumeMessageLoop")

}

func (app *App) pushMessageEventToAllClients(category string, obj interface{}) (err error) {
	// 定义变量
	var (
		messageEvent *MessageEvent
		objData      []byte
		messageData  []byte
		client       *Client
	)

	if objData, err = json.Marshal(obj); err != nil {
		log.Println("json序列化出错：", err)
		return err
	}

	// 构造MessageEvent
	messageEvent = &MessageEvent{
		Category: category,
		Data:     string(objData),
	}

	// 发送数据
	messageData = common.PacketInterfaceData(messageEvent)

	// 发送数据
	for _, client = range app.clients {
		// 发送数据
		if err = client.SendMessage(1, messageData, false); err != nil {
			log.Printf("发送消息给%s出错:%s", client.RemoteAddr, err)
		} else {
			// 发送消息成功
		}
	}
	return nil
}

func initApp() {
	if app != nil {

	} else {
		app = &App{
			etcd:                 datasources.GetEtcd(),
			clients:              make(map[string]*Client),
			BroadcastMessageChan: make(chan string, 1024),
			clientMux:            &sync.RWMutex{},
			messageChan:          make(chan *Message, 500),
		}
		// 启动消息消息的协程
		go app.ConsumeMessageLoop()

		// 启动watch事件
		etcd := datasources.GetEtcd()
		watchJobs := &WatchJobsHandler{
			KeyDir: common.ETCD_JOBS_DIR,
			app:    app,
		}
		watchKill := &WatchKillHandler{
			KeyDir: common.ETCD_JOB_KILL_DIR,
			app:    app,
		}
		go etcd.WatchKeys(watchJobs.KeyDir, watchJobs)
		go etcd.WatchKeys(watchKill.KeyDir, watchKill)

	}
}

// 关闭
func Stop() {
	// 需要关闭socket了
	app.closeChan <- true
}
