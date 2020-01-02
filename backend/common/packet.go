package common

import (
	"bytes"
	"encoding/binary"
)

const SocketMessageHeader = "www.codelieche.com"          // Socket发送数据包的头部
var SocketHeaderLength = len([]byte(SocketMessageHeader)) // Socket发送消息
var SocketDataLength int = 4                              // 32位

// 对要发送的数据打包
// 打包数据：Header + len(message) + message
func PacketData(message []byte) []byte {

	// 1. 先写入消息的头部信息
	dataBuf := bytes.NewBuffer([]byte(SocketMessageHeader))

	// 2. dataBuf写入：消息的长度信息（32位的）【它需要占四位】
	messageLength := uint32(len(message))
	messageLengthData := make([]byte, 4)
	binary.BigEndian.PutUint32(messageLengthData, messageLength)

	dataBuf.Write(messageLengthData)

	// 3. 写入消息的内容
	dataBuf.Write(message)

	return dataBuf.Bytes()
}

// 对数据包解包
func BinaryToInt(data []byte) (n int) {
	// 写入的时候插入的是uint32
	var length uint32
	dataBuf := bytes.NewBuffer(data)
	binary.Read(dataBuf, binary.BigEndian, &length)

	n = int(length)
	return n
}

// 对数据包解封装
func UnPacketData(data []byte, c chan<- []byte) []byte {
	length := len(data)
	var i int
	//socketHeaderData := []byte(SocketMessageHeader)
	for i = 0; i < length; i++ {
		// 余下的要处理的消息长度小于，socket的头部长度了
		if length < i+SocketHeaderLength {
			return data[i:]
		}

		// 判断是否接下来的字节是header信息
		if string(data[i:i+SocketHeaderLength]) == SocketMessageHeader {
			messageLengthData := data[i+SocketHeaderLength : i+SocketHeaderLength+SocketDataLength]
			messageLength := BinaryToInt(messageLengthData)
			message := data[i+SocketHeaderLength+SocketDataLength : i+SocketHeaderLength+SocketDataLength+messageLength]
			//log.Println("找到消息了：", i, SocketHeaderLength, SocketDataLength, messageLength, message)
			// 把消息插入到channel中
			c <- message
		}
	}
	return []byte{}
}
