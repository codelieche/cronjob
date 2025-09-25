package config

import (
	"os"
)

// GetDefaultEnv 获取环境变量，若不存在则返回默认值
// key: 环境变量名
// value: 默认值
// return: 环境变量值
func GetDefaultEnv(key, value string) string {
	val, ok := os.LookupEnv(key)
	if !ok {
		return value
	}
	return os.ExpandEnv(val)
}
