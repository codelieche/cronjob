package main

import (
	"context"
	"fmt"
	"log"

	"github.com/kataras/iris/v12"
	"github.com/kataras/iris/v12/websocket"
)

// 缺点：当client发送消息太快的时候，服务端会丢失数据
func main() {
	app := iris.New()

	log.SetFlags(log.Lshortfile | log.LstdFlags)

	clients := make(map[string]*websocket.Conn)

	app.Get("/", func(ctx iris.Context) {
		ctx.Text("Hell Index Page")
	})

	app.Get("/client", func(ctx iris.Context) {
		//log.Println(os.Getenv("PWD"))
		ctx.ServeFile("./socket.html", false)
	})

	app.Get("/info", func(ctx iris.Context) {
		data := make(map[string]interface{})
		data["url"] = "info"
		data["clients"] = clients
		ctx.JSON(data)
	})

	// 设置socket
	serverEvents := websocket.Namespaces{
		"default": websocket.Events{
			websocket.OnNamespaceConnected: func(nsConn *websocket.NSConn, msg websocket.Message) error {
				// with `websocket.GetContext` you can retrieve the Iris' `Context`.
				ctx := websocket.GetContext(nsConn.Conn)

				log.Printf("[%s] 连接成功 [%s-%s] with IP [%s]",
					nsConn, msg.Namespace, nsConn.Conn.ID(),
					ctx.RemoteAddr())
				return nil
			},
			websocket.OnNamespaceDisconnect: func(nsConn *websocket.NSConn, msg websocket.Message) error {
				log.Printf("[%s] 断开连接 [%s-%s]", nsConn, msg.Namespace, nsConn.Conn.ID())
				return nil
			},
			"message": func(nsConn *websocket.NSConn, msg websocket.Message) error {
				ctx := websocket.GetContext(nsConn.Conn)
				log.Printf("收到message消息: %s from [%s]-[%s]", msg.Body, nsConn.Conn.ID(), ctx.RemoteAddr())
				//log.Println(msg)
				nsConn.Conn.Server().Broadcast(nsConn, msg)
				data := []byte(fmt.Sprintf("我收到了消息:%s", msg.Body))
				nsConn.Ask(context.Background(), "message", data)
				//nsConn.Conn.Write(msg)
				return nil
			},

			"createJob": func(nsConn *websocket.NSConn, msg websocket.Message) error {
				log.Println("创建job事件")
				ctx := websocket.GetContext(nsConn.Conn)
				log.Printf("收到createJob消息: %s(%s) from [%s]-[%s]", msg.Body, msg.Event, nsConn.Conn.ID(), ctx.RemoteAddr())
				//log.Println(msg)

				data := []byte(fmt.Sprintf("我收到了消息:%s", msg.Body))
				nsConn.Ask(context.Background(), "createJob", data)
				nsConn.Conn.Write(msg)
				nsConn.Conn.Server().Broadcast(nsConn, msg)
				return nil
			},
		},
	}
	//ws := websocket.New(websocket.DefaultGobwasUpgrader, websocket.Events{
	//	websocket.OnNativeMessage: func(nsConn *websocket.NSConn, msg websocket.Message) error {
	//		ctx := websocket.GetContext(nsConn.Conn)
	//		log.Printf("收到消息: %s from [%s]-[%s]", msg.Body, nsConn.Conn.ID(), ctx.RemoteAddr())
	//		//log.Println(msg)
	//		nsConn.Conn.Server().Broadcast(nsConn, msg)
	//		return nil
	//	},
	//})
	ws := websocket.New(websocket.DefaultGobwasUpgrader, serverEvents)

	ws.OnConnect = func(c *websocket.Conn) error {
		log.Println("收到连接：", c.ID())
		clients[c.ID()] = c
		data := []byte(c.ID())
		msg := websocket.Message{
			Event: "message",
			Body:  data,
		}
		//c.Ask(context.Background(), msg)
		c.Write(msg)
		return nil
	}

	ws.OnDisconnect = func(c *websocket.Conn) {
		log.Println("断开连接：", c.ID())
	}

	app.Get("/ws", websocket.Handler(ws))

	app.Run(iris.Addr(":9000"))
}
