package config

import (
	"strconv"
)

// web 配置
type web struct {
	Host             string // 监听主机
	Port             int    // 监听端口
	SessionSecretKey string // 会话的secretKey
	SessionIDName    string // 会话的cookie name
	LogStorage       string // 保存日志默认的存储类型
}

// Address 获取web服务监听的地址
func (w *web) Address() string {
	return w.Host + ":" + strconv.Itoa(w.Port)
}

var Web *web

// parseWeb 解析web配置
func parseWeb() {
	host := GetDefaultEnv("WEB_HOST", "0.0.0.0")
	portStr := GetDefaultEnv("WEB_PORT", "8000")
	sessionSecretKey := GetDefaultEnv("SESSION_SECRET_KEY", "SessionIsSecret")
	sessionIDName := GetDefaultEnv("SESSION_ID_NAME", "cronjob_sessionid")
	logStorage := GetDefaultEnv("LOG_STORAGE", "file")
	port, err := strconv.Atoi(portStr)

	// 解析端口
	if err != nil {
		port = 8000
	}

	Web = &web{
		host,
		port,
		sessionSecretKey,
		sessionIDName,
		logStorage,
	}
}

// parseWeb 解析web配置
func init() {
	parseWeb()
}
