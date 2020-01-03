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

func TestBinaryToInt(t *testing.T) {
	data := []byte("abcd")
	log.Println(BinaryToInt(data))

	data2 := []byte("000a")
	log.Println(BinaryToInt(data2))
}
