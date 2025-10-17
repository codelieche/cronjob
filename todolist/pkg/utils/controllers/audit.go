package controllers

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"gorm.io/gorm"
)

// AuditLevel 审计日志级别
type AuditLevel int

const (
	AuditLevelInfo AuditLevel = iota
	AuditLevelWarning
	AuditLevelError
	AuditLevelCritical
)

// AuditAction 审计操作类型
type AuditAction string

const (
	AuditActionCreate AuditAction = "create"
	AuditActionUpdate AuditAction = "update"
	AuditActionDelete AuditAction = "delete"
	AuditActionRead   AuditAction = "read"
	AuditActionLogin  AuditAction = "login"
	AuditActionLogout AuditAction = "logout"
)

// AuditLog 审计日志结构
type AuditLog struct {
	ID         uint                   `gorm:"primaryKey;autoIncrement" json:"id"`                      // 审计日志ID（自增主键）
	Action     AuditAction            `gorm:"type:varchar(50);not null;index" json:"action"`           // 操作类型
	Resource   string                 `gorm:"type:varchar(100);not null;index" json:"resource"`        // 资源类型
	ResourceID string                 `gorm:"type:varchar(100);index" json:"resource_id"`              // 资源ID
	UserID     string                 `gorm:"type:varchar(100);not null;index" json:"user_id"`         // 用户ID
	Username   string                 `gorm:"type:varchar(100);index" json:"username"`                 // 用户名
	IP         string                 `gorm:"type:varchar(45);index" json:"ip"`                        // 客户端IP（支持IPv6）
	UserAgent  string                 `gorm:"type:text" json:"user_agent"`                             // 用户代理
	RequestID  string                 `gorm:"type:varchar(100);index" json:"request_id"`               // 请求ID
	Data       map[string]interface{} `gorm:"type:json" json:"data"`                                   // 操作数据（JSON格式）
	Level      AuditLevel             `gorm:"type:tinyint;not null;default:0;index" json:"level"`      // 日志级别
	Message    string                 `gorm:"type:text" json:"message"`                                // 日志消息
	Success    bool                   `gorm:"type:boolean;not null;default:true;index" json:"success"` // 操作是否成功
	Error      string                 `gorm:"type:text" json:"error"`                                  // 错误信息（如果有）
	CreatedAt  time.Time              `gorm:"autoCreateTime;index" json:"created_at"`                  // 创建时间
	UpdatedAt  time.Time              `gorm:"autoUpdateTime" json:"updated_at"`                        // 更新时间
	DeletedAt  gorm.DeletedAt         `gorm:"index" json:"-"`                                          // 软删除时间
}

// TableName 指定表名
func (AuditLog) TableName() string {
	return "audit_logs"
}

// AuditService 审计服务接口
type AuditService interface {
	// Send 发送审计日志
	Send(ctx context.Context, log *AuditLog) error

	// SendAsync 异步发送审计日志
	SendAsync(ctx context.Context, log *AuditLog) error

	// BatchSend 批量发送审计日志
	BatchSend(ctx context.Context, logs []*AuditLog) error

	// Close 关闭审计服务
	Close() error
}

// AuditHook 审计日志处理钩子函数
// 用户可以自定义审计日志的处理方式，比如保存到数据库、发送到消息队列等
type AuditHook func(ctx context.Context, log *AuditLog) error

// AuditConfig 审计服务配置
type AuditConfig struct {
	// 是否启用异步处理
	Async bool
	// 批量处理大小
	BatchSize int
	// 重试次数
	MaxRetries int
	// 重试间隔
	RetryInterval time.Duration
	// 钩子函数
	Hook AuditHook
}

// DefaultAuditService 默认审计服务实现
type DefaultAuditService struct {
	config    *AuditConfig
	db        *gorm.DB
	hook      AuditHook
	asyncChan chan *AuditLog
	closeChan chan struct{}
	closed    bool
}

// NewDefaultAuditService 创建默认审计服务
func NewDefaultAuditService() *DefaultAuditService {
	return &DefaultAuditService{
		config: &AuditConfig{
			Async:         false,
			BatchSize:     100,
			MaxRetries:    3,
			RetryInterval: time.Second,
		},
	}
}

// NewAuditService 创建审计服务
func NewAuditService(config *AuditConfig, db *gorm.DB) *DefaultAuditService {
	service := &DefaultAuditService{
		config: config,
		db:     db,
		hook:   config.Hook,
	}

	// 如果启用异步处理，启动异步处理协程
	if config.Async {
		service.asyncChan = make(chan *AuditLog, config.BatchSize*2)
		service.closeChan = make(chan struct{})
		go service.asyncProcessor()
	}

	return service
}

// NewDatabaseAuditService 创建数据库审计服务
func NewDatabaseAuditService(db *gorm.DB, async bool) *DefaultAuditService {
	config := &AuditConfig{
		Async:         async,
		BatchSize:     100,
		MaxRetries:    3,
		RetryInterval: time.Second,
		Hook:          NewDatabaseAuditHook(db),
	}

	return NewAuditService(config, db)
}

// Send 发送审计日志（同步）
func (s *DefaultAuditService) Send(ctx context.Context, log *AuditLog) error {
	if s.closed {
		return errors.New("audit service is closed")
	}

	// 设置默认值
	s.setDefaults(log)

	// 如果设置了钩子函数，使用钩子函数处理
	if s.hook != nil {
		return s.sendWithRetry(ctx, log)
	}

	// 默认实现：打印到控制台
	logBytes, _ := json.MarshalIndent(log, "", "  ")
	println("AUDIT LOG:", string(logBytes))

	return nil
}

// SendAsync 异步发送审计日志
func (s *DefaultAuditService) SendAsync(ctx context.Context, log *AuditLog) error {
	if s.closed {
		return errors.New("audit service is closed")
	}

	// 设置默认值
	s.setDefaults(log)

	// 如果启用异步处理，发送到异步通道
	if s.config.Async && s.asyncChan != nil {
		select {
		case s.asyncChan <- log:
			return nil
		case <-ctx.Done():
			return ctx.Err()
		default:
			// 通道满了，降级为同步处理
			return s.Send(ctx, log)
		}
	}

	// 降级为同步处理
	return s.Send(ctx, log)
}

// BatchSend 批量发送审计日志
func (s *DefaultAuditService) BatchSend(ctx context.Context, logs []*AuditLog) error {
	if s.closed {
		return errors.New("audit service is closed")
	}

	// 设置默认值
	for _, log := range logs {
		s.setDefaults(log)
	}

	// 如果设置了钩子函数，批量处理
	if s.hook != nil {
		return s.batchSendWithRetry(ctx, logs)
	}

	// 默认实现：逐个发送
	for _, log := range logs {
		if err := s.Send(ctx, log); err != nil {
			return err
		}
	}

	return nil
}

// Close 关闭审计服务
func (s *DefaultAuditService) Close() error {
	if s.closed {
		return nil
	}

	s.closed = true

	// 关闭异步通道
	if s.closeChan != nil {
		close(s.closeChan)
	}

	// 等待异步处理完成
	if s.asyncChan != nil {
		close(s.asyncChan)
	}

	return nil
}

// setDefaults 设置审计日志的默认值
func (s *DefaultAuditService) setDefaults(log *AuditLog) {
	if log.CreatedAt.IsZero() {
		log.CreatedAt = time.Now()
	}
	if log.UpdatedAt.IsZero() {
		log.UpdatedAt = time.Now()
	}
	if log.Level == 0 {
		log.Level = AuditLevelInfo
	}
	if log.Success && log.Error != "" {
		log.Success = false
	}
}

// sendWithRetry 带重试的发送
func (s *DefaultAuditService) sendWithRetry(ctx context.Context, log *AuditLog) error {
	var lastErr error
	for i := 0; i < s.config.MaxRetries; i++ {
		if err := s.hook(ctx, log); err == nil {
			return nil
		} else {
			lastErr = err
			if i < s.config.MaxRetries-1 {
				time.Sleep(s.config.RetryInterval)
			}
		}
	}
	return lastErr
}

// batchSendWithRetry 带重试的批量发送
func (s *DefaultAuditService) batchSendWithRetry(ctx context.Context, logs []*AuditLog) error {
	var lastErr error
	for i := 0; i < s.config.MaxRetries; i++ {
		if err := s.batchSend(ctx, logs); err == nil {
			return nil
		} else {
			lastErr = err
			if i < s.config.MaxRetries-1 {
				time.Sleep(s.config.RetryInterval)
			}
		}
	}
	return lastErr
}

// batchSend 批量发送
func (s *DefaultAuditService) batchSend(ctx context.Context, logs []*AuditLog) error {
	// 分批处理
	for i := 0; i < len(logs); i += s.config.BatchSize {
		end := i + s.config.BatchSize
		if end > len(logs) {
			end = len(logs)
		}

		batch := logs[i:end]
		for _, log := range batch {
			if err := s.hook(ctx, log); err != nil {
				return err
			}
		}
	}
	return nil
}

// asyncProcessor 异步处理器
func (s *DefaultAuditService) asyncProcessor() {
	batch := make([]*AuditLog, 0, s.config.BatchSize)
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for {
		select {
		case log, ok := <-s.asyncChan:
			if !ok {
				// 通道关闭，处理剩余批次
				if len(batch) > 0 {
					s.batchSend(context.Background(), batch)
				}
				return
			}

			batch = append(batch, log)
			if len(batch) >= s.config.BatchSize {
				s.batchSend(context.Background(), batch)
				batch = batch[:0]
			}

		case <-ticker.C:
			// 定时处理批次
			if len(batch) > 0 {
				s.batchSend(context.Background(), batch)
				batch = batch[:0]
			}

		case <-s.closeChan:
			// 关闭信号
			if len(batch) > 0 {
				s.batchSend(context.Background(), batch)
			}
			return
		}
	}
}

// 全局审计服务实例
var auditService AuditService

// SetAuditService 设置审计服务
func SetAuditService(service AuditService) {
	auditService = service
}

// GetAuditService 获取审计服务
func GetAuditService() AuditService {
	if auditService == nil {
		auditService = NewDefaultAuditService()
	}
	return auditService
}

// SetAuditHook 设置审计日志处理钩子函数
// 这是一个便捷方法，用于设置钩子函数而不需要创建新的服务实例
func SetAuditHook(hook AuditHook) {
	if auditService == nil {
		auditService = NewDefaultAuditService()
	}

	// 如果当前服务是DefaultAuditService，设置钩子函数
	if defaultService, ok := auditService.(*DefaultAuditService); ok {
		defaultService.hook = hook
	}
}

// NewDatabaseAuditHook 创建数据库审计日志钩子函数
// 自动创建审计日志表并保存日志到数据库
func NewDatabaseAuditHook(db *gorm.DB) AuditHook {
	// 自动迁移审计日志表
	if err := db.AutoMigrate(&AuditLog{}); err != nil {
		// 如果迁移失败，记录错误但不中断程序
		println("Failed to migrate audit log table:", err.Error())
	}

	// 创建索引以提高查询性能
	indexes := []string{
		"CREATE INDEX IF NOT EXISTS idx_audit_logs_user_id ON audit_logs(user_id)",
		"CREATE INDEX IF NOT EXISTS idx_audit_logs_action ON audit_logs(action)",
		"CREATE INDEX IF NOT EXISTS idx_audit_logs_resource ON audit_logs(resource)",
		"CREATE INDEX IF NOT EXISTS idx_audit_logs_created_at ON audit_logs(created_at)",
		"CREATE INDEX IF NOT EXISTS idx_audit_logs_user_action ON audit_logs(user_id, action)",
		"CREATE INDEX IF NOT EXISTS idx_audit_logs_resource_action ON audit_logs(resource, action)",
		"CREATE INDEX IF NOT EXISTS idx_audit_logs_success ON audit_logs(success)",
		"CREATE INDEX IF NOT EXISTS idx_audit_logs_level ON audit_logs(level)",
	}

	for _, indexSQL := range indexes {
		if err := db.Exec(indexSQL).Error; err != nil {
			println("Failed to create index:", err.Error())
		}
	}

	return func(ctx context.Context, log *AuditLog) error {
		// 设置默认值
		if log.CreatedAt.IsZero() {
			log.CreatedAt = time.Now()
		}
		if log.UpdatedAt.IsZero() {
			log.UpdatedAt = time.Now()
		}

		// 保存审计日志到数据库
		result := db.WithContext(ctx).Create(log)
		if result.Error != nil {
			// 如果保存失败，记录错误信息
			println("Failed to save audit log:", result.Error.Error())
			return result.Error
		}
		return nil
	}
}

// NewAsyncDatabaseAuditHook 创建异步数据库审计日志钩子函数
// 使用goroutine异步保存日志到数据库，提高性能
func NewAsyncDatabaseAuditHook(db *gorm.DB) AuditHook {
	// 自动迁移审计日志表
	if err := db.AutoMigrate(&AuditLog{}); err != nil {
		println("Failed to migrate audit log table:", err.Error())
	}

	return func(ctx context.Context, log *AuditLog) error {
		// 异步保存到数据库
		go func() {
			// 设置默认值
			if log.CreatedAt.IsZero() {
				log.CreatedAt = time.Now()
			}
			if log.UpdatedAt.IsZero() {
				log.UpdatedAt = time.Now()
			}

			result := db.WithContext(ctx).Create(log)
			if result.Error != nil {
				println("Failed to save audit log asynchronously:", result.Error.Error())
			}
		}()
		return nil
	}
}

// 初始化默认审计服务
func init() {
	auditService = NewDefaultAuditService()
}
