package logger

import (
	"fmt"
	"os"
	"path"
	"runtime"
	"strings"
	"sync"

	"github.com/codelieche/cronjob/worker/pkg/config"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
)

// 全局日志器
var (
	logger *zap.Logger
	level  zap.AtomicLevel
	once   sync.Once
)

// 创建文件日志写入器
func createFileSyncer() zapcore.WriteSyncer {
	// 渠道配置
	logConfig := config.Log

	// 确保日志目录存在
	logDir := path.Dir(logConfig.FilePath)
	if err := os.MkdirAll(logDir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "创建日志目录失败: %v\n", err)
		os.Exit(1)
	}

	// 使用 lumberjack 进行日志文件滚动
	writer := &lumberjack.Logger{
		Filename:   logConfig.FilePath,
		MaxSize:    logConfig.MaxSize,
		MaxAge:     logConfig.MaxAge,
		MaxBackups: logConfig.MaxBackups,
		Compress:   logConfig.Compress,
	}
	return zapcore.AddSync(writer)
}

// 初始化日志
func InitLogger() {
	once.Do(func() {
		logConfig := config.Log

		// 设置日志级别
		level = zap.NewAtomicLevel()
		setLogLevel(logConfig.Level)

		// 创建编码器
		encoderConfig := zap.NewProductionEncoderConfig()
		encoderConfig.TimeKey = "time"
		encoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
		encoderConfig.LevelKey = "level"
		encoderConfig.EncodeLevel = zapcore.CapitalLevelEncoder
		encoderConfig.CallerKey = "caller"
		encoderConfig.EncodeCaller = zapcore.ShortCallerEncoder
		encoderConfig.MessageKey = "msg"
		encoderConfig.StacktraceKey = "stacktrace"
		encoderConfig.EncodeDuration = zapcore.StringDurationEncoder

		// 根据格式选择编码器
		var encoder zapcore.Encoder
		if logConfig.Format == "json" {
			encoder = zapcore.NewJSONEncoder(encoderConfig)
		} else {
			encoder = zapcore.NewConsoleEncoder(encoderConfig)
		}

		// 根据输出方式选择写入器
		var writeSyncer zapcore.WriteSyncer
		// 日志输出到文件/console
		switch logConfig.Output {
		case "all", "both":
			// 标准输出
			stdoutSyncer := zapcore.AddSync(os.Stdout)
			// 创建文件写入器
			fileSyncer := createFileSyncer()
			// 同时输出到文件和控制台
			writeSyncer = zapcore.NewMultiWriteSyncer(stdoutSyncer, fileSyncer)
		case "file":
			// 日志输出到文件
			writeSyncer = createFileSyncer()
		case "console", "stdout":
			// 标准输出
			writeSyncer = zapcore.AddSync(os.Stdout)
		default:
			// 标准输出
			writeSyncer = zapcore.AddSync(os.Stdout)
		}

		// 创建核心
		core := zapcore.NewCore(
			encoder,
			writeSyncer,
			level,
		)

		// 创建日志器
		logger = zap.New(
			core,
			zap.AddCaller(),
			zap.AddStacktrace(zapcore.ErrorLevel),
			zap.Development(), // 开发模式，错误日志更详细
		)
		// 禁用所有堆栈跟踪
		logger = logger.WithOptions(zap.AddStacktrace(zapcore.FatalLevel))

		// 替换全局日志
		zap.ReplaceGlobals(logger)
	})
}

// 设置日志级别
func setLogLevel(levelStr string) {
	var zapLevel zapcore.Level

	switch strings.ToLower(levelStr) {
	case "debug":
		zapLevel = zapcore.DebugLevel
	case "info":
		zapLevel = zapcore.InfoLevel
	case "warn", "warning":
		zapLevel = zapcore.WarnLevel
	case "error":
		zapLevel = zapcore.ErrorLevel
	case "dpanic":
		zapLevel = zapcore.DPanicLevel
	case "panic":
		zapLevel = zapcore.PanicLevel
	case "fatal":
		zapLevel = zapcore.FatalLevel
	default:
		zapLevel = zapcore.InfoLevel
		fmt.Fprintf(os.Stderr, "无效的日志级别: %s, 使用默认级别 info\n", levelStr)
	}

	level.SetLevel(zapLevel)
}

// 获取当前日志级别
func GetLevel() string {
	return level.String()
}

// 设置日志级别
func SetLevel(levelStr string) {
	setLogLevel(levelStr)
}

// 获取logger实例
func Logger() *zap.Logger {
	InitLogger()
	return logger
}

// Debug 级别日志
func Debug(msg string, fields ...zap.Field) {
	InitLogger()
	logger.Debug(msg, fields...)
}

// Info 级别日志
func Info(msg string, fields ...zap.Field) {
	InitLogger()
	logger.Info(msg, fields...)
}

// Warn 级别日志
func Warn(msg string, fields ...zap.Field) {
	InitLogger()
	logger.Warn(msg, fields...)
}

// Error 级别日志
func Error(msg string, fields ...zap.Field) {
	InitLogger()
	logger.Error(msg, fields...)
}

// DPanic 级别日志
func DPanic(msg string, fields ...zap.Field) {
	InitLogger()
	logger.DPanic(msg, fields...)
}

// Panic 级别日志
func Panic(msg string, fields ...zap.Field) {
	InitLogger()
	logger.Panic(msg, fields...)
}

// Fatal 级别日志
func Fatal(msg string, fields ...zap.Field) {
	InitLogger()
	logger.Fatal(msg, fields...)
}

// With 包装字段，返回新的logger
func With(fields ...zap.Field) *zap.Logger {
	InitLogger()
	return logger.With(fields...)
}

// Sync 刷新日志缓存到磁盘
func Sync() error {
	if logger != nil {
		return logger.Sync()
	}
	return nil
}

// Debugf 格式化的Debug日志
func Debugf(format string, args ...interface{}) {
	InitLogger()
	logger.Debug(fmt.Sprintf(format, args...), callerSkip(1))
}

// Infof 格式化的Info日志
func Infof(format string, args ...interface{}) {
	InitLogger()
	logger.Info(fmt.Sprintf(format, args...), callerSkip(1))
}

// Warnf 格式化的Warn日志
func Warnf(format string, args ...interface{}) {
	InitLogger()
	logger.Warn(fmt.Sprintf(format, args...), callerSkip(1))
}

// Errorf 格式化的Error日志
func Errorf(format string, args ...interface{}) {
	InitLogger()
	logger.Error(fmt.Sprintf(format, args...), callerSkip(1))
}

// DPanicf 格式化的DPanic日志
func DPanicf(format string, args ...interface{}) {
	InitLogger()
	logger.DPanic(fmt.Sprintf(format, args...), callerSkip(1))
}

// Panicf 格式化的Panic日志
func Panicf(format string, args ...interface{}) {
	InitLogger()
	logger.Panic(fmt.Sprintf(format, args...), callerSkip(1))
}

// Fatalf 格式化的Fatal日志
func Fatalf(format string, args ...interface{}) {
	InitLogger()
	logger.Fatal(fmt.Sprintf(format, args...), callerSkip(1))
}

// 调整调用栈，显示正确的调用位置
func callerSkip(skip int) zap.Field {
	_, file, line, ok := runtime.Caller(skip + 2) // +2 是为了跳过当前函数和日志函数
	if !ok {
		return zap.String("caller", "unknown")
	}

	// 简化文件路径
	file = strings.TrimPrefix(file, os.Getenv("GOPATH")+"/src/")
	file = strings.TrimPrefix(file, os.Getenv("GOROOT")+"/src/")

	return zap.String("caller", fmt.Sprintf("%s:%d", file, line))
}
