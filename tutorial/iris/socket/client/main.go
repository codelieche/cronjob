package main

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/kataras/iris/v12/websocket"
)

func main() {
	log.SetFlags(log.Lshortfile)
	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(10*time.Second))
	defer cancel()

	dialer := websocket.DefaultGobwasDialer
	clientEvents := websocket.Namespaces{
		"default": websocket.Events{
			websocket.OnNamespaceConnected: func(c *websocket.NSConn, msg websocket.Message) error {
				log.Printf("连接到namespace: %s", msg.Namespace)
				return nil
			},

			websocket.OnNamespaceDisconnect: func(c *websocket.NSConn, msg websocket.Message) error {
				log.Printf("断开连接，namespace: %s", msg.Namespace)
				return nil
			},

			"createJob": func(c *websocket.NSConn, msg websocket.Message) error {
				log.Printf("%s(%s)", string(msg.Body), msg.Event)
				return nil
			},

			"message": func(c *websocket.NSConn, msg websocket.Message) error {
				content := fmt.Sprintf("%s(%s)\n>>请先回车，然后再输入", string(msg.Body), msg.Event)
				log.Println(content)
				return nil
			},
		},
	}
	if client, err := websocket.Dial(ctx, dialer, "ws://127.0.0.1:9000/ws", clientEvents); err != nil {
		log.Println(err.Error())
	} else {

		defer client.Close()

		if c, err := client.Connect(ctx, "default"); err != nil {
			log.Println(err.Error())
		} else {
			//c.Emit("message", []byte("发送测试消息"))

			// 持续输入消息
			scanner := bufio.NewScanner(os.Stdin)

			for {

				fmt.Fprint(os.Stdout, ">> ")

				if !scanner.Scan() {
					log.Println(scanner.Err())
					return
				}

				data := scanner.Bytes()

				if bytes.Equal(data, []byte("exit")) {
					if err := c.Disconnect(nil); err != nil {
						log.Println("关闭出错：", err.Error())
					}
					break
				}

				if ok := c.Emit("message", data); !ok {
					log.Println("发送消息出错")
					break
				}

				//fmt.Fprint(os.Stdout, ">>")
			}
		}
	}

	log.Println("Done")
}
