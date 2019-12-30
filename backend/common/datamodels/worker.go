package datamodels

import (
	"errors"
	"fmt"
	"net"
	"os"
	"os/user"
	"strings"
)

// Worker节点的信息
type Worker struct {
	Name string `json:"name"` // 节点的名称：Ip:Port（这样就算唯一的了）
	Host string `json:"host"` // 主机名
	User string `json:"user"` // 执行程序的用户
	Ip   string `json:"ip"`   // IP地址
	Port int    `json:"port"` // worker 监控服务的端口
	Pid  int    `json:"pid"`  // Worker的端口号
}

// 获取本机的第一个网卡IP地址
func GetFirstLocalIpAddress() (ipv4 string, err error) {
	var (
		addrs   []net.Addr
		addr    net.Addr
		ipNet   *net.IPNet
		isIpNet bool
	)
	// 获取所有的网卡
	if addrs, err = net.InterfaceAddrs(); err != nil {
		return
	}

	// 取第一个非lO的网卡IP
	for _, addr = range addrs {
		// ipv4、ipv6
		if ipNet, isIpNet = addr.(*net.IPNet); isIpNet && !ipNet.IP.IsLoopback() {

			// 转成ipv4
			if ipNet.IP.To4() != nil {
				// ipv4 = ipNet.String() // 192.168.1.101/24
				ipv4 = ipNet.IP.String() // 192.168.1.101
				// 得到第一个就返回
				return
			}
		}
	}

	// 没有获取到网卡
	return "", errors.New("没有网卡")
}

func (worker *Worker) GetInfo() {
	// 先连接etcd相关
	var (
		hostName    string
		userCurrent *user.User
		userName    string
		ipAddress   string
		err         error
	)

	// 获取到主机名
	if hostName, err = os.Hostname(); err != nil {
		hostName = "unkownhost" // 未知主机
	}
	// 获取执行程序的用户名
	if userCurrent, err = user.Current(); err != nil {
		userName = "unkownuser"
	} else {
		userName = userCurrent.Username
	}

	// 获取主机的IP
	if ipAddress, err = GetFirstLocalIpAddress(); err != nil {
		return
	} else {
		ipAddress = strings.Split(ipAddress, "/")[0]
	}

	// 对当前worker赋值
	worker.Host = hostName
	worker.User = userName
	worker.Ip = ipAddress
	worker.Pid = os.Getppid()
	worker.Name = fmt.Sprintf("%s-%s:%d", worker.Ip, worker.Host, worker.Port)
}
