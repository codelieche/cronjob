package common

import (
	"errors"
	"net"
)

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
