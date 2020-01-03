package worker

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/codelieche/cronjob/backend/common/datamodels"

	"github.com/codelieche/cronjob/backend/master/sockets"

	"github.com/codelieche/cronjob/backend/common"
	"github.com/gorilla/websocket"
)

type Socket struct {
	lock      *sync.RWMutex   // 读写锁
	conn      *websocket.Conn // Socket的连接
	IsActive  bool            // 是否有效，断开的时候设置为false
	dataChan  chan []byte     // 接收数据的channel
	closeChan chan bool       // 关闭通道
}

var socket *Socket

// 接收消息
func (socket *Socket) ReadeLoop() {
	log.Printf("启动读取socket程序循环：%s --> %s", socket.conn.LocalAddr(), socket.conn.RemoteAddr())
	tmpBuffer := make([]byte, 0)
	go socket.consuleMessage(socket.dataChan)
	for {
		messageType, data, err := socket.conn.ReadMessage()
		//log.Println(messageType, data)
		//log.Printf("消息类型[%d]: %s", messageType, data)
		if err != nil {
			log.Println(err)
			if messageType == -1 {
				log.Println("messageType == -1：退出")
				break
			}
		}
		if len(data) > 0 && messageType == websocket.TextMessage {
			// 只对TextMessage类型的消息解包
			common.UnPacketData(append(tmpBuffer, data...), socket.dataChan)
		} else {
			log.Printf("消息为空, 或者消息格式不对：[%d] --> %s", messageType, data)
		}
	}
	// 连接断开了
	socket.closeChan <- true

	time.Sleep(time.Second)
	// 需要重新连接
	log.Println("开始尝试重新连接socket")
	connectMasterSocket(1)
}

// 把得到的消息，发送给app.messageChan，交给它处理
func (socket *Socket) consuleMessage(c <-chan []byte) {
	for {
		select {
		case data := <-c:
			//log.Println(data)
			//log.Printf("%s", data)
			// 对数据序列化
			messageEvent := &sockets.MessageEvent{}
			if err := json.Unmarshal(data, messageEvent); err != nil {
				msg := fmt.Sprintf("收到消息：%s", data)
				log.Println(msg)
			} else {
				// 对结果进行判断
				switch messageEvent.Category {
				case "jobEvent":
					// 处理job相关的事件
					data := []byte(messageEvent.Data)
					jobEvent := &datamodels.JobEvent{}
					if err := json.Unmarshal(data, jobEvent); err != nil {
						msg := fmt.Sprintf("jobEvent内容有误：%s", messageEvent.Data)
						log.Println(msg)
					} else {
						// 把事件加入到channel中
						app.Scheduler.jobEventChan <- jobEvent
					}
				default:
					log.Printf("%s", data)
					msg := fmt.Sprintf("worker暂时还处理不了类型为%s的事件", messageEvent.Category)
					log.Println(msg)
				}
			}

		case <-socket.closeChan:
			goto END
		}
	}
END:
	log.Println("连接断开了,结束消费消息：", socket.conn.RemoteAddr())
}

// 发送消息
// messageType: 消息类型
// data []byte: 发送小消息内容
// needPacket bool: 是否需要封装一下包，有时候可自行封装
func (socket *Socket) SendMessage(messageType int, data []byte, needPacket bool) (err error) {
	var message []byte
	// 对数据打包
	if messageType == 1 && needPacket {
		message = common.PacketData(data)
	} else {
		message = data
	}

	if err = socket.conn.WriteMessage(messageType, message); err != nil {
		log.Printf("发送消息给%s失败：%s", socket.conn.RemoteAddr(), err)
		return err
	} else {
		return nil
	}
}

// 发送消息
// messageType: 消息类型
// data []byte: 发送小消息内容
// needPacket bool: 是否需要封装一下包，有时候可自行封装
func (socket *Socket) SendMessageEventToMaster(category string, data string) (err error) {
	var (
		messageEvent *sockets.MessageEvent
	)

	messageEvent = &sockets.MessageEvent{
		Category: category,
		Data:     data,
	}

	messageEventData := common.PacketInterfaceData(messageEvent)

	if err = socket.SendMessage(1, messageEventData, false); err != nil {
		log.Println("发送消息失败：", messageEvent)
	}

	return err
}

// socket即将关闭的相关操作
func (socket *Socket) Stop() {
	socket.closeChan <- true
}

// 连接Master的Socket
func connectMasterSocket(times int) {
	// 1. 定义变量
	var (
		config          *common.Config
		masterSocketUrl string
		conn            *websocket.Conn
		response        *http.Response
		err             error
	)

	// 2. 获取变量
	if !app.IsActive {
		log.Println("当前socket状态已经是false了，无需再次重连")
		return
	}
	config = common.GetConfig()
	if masterSocketUrl, err = config.Worker.GetSocketUrl(); err != nil {
		log.Println("获取socket的url出错：", err.Error())
		os.Exit(1)
	}

	// 3. 连接socket
	log.Println(masterSocketUrl)
	if conn, response, err = websocket.DefaultDialer.Dial(masterSocketUrl, nil); err != nil {
		log.Printf("第%d次连接socket出错：%s", times, err)
		if times < 10 {
			sleepSecond := times * 5
			log.Printf("%d秒后重试\n", sleepSecond)
			time.Sleep(time.Second * time.Duration(times*5))
			connectMasterSocket(times + 1)
		} else {
			os.Exit(1)
		}
	} else {
		// log.Println(response)
		response = response
		// 连接成功

		// 4. 实例化socket
		socket = &Socket{
			conn:      conn,
			lock:      &sync.RWMutex{},
			IsActive:  true,
			dataChan:  make(chan []byte, 100),
			closeChan: make(chan bool, 5),
		}
		app.socket = socket
		// 读取socket的消息
		go socket.ReadeLoop()
		// socket发送getEvent的消息
		time.Sleep(time.Second)
		go socket.SendMessageEventToMaster("getJobs", `{"category": "getJobs", "data": "0"}`)
	}

}
