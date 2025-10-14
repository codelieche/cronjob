package config

import (
	stdlog "log" // 使用别名避免与config/log.go中的log类型冲突
	"os"
)

// EncryptionConfig 加密配置结构体
// 用于敏感数据的对称加密（凭证、密码等）
type EncryptionConfig struct {
	// Key 加密密钥（AES-128需要16字节）
	// 环境变量: ENCRYPTION_KEY（必须设置！）
	Key string `json:"key"`

	// Algorithm 加密算法（预留，当前固定AES-128-CBC）
	Algorithm string `json:"algorithm"`
}

// Encryption 全局加密配置实例
var Encryption *EncryptionConfig

// parseEncryption 解析加密配置
//
// 从环境变量读取配置，支持的环境变量：
// - ENCRYPTION_KEY: 加密密钥（必须设置！）
//
// 安全要求：
// 1. 必须设置 ENCRYPTION_KEY 环境变量，否则程序无法启动
// 2. 密钥长度建议16字节（AES-128）或32字节（AES-256）
// 3. 密钥变更会导致旧数据无法解密，需要数据迁移
// 4. 生产环境密钥应该通过Secret管理（K8s Secret、Docker Secret等）
func parseEncryption() {
	// 从环境变量读取加密密钥
	key := os.Getenv("ENCRYPTION_KEY")

	// 安全检查：密钥不能为空
	if key == "" {
		stdlog.Fatal("❌ 致命错误: 未设置 ENCRYPTION_KEY 环境变量！\n" +
			"加密密钥是保护敏感数据的关键，必须设置。\n" +
			"请执行: export ENCRYPTION_KEY=\"your-secure-key-here\"\n" +
			"生成密钥: openssl rand -hex 16")
	}

	// 安全检查：密钥长度建议（警告但不阻止）
	if len(key) < 16 {
		stdlog.Printf("⚠️  警告: ENCRYPTION_KEY 长度过短（当前: %d字节），建议至少16字节（AES-128）\n", len(key))
	}

	Encryption = &EncryptionConfig{
		Key:       key,
		Algorithm: "AES-128-CBC",
	}
}

func init() {
	parseEncryption()
}
