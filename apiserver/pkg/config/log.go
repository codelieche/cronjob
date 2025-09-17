package config

import "strconv"

type log struct {
	Level      string // 日志级别
	Format     string // 日志格式
	Output     string // 日志输出
	FilePath   string // 日志文件路径
	MaxSize    int    // 日志文件最大大小
	MaxAge     int    // 日志文件最大年龄
	MaxBackups int    // 日志文件最大备份数
	Compress   bool   // 日志文件是否压缩
}

var Log *log

func init() {
	maxSize := GetDefaultEnv("LOG_MAX_SIZE", "100")
	maxAge := GetDefaultEnv("LOG_MAX_AGE", "7")
	maxBackups := GetDefaultEnv("LOG_MAX_BACKUPS", "0")
	compress := GetDefaultEnv("LOG_COMPRESS", "false")

	maxSizeInt, _ := strconv.Atoi(maxSize)
	maxAgeInt, _ := strconv.Atoi(maxAge)
	maxBackupsInt, _ := strconv.Atoi(maxBackups)
	compressBool, _ := strconv.ParseBool(compress)

	Log = &log{
		Level:      GetDefaultEnv("LOG_LEVEL", "info"),
		Format:     GetDefaultEnv("LOG_FORMAT", "json"),
		Output:     GetDefaultEnv("LOG_OUTPUT", "both"),
		FilePath:   GetDefaultEnv("LOG_FILE_PATH", "./logs/cronjob-apiserver.log"),
		MaxSize:    maxSizeInt,
		MaxAge:     maxAgeInt,
		MaxBackups: maxBackupsInt,
		Compress:   compressBool,
	}
}
