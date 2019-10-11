package common

import (
	"strings"
	"testing"
)

// 测试获取ip地址
func TestGetFirstLocalIP(t *testing.T) {
	if ip, err := GetFirstLocalIpAddress(); err != nil {
		t.Error(err)
	} else {
		ipAddress := strings.Split(ip, "/")[0]
		t.Log(ip)
		t.Log(ipAddress)
	}
}
