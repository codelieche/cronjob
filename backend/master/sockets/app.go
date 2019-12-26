package sockets

import (
	"sync"

	"github.com/codelieche/cronjob/backend/common/datamodels"
	"github.com/codelieche/cronjob/backend/common/datasources"
	"github.com/gorilla/websocket"
)

var app *App

// 连接的客户端
type Client struct {
	conn       *websocket.Conn
	RemoteAddr string // 远端的地址
	IsActive   bool   // 是否是有效的，断开的时候需要设置为false
}

type App struct {
	etcd                 *datasources.Etcd
	clients              map[string]*Client              // 客户端连接
	BroadcastMessageChan chan string                     // 广播消息channel
	etcdLocksMap         map[string]*datamodels.EtcdLock // etcd锁
	opEtcdLockMux        *sync.RWMutex                   // 读写锁
	clientMux            *sync.RWMutex                   // 客户端相关信息的读写锁
}

// 消息事件
// 通过消息来判断事件的类型，比如:message, createJob, jobExecute, tryLock, leaseLock, releaseLock,
type MessageEvent struct {
	Category string `json:"category"` // 消息分类
	Data     string `json:"data"`     // 数据
}

func initApp() {
	if app != nil {

	} else {
		app = &App{
			etcd:                 datasources.GetEtcd(),
			clients:              make(map[string]*Client),
			BroadcastMessageChan: make(chan string, 1024),
			etcdLocksMap:         make(map[string]*datamodels.EtcdLock),
			opEtcdLockMux:        &sync.RWMutex{},
			clientMux:            &sync.RWMutex{},
		}
	}
}
