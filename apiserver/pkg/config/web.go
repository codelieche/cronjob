package config

import (
	"crypto/rand"
	"encoding/hex"
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
	sessionSecretKey := GetDefaultEnv("SESSION_SECRET_KEY", generateRandomSessionKey())
	sessionIDName := GetDefaultEnv("SESSION_ID_NAME", "cronjob_sessionid")
	logStorage := GetDefaultEnv("LOG_STORAGE", "file")
	port, err := strconv.Atoi(portStr)

	// 解析端口
	if err != nil {
		port = 8000
	}

	Web = &web{
		Host:             host,
		Port:             port,
		SessionSecretKey: sessionSecretKey,
		SessionIDName:    sessionIDName,
		LogStorage:       logStorage,
	}
}

// generateRandomSessionKey 生成随机的会话密钥
// 如果环境变量未设置SESSION_SECRET_KEY，则生成一个32字节的随机密钥
func generateRandomSessionKey() string {
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		// 如果生成随机密钥失败，使用一个固定但复杂的默认密钥
		return "CronJob-Default-Session-Secret-Key-2024-Please-Change-In-Production!"
	}
	return hex.EncodeToString(key)
}

// parseWeb 解析web配置
func init() {
	parseWeb()
}
