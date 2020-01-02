package common

import (
	"log"
	"testing"
)

func TestPacketData(t *testing.T) {
	message := []byte("Hello")
	data := PacketData(message)
	log.Println(data)
	lengthData := data[SocketHeaderLength : SocketHeaderLength+SocketDataLength]
	log.Println("消息内容的长度是：", BinaryToInt(lengthData))
	log.Printf("%s", data)
}
