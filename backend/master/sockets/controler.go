package sockets

import (
	"fmt"
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
		clientStr := fmt.Sprintf("%s", conn.RemoteAddr())
		client := &Client{
			conn:       conn,
			RemoteAddr: conn.RemoteAddr().String(),
			IsActive:   true,
		}

		app.clients[clientStr] = client
		// 启动一个处理不断接收消息的协程
		go readLoop(client)
	}
}

// socket连接
func (c *WebsocketController) GetClient(ctx iris.Context) {
	ctx.ServeFile("./web/templates/socket.html", false)
}

// 查看当前系统中的锁
func (c *WebsocketController) GetEtcdlockList(ctx iris.Context) {
	if app == nil {
		initApp()
	}

	ctx.JSON(app.etcdLocksMap)
}
