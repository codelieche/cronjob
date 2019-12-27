package worker

import (
	"log"
	"net/http"
	"os"
	"sync"

	"github.com/codelieche/cronjob/backend/common"
	"github.com/gorilla/websocket"
)

type Socket struct {
	lock       *sync.RWMutex    // 读写锁
	RequestID  int              // 请求序号
	jobLockMap map[int]*JobLock // JobLock
	conn       *websocket.Conn  // Socket的连接
	IsActive   bool             // 是否有效，断开的时候设置为false
}

var socket *Socket

func connectMasterSocket() {
	// 1. 定义变量
	var (
		config          *common.Config
		masterSocketUrl string
		conn            *websocket.Conn
		response        *http.Response
		err             error
	)

	// 2. 获取变量
	config = common.GetConfig()
	if masterSocketUrl, err = config.Worker.GetSocketUrl(); err != nil {
		log.Println("获取socket的url出错：", err.Error())
		os.Exit(1)
	}

	// 3. 连接socket
	log.Println(masterSocketUrl)
	if conn, response, err = websocket.DefaultDialer.Dial(masterSocketUrl, nil); err != nil {
		log.Println("连接socket出错：", err)
		os.Exit(1)
	} else {
		log.Println(response)
		// 连接成功
	}

	// 4. 实例化socket
	socket = &Socket{
		conn:       conn,
		RequestID:  0,
		lock:       &sync.RWMutex{},
		IsActive:   true,
		jobLockMap: make(map[int]*JobLock),
	}
	go readLoop(socket)
	app.socket = socket

}
