package common

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"log"
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

func PacketInterfaceData(object interface{}) (data []byte) {
	if data, err := json.Marshal(object); err != nil {
		log.Println(err)
		return []byte{}
	} else {
		return PacketData(data)
	}
}

// 对数据包解包：消息的长度
func BinaryToInt(data []byte) (n int) {
	// 写入的时候插入的是uint32
	var length uint32
	dataBuf := bytes.NewBuffer(data)
	binary.Read(dataBuf, binary.BigEndian, &length)

	// 对数据进行校验，比如传递了个：[]byte("abcd")
	if length > 65535 {
		log.Printf("解析BinaryToInt值是:%d > 65535| %s", length, data)
		if string(data) == "0000" {
			return 1024
		}
		n = 0
	} else {
		n = int(length)
	}

	return n
}

// 对数据包解封装
func UnPacketData(data []byte, c chan<- []byte) []byte {
	length := len(data)
	var i int
	//socketHeaderData := []byte(SocketMessageHeader)
	nextStartIndex := 0
	for i = 0; i < length; i++ {
		// 余下的要处理的消息长度小于，socket的头部长度了
		if length < i+SocketHeaderLength {
			return data[i:]
		}

		// 判断是否接下来的字节是header信息
		if string(data[i:i+SocketHeaderLength]) == SocketMessageHeader {
			messageLengthData := data[i+SocketHeaderLength : i+SocketHeaderLength+SocketDataLength]
			messageLength := BinaryToInt(messageLengthData)

			// 如果后面2字节解析出的长度大于0，才获取其中的数据
			// 有时候，解析的数据太大了，大于65535，length不正常
			if messageLength > 0 {
				// 还需要注意判断i+SocketHeaderLength+SocketDataLength+messageLength是否超出了范围
				messageEndIndx := i + SocketHeaderLength + SocketDataLength + messageLength
				if messageEndIndx > length {
					// 超出了范围:
					if messageLength == 1024 {
						messageEndIndx = length
					}
				} else {
					// 未超出范围
				}

				message := data[i+SocketHeaderLength+SocketDataLength : messageEndIndx]
				//log.Println("找到消息了：", i, SocketHeaderLength, SocketDataLength, messageLength, message)
				// 把消息插入到channel中
				c <- message
				// 计算下一个开始的下标
				i += SocketHeaderLength + SocketDataLength + messageLength
				nextStartIndex = i

			}
		}
	}

	// 有一种情况，socket收到的消息不是Pack的包呢

	// 跳出循环了，要判断是否，尾部有部分包
	if nextStartIndex >= length {
		return []byte{}
	} else {
		return data[nextStartIndex:]
	}
}
