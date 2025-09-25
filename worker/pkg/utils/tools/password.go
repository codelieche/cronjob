package tools

import (
	"crypto/aes"
	"crypto/cipher"
	"encoding/hex"
	"errors"
	"math/rand"
	"strings"
	"time"
)

// 密码相关的工具
// 1. RandomPassword: 随机生成一个密码（默认16位）
// 2. Cryptography: 对称加密

// RandomPassword 随机获取N位密码
// length: 密码长度，默认16位
func RandomPassword(length int) string {
	if length <= 0 {
		length = 16
	}

	strings := "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	result := make([]byte, length)

	// 使用当前时间作为随机数种子
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	for i := range result {
		result[i] = strings[r.Intn(len(strings))]
	}

	return string(result)
}

// Cryptography 对称加密
// Symetric Cryptography
type Cryptography struct {
	key  []byte
	mode cipher.BlockMode
}

// NewCryptography 创建一个新的加密实例
// key: 密钥
func NewCryptography(key string) *Cryptography {
	// 处理密钥长度，必须为16位
	if len(key) > 16 {
		key = key[0:16]
	} else if len(key) < 16 {
		key = key + strings.Repeat(".", 16-len(key))
	}

	// 将字符串密钥转换为字节数组
	keyBytes := []byte(key)

	// 创建IV（初始化向量），这里使用固定的IV，实际应用中应该使用随机IV
	iv := []byte("0000000000000000")

	// 创建CBC模式的block模式
	block, _ := aes.NewCipher(keyBytes)
	mode := cipher.NewCBCEncrypter(block, iv)

	return &Cryptography{
		key:  keyBytes,
		mode: mode,
	}
}

// Encrypt 加密操作
func (c *Cryptography) Encrypt(text string) (string, error) {
	// 填充文本使其长度为16的倍数
	length := 16
	count := len(text)
	var paddedText string

	if count < length {
		paddedText = text + strings.Repeat(" ", length-count)
	} else {
		add := length - (count % length)
		paddedText = text + strings.Repeat(" ", add)
	}

	// 创建一个与填充后文本长度相同的字节数组
	ciphertext := make([]byte, len(paddedText))

	// 执行加密操作
	c.mode.CryptBlocks(ciphertext, []byte(paddedText))

	// 将加密后的字节转换为十六进制字符串
	return hex.EncodeToString(ciphertext), nil
}

// Decrypt 解密操作
func (c *Cryptography) Decrypt(text string) (string, error) {
	// 将十六进制字符串转换回字节数组
	ciphertext, err := hex.DecodeString(text)
	if err != nil {
		return "", errors.New("无效的加密格式")
	}

	// 创建一个与密文长度相同的字节数组用于存储明文
	plaintext := make([]byte, len(ciphertext))

	// 创建一个新的解密器
	block, _ := aes.NewCipher(c.key)
	iv := []byte("0000000000000000")
	decrypter := cipher.NewCBCDecrypter(block, iv)

	// 执行解密操作
	decrypter.CryptBlocks(plaintext, ciphertext)

	// 去除填充的空格
	return strings.TrimSpace(string(plaintext)), nil
}

// CheckCanDecrypt 判断value是否是加密后的值
func (c *Cryptography) CheckCanDecrypt(value string) (bool, string) {
	tryDecrypt, err := c.Decrypt(value)
	if err != nil {
		return false, ""
	}
	return true, tryDecrypt
}
