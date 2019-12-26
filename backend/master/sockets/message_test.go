package sockets

import (
	"fmt"
	"log"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

func TestSendMessage(t *testing.T) {
	if conn, response, err := websocket.DefaultDialer.Dial("ws://127.0.0.1:9000/websocket", nil); err != nil {
		t.Error(err.Error())
	} else {
		log.Println(response)
		i := 0
		for i < 100 {
			i++
			msg := fmt.Sprintf("test message %d", i)
			if err = conn.WriteMessage(1, []byte(msg)); err != nil {
				t.Error(err)
				break
			}
		}
		time.Sleep(time.Second)
		log.Println("Done")
	}
}

func TestSendJsonMessage(t *testing.T) {
	if conn, response, err := websocket.DefaultDialer.Dial("ws://127.0.0.1:9000/websocket", nil); err != nil {
		t.Error(err.Error())
	} else {
		log.Println(response)
		i := 0
		for i < 100 {
			i++
			msg := fmt.Sprintf("test message %d", i)
			data := map[string]interface{}{
				"category": "message",
				"data":     msg,
			}
			if err = conn.WriteJSON(data); err != nil {
				t.Error(err)
				break
			}
		}
		time.Sleep(time.Second)
		log.Println("Done")
	}
}
