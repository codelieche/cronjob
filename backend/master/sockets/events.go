package sockets

import (
	"log"

	"github.com/kataras/iris/v12/websocket"
)

var NameSpace = "default"

// WebSocket的Events
var serverEvents = websocket.Namespaces{
	NameSpace: websocket.Events{
		websocket.OnNamespaceConnected: func(nsConn *websocket.NSConn, msg websocket.Message) error {
			// 当socket连接到这个命名空间的时候
			ctx := websocket.GetContext(nsConn.Conn)
			log.Printf("新的连接：[%s-%s], IP地址为：%s\n", msg.Namespace, nsConn.Conn.ID(), ctx.RemoteAddr())

			return nil
		},
		websocket.OnNamespaceDisconnect: func(nsConn *websocket.NSConn, msg websocket.Message) error {
			// 断开连接
			ctx := websocket.GetContext(nsConn.Conn)
			log.Printf("断开连接：[%s-%s], IP地址为：%s\n", msg.Namespace, nsConn.Conn.ID(), ctx.RemoteAddr())
			return nil
		},
		// 自定义的事件：c.Emit("事件类型", "发送的数据")
		"message":   messageEventHandler,
		"createJob": createJobEventHandler,
		"etcdLock":  etcdLockEventHandler,
	},
}
