package services

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/codelieche/cronjob/apiserver/pkg/config"
	"github.com/codelieche/cronjob/apiserver/pkg/core"
	"github.com/codelieche/cronjob/apiserver/pkg/store"
	"github.com/codelieche/cronjob/apiserver/pkg/utils/filters"
	"github.com/codelieche/cronjob/apiserver/pkg/utils/logger"
	"github.com/codelieche/cronjob/apiserver/pkg/utils/tools"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// TaskLogShardService 分片感知的TaskLog服务
// 🔥 复用现有的存储逻辑，无需额外的LogContentStore接口
type TaskLogShardService struct {
	shardStore store.TaskLogShardStore
}

// NewTaskLogShardService 创建分片TaskLog服务
func NewTaskLogShardService(shardStore store.TaskLogShardStore) core.TaskLogService {
	return &TaskLogShardService{
		shardStore: shardStore,
	}
}

// Create 创建TaskLog
func (s *TaskLogShardService) Create(ctx context.Context, taskLog *core.TaskLog) (*core.TaskLog, error) {
	// 验证参数
	if taskLog.TaskID == uuid.Nil {
		logger.Error("task id is required")
		return nil, core.ErrBadRequest
	}

	// 设置默认值
	if taskLog.Storage == "" {
		taskLog.Storage = config.Web.LogStorage
	}

	// 根据存储类型自动生成路径
	if err := s.generatePath(taskLog); err != nil {
		logger.Error("generate path error", zap.Error(err), zap.String("storage", taskLog.Storage))
		return nil, err
	}

	// 如果提供了content，先保存内容到外部存储
	if taskLog.Content != "" {
		if err := s.saveContentToStorage(ctx, taskLog, taskLog.Content); err != nil {
			logger.Error("save content to storage error", zap.Error(err), zap.String("storage", taskLog.Storage))
			return nil, err
		}
		// 保存成功后，清空content字段（文件/S3存储时）
		if taskLog.Storage != core.TaskLogStorageDB {
			taskLog.Content = ""
		}
	}

	// 创建TaskLog记录到分片表
	result, err := s.shardStore.Create(ctx, taskLog)
	if err != nil {
		logger.Error("create task log error", zap.Error(err), zap.String("task_id", taskLog.TaskID.String()))
		return nil, err
	}

	return result, nil
}

// 🔥🔥 统一的智能查询方法 - 自动优化，向后兼容
// FindByTaskID 智能查询TaskLog，自动从context中获取优化信息，向后兼容
func (s *TaskLogShardService) FindByTaskID(ctx context.Context, taskID string) (*core.TaskLog, error) {
	return s.shardStore.FindByTaskIDSmart(ctx, taskID)
}

// FindByTaskIDWithTimeRange 根据TaskID和时间信息查找TaskLog（向后兼容）
// 🔥 性能优化：支持精确时间或时间范围过滤，避免查询所有分片表
func (s *TaskLogShardService) FindByTaskIDWithTimeRange(ctx context.Context, taskID string, createdAt, startTime, endTime *time.Time) (*core.TaskLog, error) {
	// 🔥🔥 使用智能优化机制：通过Context传递优化信息
	if createdAt != nil || startTime != nil || endTime != nil {
		opt := &store.TaskLogOptimization{
			CreatedAt: createdAt,
			StartTime: startTime,
			EndTime:   endTime,
		}
		ctx = store.WithTaskLogOptimization(ctx, opt)
	}

	// 统一使用智能查询方法
	return s.shardStore.FindByTaskIDSmart(ctx, taskID)
}

// FindByTaskIDSmart 智能查询TaskLog，自动从context中获取优化信息
func (s *TaskLogShardService) FindByTaskIDSmart(ctx context.Context, taskID string) (*core.TaskLog, error) {
	return s.shardStore.FindByTaskIDSmart(ctx, taskID)
}

// UpdateSmart 智能更新TaskLog，优先使用context中的优化信息
func (s *TaskLogShardService) UpdateSmart(ctx context.Context, taskLog *core.TaskLog) (*core.TaskLog, error) {
	return s.shardStore.UpdateSmart(ctx, taskLog)
}

// DeleteSmart 智能删除TaskLog，优先使用context中的优化信息
func (s *TaskLogShardService) DeleteSmart(ctx context.Context, taskID string) error {
	return s.shardStore.DeleteByTaskIDSmart(ctx, taskID)
}

// Update 更新TaskLog
func (s *TaskLogShardService) Update(ctx context.Context, taskLog *core.TaskLog) (*core.TaskLog, error) {
	updatedTaskLog, err := s.shardStore.Update(ctx, taskLog)
	if err != nil {
		return nil, fmt.Errorf("更新TaskLog失败: %w", err)
	}

	logger.Info("成功更新TaskLog",
		zap.String("task_id", updatedTaskLog.TaskID.String()))

	return updatedTaskLog, nil
}

// Delete 删除TaskLog
func (s *TaskLogShardService) Delete(ctx context.Context, taskLog *core.TaskLog) error {
	// 验证参数
	if taskLog.TaskID == uuid.Nil {
		logger.Error("task id is required")
		return core.ErrBadRequest
	}

	return s.DeleteByTaskID(ctx, taskLog.TaskID.String())
}

// DeleteByTaskID 根据TaskID删除TaskLog
func (s *TaskLogShardService) DeleteByTaskID(ctx context.Context, taskID string) error {
	// 1. 先获取TaskLog信息
	taskLog, err := s.shardStore.FindByTaskID(ctx, taskID)
	if err != nil {
		return err
	}

	// 2. 删除日志内容（根据存储类型）
	if err := s.deleteLogContent(ctx, taskLog); err != nil {
		logger.Warn("删除日志内容失败",
			zap.String("task_id", taskID),
			zap.Error(err))
		// 继续删除数据库记录，不因为文件删除失败而中断
	}

	// 3. 删除数据库记录
	if err := s.shardStore.DeleteByTaskID(ctx, taskID); err != nil {
		return fmt.Errorf("删除TaskLog失败: %w", err)
	}

	logger.Info("成功删除TaskLog",
		zap.String("task_id", taskID))

	return nil
}

// deleteLogContent 删除日志内容（根据存储类型）
func (s *TaskLogShardService) deleteLogContent(ctx context.Context, log *core.TaskLog) error {
	switch log.Storage {
	case core.TaskLogStorageDB:
		// 数据库存储：无需额外删除操作
		return nil
	case core.TaskLogStorageFile:
		// 文件存储：删除文件
		return s.deleteLogFile(ctx, log)
	case core.TaskLogStorageS3:
		// S3存储：删除对象
		return s.deleteLogFromS3(ctx, log)
	default:
		return fmt.Errorf("unsupported storage type: %s", log.Storage)
	}
}

// deleteLogFile 删除日志文件
func (s *TaskLogShardService) deleteLogFile(ctx context.Context, log *core.TaskLog) error {
	if log.Path == "" {
		return nil // 没有路径，无需删除
	}

	if err := os.Remove(log.Path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("delete log file failed: %w", err)
	}

	return nil
}

// deleteLogFromS3 从S3删除日志
func (s *TaskLogShardService) deleteLogFromS3(ctx context.Context, log *core.TaskLog) error {
	if log.Path == "" {
		return nil // 没有路径，无需删除
	}

	// 创建MinIO客户端
	client, err := tools.NewMinIOClientFromEnv()
	if err != nil {
		return fmt.Errorf("create minio client failed: %w", err)
	}
	defer client.Close()

	// 删除S3对象
	if err := client.DeleteObject(ctx, log.Path); err != nil {
		return fmt.Errorf("delete object from s3 failed: %w", err)
	}

	return nil
}

// List 列表查询 - 支持团队过滤
func (s *TaskLogShardService) List(ctx context.Context, offset, limit int, filterActions ...filters.Filter) ([]*core.TaskLog, error) {
	return s.shardStore.List(ctx, offset, limit, filterActions...)
}

// Count 计数查询 - 支持团队过滤
func (s *TaskLogShardService) Count(ctx context.Context, filterActions ...filters.Filter) (int64, error) {
	return s.shardStore.Count(ctx, filterActions...)
}

// ListByTeams 根据团队列表查询TaskLog
func (s *TaskLogShardService) ListByTeams(ctx context.Context, teamIDs []string, offset, limit int, filterActions ...filters.Filter) ([]*core.TaskLog, error) {
	return s.shardStore.ListByTeams(ctx, teamIDs, offset, limit, filterActions...)
}

// CountByTeams 根据团队列表计数TaskLog
func (s *TaskLogShardService) CountByTeams(ctx context.Context, teamIDs []string, filterActions ...filters.Filter) (int64, error) {
	return s.shardStore.CountByTeams(ctx, teamIDs, filterActions...)
}

// GetLogContent 获取日志内容（根据存储类型读取）
func (s *TaskLogShardService) GetLogContent(ctx context.Context, log *core.TaskLog) (string, error) {
	switch log.Storage {
	case core.TaskLogStorageDB:
		return log.Content, nil
	case core.TaskLogStorageFile:
		return s.readLogFromFile(ctx, log)
	case core.TaskLogStorageS3:
		return s.readLogFromS3(ctx, log)
	default:
		return "", fmt.Errorf("unsupported storage type: %s", log.Storage)
	}
}

// SaveLogContent 保存日志内容（根据存储类型保存）
func (s *TaskLogShardService) SaveLogContent(ctx context.Context, log *core.TaskLog, content string) error {
	switch log.Storage {
	case core.TaskLogStorageDB:
		return s.saveLogToDB(ctx, log, content)
	case core.TaskLogStorageFile:
		return s.saveLogToFile(ctx, log, content)
	case core.TaskLogStorageS3:
		return s.saveLogToS3(ctx, log, content)
	default:
		return fmt.Errorf("unsupported storage type: %s", log.Storage)
	}
}

// AppendLogContent 追加日志内容（如果不存在则创建）
func (s *TaskLogShardService) AppendLogContent(ctx context.Context, log *core.TaskLog, content string) (*core.TaskLog, error) {
	// 验证参数
	if log.TaskID == uuid.Nil {
		logger.Error("task id is required")
		return nil, core.ErrBadRequest
	}

	// 尝试获取现有任务日志
	existingLog, err := s.shardStore.FindByTaskID(ctx, log.TaskID.String())
	if err != nil {
		if err == core.ErrNotFound {
			// 如果不存在，则创建新的任务日志
			logger.Info("task log not found, creating new one", zap.String("task_id", log.TaskID.String()))

			// 设置默认值
			if log.Storage == "" {
				log.Storage = config.Web.LogStorage
			}

			// 根据存储类型自动生成路径
			if err := s.generatePath(log); err != nil {
				logger.Error("generate path error", zap.Error(err), zap.String("storage", log.Storage))
				return nil, err
			}

			// 保存内容到外部存储
			if err := s.saveContentToStorage(ctx, log, content); err != nil {
				logger.Error("save content to storage error", zap.Error(err), zap.String("storage", log.Storage))
				return nil, err
			}

			// 清空content字段（文件/S3存储时）
			if log.Storage != core.TaskLogStorageDB {
				log.Content = ""
			}

			// 创建TaskLog记录
			taskLog, err := s.shardStore.Create(ctx, log)
			if err != nil {
				logger.Error("create task log error", zap.Error(err), zap.String("task_id", log.TaskID.String()))
				return nil, err
			}

			return taskLog, nil
		}
		// 其他错误直接返回
		return nil, err
	}

	// 如果存在，则追加内容
	existingContent, err := s.GetLogContent(ctx, existingLog)
	if err != nil {
		return nil, fmt.Errorf("get existing content failed: %w", err)
	}

	// 追加新内容
	newContent := existingContent + content

	// 保存更新后的内容
	err = s.SaveLogContent(ctx, existingLog, newContent)
	if err != nil {
		return nil, fmt.Errorf("save log content failed: %w", err)
	}
	// 重新获取一次，因为size变更了
	latestLog, err := s.shardStore.FindByTaskID(ctx, log.TaskID.String())
	if err != nil {
		return nil, fmt.Errorf("find task log by task id failed: %w", err)
	}
	return latestLog, nil
}

// 🔥 以下是复用现有TaskLogService的存储逻辑

// saveLogToDB 保存日志到数据库
func (s *TaskLogShardService) saveLogToDB(ctx context.Context, log *core.TaskLog, content string) error {
	log.Content = content
	log.Size = int64(len(content))
	_, err := s.shardStore.Update(ctx, log)
	return err
}

// saveLogToFile 保存日志到文件
func (s *TaskLogShardService) saveLogToFile(ctx context.Context, log *core.TaskLog, content string) error {
	// 确保目录存在
	dir := filepath.Dir(log.Path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create directory failed: %w", err)
	}

	// 写入文件
	if err := os.WriteFile(log.Path, []byte(content), 0644); err != nil {
		return fmt.Errorf("write file failed: %w", err)
	}

	// 更新大小，content字段保持为空
	log.Size = int64(len(content))
	log.Content = "" // 文件存储时content字段为空
	_, err := s.shardStore.Update(ctx, log)
	return err
}

// saveLogToS3 保存日志到S3
func (s *TaskLogShardService) saveLogToS3(ctx context.Context, log *core.TaskLog, content string) error {
	// 创建MinIO客户端
	client, err := tools.NewMinIOClientFromEnv()
	if err != nil {
		return fmt.Errorf("create minio client failed: %w", err)
	}
	defer client.Close()

	// 生成对象名称
	objectName := log.Path
	if objectName == "" {
		objectName = client.GenerateTaskLogObjectName(log.TaskID.String())
		log.Path = objectName
	}

	// 上传到S3（设置Content-Type为text/plain，支持预览）
	reader := strings.NewReader(content)
	if err := client.PutObjectWithContentType(ctx, objectName, reader, int64(len(content)), "text/plain; charset=utf-8"); err != nil {
		return fmt.Errorf("upload to s3 failed: %w", err)
	}

	// 更新大小，content字段保持为空
	log.Size = int64(len(content))
	log.Content = "" // S3存储时content字段为空
	_, err = s.shardStore.Update(ctx, log)
	return err
}

// readLogFromFile 从文件读取日志
func (s *TaskLogShardService) readLogFromFile(ctx context.Context, log *core.TaskLog) (string, error) {
	content, err := os.ReadFile(log.Path)
	if err != nil {
		return "", fmt.Errorf("read file failed: %w", err)
	}
	return string(content), nil
}

// readLogFromS3 从S3读取日志
func (s *TaskLogShardService) readLogFromS3(ctx context.Context, log *core.TaskLog) (string, error) {
	// 创建MinIO客户端
	client, err := tools.NewMinIOClientFromEnv()
	if err != nil {
		return "", fmt.Errorf("create minio client failed: %w", err)
	}
	defer client.Close()

	// 从S3获取对象
	content, err := client.GetObjectAsString(ctx, log.Path)
	if err != nil {
		return "", fmt.Errorf("get object from s3 failed: %w", err)
	}

	return content, nil
}

// saveContentToStorage 保存内容到外部存储（不更新数据库）
func (s *TaskLogShardService) saveContentToStorage(ctx context.Context, taskLog *core.TaskLog, content string) error {
	switch taskLog.Storage {
	case core.TaskLogStorageDB:
		// 数据库存储：直接设置content和size
		taskLog.Content = content
		taskLog.Size = int64(len(content))
		return nil
	case core.TaskLogStorageFile:
		// 文件存储：保存到文件
		return s.saveContentToFile(ctx, taskLog, content)
	case core.TaskLogStorageS3:
		// S3存储：保存到S3
		return s.saveContentToS3(ctx, taskLog, content)
	default:
		return fmt.Errorf("unsupported storage type: %s", taskLog.Storage)
	}
}

// saveContentToFile 保存内容到文件（不更新数据库）
func (s *TaskLogShardService) saveContentToFile(ctx context.Context, taskLog *core.TaskLog, content string) error {
	// 确保目录存在
	dir := filepath.Dir(taskLog.Path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create directory failed: %w", err)
	}

	// 写入文件
	if err := os.WriteFile(taskLog.Path, []byte(content), 0644); err != nil {
		return fmt.Errorf("write file failed: %w", err)
	}

	// 设置大小
	taskLog.Size = int64(len(content))
	return nil
}

// saveContentToS3 保存内容到S3（不更新数据库）
func (s *TaskLogShardService) saveContentToS3(ctx context.Context, taskLog *core.TaskLog, content string) error {
	// 创建MinIO客户端
	client, err := tools.NewMinIOClientFromEnv()
	if err != nil {
		return fmt.Errorf("create minio client failed: %w", err)
	}
	defer client.Close()

	// 上传到S3（设置Content-Type为text/plain，支持预览）
	reader := strings.NewReader(content)
	if err := client.PutObjectWithContentType(ctx, taskLog.Path, reader, int64(len(content)), "text/plain; charset=utf-8"); err != nil {
		return fmt.Errorf("upload to s3 failed: %w", err)
	}

	// 设置大小
	taskLog.Size = int64(len(content))
	return nil
}

// generatePath 根据存储类型生成路径
func (s *TaskLogShardService) generatePath(taskLog *core.TaskLog) error {
	switch taskLog.Storage {
	case core.TaskLogStorageFile:
		// 文件存储：logs/{yearMonth}/task/{task_id}.log
		yearMonth := time.Now().Format("200601")
		taskLog.Path = fmt.Sprintf("logs/%s/task/%s.log", yearMonth, taskLog.TaskID.String())
		// 注意：不在这里清空content，让Create方法处理
	case core.TaskLogStorageS3:
		// S3存储：生成S3对象键
		client, err := tools.NewMinIOClientFromEnv()
		if err != nil {
			return fmt.Errorf("create minio client failed: %w", err)
		}
		defer client.Close()
		taskLog.Path = client.GenerateTaskLogObjectName(taskLog.TaskID.String())
		// 注意：不在这里清空content，让Create方法处理
	case core.TaskLogStorageDB:
		// 数据库存储：路径为空
		taskLog.Path = ""
		// content字段保持原值
	default:
		return fmt.Errorf("unsupported storage type: %s", taskLog.Storage)
	}
	return nil
}
