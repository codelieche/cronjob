package tools

import (
	"fmt"
	"testing"
)

func TestRandomPassword(t *testing.T) {
	// 测试默认长度（16位）
	password := RandomPassword(0)
	if len(password) != 16 {
		t.Errorf("Expected password length 16, got %d", len(password))
	}
	fmt.Printf("Default password (16 chars): %s\n", password)

	// 测试自定义长度
	customLength := 20
	customPassword := RandomPassword(customLength)
	if len(customPassword) != customLength {
		t.Errorf("Expected password length %d, got %d", customLength, len(customPassword))
	}
	fmt.Printf("Custom password (%d chars): %s\n", customLength, customPassword)
}

func TestCryptography(t *testing.T) {
	// 创建一个测试密钥
	key := RandomPassword(16)
	fmt.Printf("Test key: %s\n", key)

	// 创建Cryptography实例
	crypto := NewCryptography(key)

	// 测试加密和解密
	testText := "codelieche"
	fmt.Printf("Original text: %s\n", testText)

	// 加密
	encrypted, err := crypto.Encrypt(testText)
	if err != nil {
		t.Errorf("Encryption failed: %v", err)
	}
	fmt.Printf("Encrypted text: %s\n", encrypted)

	// 解密
	decrypted, err := crypto.Decrypt(encrypted)
	if err != nil {
		t.Errorf("Decryption failed: %v", err)
	}
	fmt.Printf("Decrypted text: %s\n", decrypted)

	// 验证解密后的值是否与原始值相同
	if decrypted != testText {
		t.Errorf("Decrypted text does not match original: %s vs %s", decrypted, testText)
	}

	// 测试CheckCanDecrypt方法
	canDecrypt, decryptedValue := crypto.CheckCanDecrypt(encrypted)
	if !canDecrypt {
		t.Errorf("CheckCanDecrypt failed for valid encrypted text")
	}
	if decryptedValue != testText {
		t.Errorf("Decrypted value from CheckCanDecrypt does not match original")
	}

	// 测试无效的加密格式
	invalidEncrypted := "not-a-valid-hex-string"
	canDecrypt, _ = crypto.CheckCanDecrypt(invalidEncrypted)
	if canDecrypt {
		t.Errorf("CheckCanDecrypt should return false for invalid encrypted text")
	}
}