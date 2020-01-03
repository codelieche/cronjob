package sockets

import (
	"log"
	"time"

	"github.com/codelieche/cronjob/backend/common"
	"github.com/gorilla/websocket"
)

// 连接的客户端
type Client struct {
	conn       *websocket.Conn `json:"-"`           // 连接
	RemoteAddr string          `json:"remote_addr"` // 远端的地址
	IsActive   bool            `json:"is_active"`   // 是否是有效的，断开的时候需要设置为false
	dataChan   chan []byte     `json:"-"`           // 消息的channel
	closeChan  chan bool       `json:"-"`           // 判断连接是否端口
}

// 接收消息
func (client *Client) ReadeLoop() {

	tmpBuffer := make([]byte, 0)
	go client.consuleMessage(client.dataChan)
	for {
		messageType, data, err := client.conn.ReadMessage()
		if err != nil {
			log.Println(err)
			if messageType == -1 {
				log.Println("messageType == -1：退出")
				break
			}
		}
		if len(data) > 0 && messageType == websocket.TextMessage {
			// 只对TextMessage类型的消息解包
			common.UnPacketData(append(tmpBuffer, data...), client.dataChan)
		} else {
			log.Printf("消息为空, 或者消息格式不对：[%d] --> %s", messageType, data)
		}
	}
	// 连接断开了
	client.closeChan <- true

	time.Sleep(time.Second)
	// 把连接从clients中删除
	app.clientMux.Lock()
	if _, isExist := app.clients[client.RemoteAddr]; isExist {
		//log.Printf("%s 断开连接了，删除\n", client.RemoteAddr)
		delete(app.clients, client.RemoteAddr)
	} else {
		log.Println("连接不存在：", client.RemoteAddr)
	}
	app.clientMux.Unlock()
}

// 把得到的消息，发送给app.messageChan，交给它处理
func (client *Client) consuleMessage(c <-chan []byte) {
	for {
		select {
		case data := <-c:
			//log.Println(data)
			//log.Printf("%s", data)
			// 实例化消息
			message := &Message{
				RemoteAddr: client.RemoteAddr,
				Data:       data,
			}
			app.messageChan <- message
		case <-client.closeChan:
			goto END
		}
	}
END:
	log.Println("连接断开了,结束消费消息：", client.RemoteAddr)
}

// 发送消息
// messageType: 消息类型
// data []byte: 发送小消息内容
// needPacket bool: 是否需要封装一下包，有时候可自行封装
func (client *Client) SendMessage(messageType int, data []byte, needPacket bool) (err error) {
	var message []byte
	// 对数据打包
	if messageType == 1 && needPacket {
		message = common.PacketData(data)
	} else {
		message = data
	}

	if err = client.conn.WriteMessage(messageType, message); err != nil {
		log.Printf("发送消息给%s失败：%s", client.RemoteAddr, err)
		return err
	} else {
		return nil
	}
}
