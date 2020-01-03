package sockets

import (
	"log"
	"net/http"

	"github.com/gorilla/websocket"
	"github.com/kataras/iris/v12"
	"github.com/kataras/iris/v12/sessions"
)

// mvc websocket controller
type WebsocketController struct {
	Ctx     iris.Context
	Session sessions.Session
}

func (c *WebsocketController) Get(ctx iris.Context) {
	// 判断app是否为空
	if app == nil {
		initApp()
	}

	r := ctx.Request()
	w := ctx.ResponseWriter()

	upgrader := websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
		CheckOrigin: func(r *http.Request) bool {
			return true
		},
	}

	if conn, err := upgrader.Upgrade(w, r, nil); err != nil {
		log.Println(err)
	} else {
		//defer conn.Close()
		log.Println("websocket收到连接", conn.RemoteAddr())
		remoteAddr := conn.RemoteAddr().String()
		client := &Client{
			conn:       conn,
			RemoteAddr: remoteAddr,
			IsActive:   true,
			dataChan:   make(chan []byte, 100),
			closeChan:  make(chan bool, 5),
		}

		app.clients[remoteAddr] = client
		// 启动一个处理不断接收消息的协程
		go client.ReadeLoop()

		// 里面读取到消息就会把消息发送给app，交给app去处理
	}
}

// socket连接
func (c *WebsocketController) GetClient(ctx iris.Context) {
	ctx.ServeFile("./web/templates/socket.html", false)
}

// 查看当前系统中的锁
func (c *WebsocketController) GetClients(ctx iris.Context) {
	if app == nil {
		initApp()
	}

	ctx.JSON(app.clients)
}
