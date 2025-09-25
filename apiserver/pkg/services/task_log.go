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
	"github.com/codelieche/cronjob/apiserver/pkg/utils/filters"
	"github.com/codelieche/cronjob/apiserver/pkg/utils/logger"
	"github.com/codelieche/cronjob/apiserver/pkg/utils/tools"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// NewTaskLogService 创建TaskLogService实例
func NewTaskLogService(store core.TaskLogStore) core.TaskLogService {
	return &TaskLogService{
		store: store,
	}
}

// TaskLogService 任务日志服务实现
type TaskLogService struct {
	store core.TaskLogStore
}

// FindByTaskID 根据任务ID获取任务日志
func (s *TaskLogService) FindByTaskID(ctx context.Context, taskID string) (*core.TaskLog, error) {
	// 解析UUID
	uuidTaskID, err := uuid.Parse(taskID)
	if err != nil {
		logger.Error("parse task id error", zap.Error(err), zap.String("task_id", taskID))
		return nil, core.ErrBadRequest
	}

	taskLog, err := s.store.FindByTaskID(ctx, uuidTaskID)
	if err != nil {
		if err != core.ErrNotFound {
			logger.Error("find task log by task id error", zap.Error(err), zap.String("task_id", taskID))
		}
	}
	return taskLog, err
}

// Create 创建任务日志
func (s *TaskLogService) Create(ctx context.Context, taskLog *core.TaskLog) (*core.TaskLog, error) {
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

	// 创建TaskLog记录
	result, err := s.store.Create(ctx, taskLog)
	if err != nil {
		logger.Error("create task log error", zap.Error(err), zap.String("task_id", taskLog.TaskID.String()))
		return nil, err
	}

	return result, nil
}

// Update 更新任务日志信息
func (s *TaskLogService) Update(ctx context.Context, taskLog *core.TaskLog) (*core.TaskLog, error) {
	// 验证参数
	if taskLog.TaskID == uuid.Nil {
		logger.Error("task id is required")
		return nil, core.ErrBadRequest
	}

	// 检查任务日志是否存在
	existingLog, err := s.store.FindByTaskID(ctx, taskLog.TaskID)
	if err != nil {
		return nil, err
	}

	// 如果存储类型发生变化，需要重新生成路径
	if existingLog.Storage != taskLog.Storage {
		if err := s.generatePath(taskLog); err != nil {
			logger.Error("generate path error", zap.Error(err), zap.String("storage", taskLog.Storage))
			return nil, err
		}
	}

	result, err := s.store.Update(ctx, taskLog)
	if err != nil {
		logger.Error("update task log error", zap.Error(err), zap.String("task_id", taskLog.TaskID.String()))
	}
	return result, err
}

// Delete 删除任务日志
func (s *TaskLogService) Delete(ctx context.Context, taskLog *core.TaskLog) error {
	// 验证参数
	if taskLog.TaskID == uuid.Nil {
		logger.Error("task id is required")
		return core.ErrBadRequest
	}

	// 检查任务日志是否存在
	_, err := s.store.FindByTaskID(ctx, taskLog.TaskID)
	if err != nil {
		return err
	}

	err = s.store.Delete(ctx, taskLog)
	if err != nil {
		logger.Error("delete task log error", zap.Error(err), zap.String("task_id", taskLog.TaskID.String()))
	}
	return err
}

// DeleteByTaskID 根据任务ID删除任务日志
func (s *TaskLogService) DeleteByTaskID(ctx context.Context, taskID string) error {
	// 解析UUID
	uuidTaskID, err := uuid.Parse(taskID)
	if err != nil {
		logger.Error("parse task id error", zap.Error(err), zap.String("task_id", taskID))
		return core.ErrBadRequest
	}

	err = s.store.DeleteByTaskID(ctx, uuidTaskID)
	if err != nil {
		logger.Error("delete task log by task id error", zap.Error(err), zap.String("task_id", taskID))
	}
	return err
}

// List 获取任务日志列表
func (s *TaskLogService) List(ctx context.Context, offset int, limit int, filterActions ...filters.Filter) (logs []*core.TaskLog, err error) {
	logs, err = s.store.List(ctx, offset, limit, filterActions...)
	if err != nil {
		logger.Error("list task logs error", zap.Error(err), zap.Int("offset", offset), zap.Int("limit", limit))
	}
	return logs, err
}

// Count 统计任务日志数量
func (s *TaskLogService) Count(ctx context.Context, filterActions ...filters.Filter) (int64, error) {
	count, err := s.store.Count(ctx, filterActions...)
	if err != nil {
		logger.Error("count task logs error", zap.Error(err))
	}
	return count, err
}

// GetLogContent 获取日志内容（根据存储类型读取）
func (s *TaskLogService) GetLogContent(ctx context.Context, log *core.TaskLog) (string, error) {
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
func (s *TaskLogService) SaveLogContent(ctx context.Context, log *core.TaskLog, content string) error {
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
func (s *TaskLogService) AppendLogContent(ctx context.Context, log *core.TaskLog, content string) (*core.TaskLog, error) {
	// 验证参数
	if log.TaskID == uuid.Nil {
		logger.Error("task id is required")
		return nil, core.ErrBadRequest
	}

	// 尝试获取现有任务日志
	existingLog, err := s.store.FindByTaskID(ctx, log.TaskID)
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
			taskLog, err := s.store.Create(ctx, log)
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
	latestLog, err := s.store.FindByTaskID(ctx, log.TaskID)
	if err != nil {
		return nil, fmt.Errorf("find task log by task id failed: %w", err)
	}
	return latestLog, nil
}

// saveLogToDB 保存日志到数据库
func (s *TaskLogService) saveLogToDB(ctx context.Context, log *core.TaskLog, content string) error {
	log.Content = content
	log.Size = int64(len(content))
	_, err := s.store.Update(ctx, log)
	return err
}

// saveLogToFile 保存日志到文件
func (s *TaskLogService) saveLogToFile(ctx context.Context, log *core.TaskLog, content string) error {
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
	_, err := s.store.Update(ctx, log)
	return err
}

// saveLogToS3 保存日志到S3
func (s *TaskLogService) saveLogToS3(ctx context.Context, log *core.TaskLog, content string) error {
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
	_, err = s.store.Update(ctx, log)
	return err
}

// readLogFromFile 从文件读取日志
func (s *TaskLogService) readLogFromFile(ctx context.Context, log *core.TaskLog) (string, error) {
	content, err := os.ReadFile(log.Path)
	if err != nil {
		return "", fmt.Errorf("read file failed: %w", err)
	}
	return string(content), nil
}

// readLogFromS3 从S3读取日志
func (s *TaskLogService) readLogFromS3(ctx context.Context, log *core.TaskLog) (string, error) {
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
func (s *TaskLogService) saveContentToStorage(ctx context.Context, taskLog *core.TaskLog, content string) error {
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
func (s *TaskLogService) saveContentToFile(ctx context.Context, taskLog *core.TaskLog, content string) error {
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
func (s *TaskLogService) saveContentToS3(ctx context.Context, taskLog *core.TaskLog, content string) error {
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
func (s *TaskLogService) generatePath(taskLog *core.TaskLog) error {
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
