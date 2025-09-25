package tools

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/codelieche/cronjob/apiserver/pkg/utils/logger"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"go.uber.org/zap"
)

// MinIOConfig MinIO配置结构
type MinIOConfig struct {
	Endpoint        string // MinIO服务端点
	AccessKeyID     string // 访问密钥ID
	SecretAccessKey string // 秘密访问密钥
	UseSSL          bool   // 是否使用SSL
	BucketName      string // 默认存储桶名称
}

// MinIOClient MinIO客户端封装
type MinIOClient struct {
	client     *minio.Client
	config     *MinIOConfig
	bucketName string
}

// NewMinIOClient 创建MinIO客户端
func NewMinIOClient(config *MinIOConfig) (*MinIOClient, error) {
	// 创建MinIO客户端
	client, err := minio.New(config.Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(config.AccessKeyID, config.SecretAccessKey, ""),
		Secure: config.UseSSL,
	})
	if err != nil {
		logger.Error("创建MinIO客户端失败", zap.Error(err))
		return nil, err
	}

	// 检查存储桶是否存在，不存在则创建
	bucketName := config.BucketName
	if bucketName == "" {
		bucketName = "cronjob-logs" // 默认存储桶名称
	}

	// 检查存储桶是否存在
	exists, err := client.BucketExists(context.Background(), bucketName)
	if err != nil {
		logger.Error("检查存储桶是否存在失败", zap.Error(err), zap.String("bucket", bucketName))
		return nil, err
	}

	// 如果存储桶不存在，则创建
	if !exists {
		err = client.MakeBucket(context.Background(), bucketName, minio.MakeBucketOptions{})
		if err != nil {
			logger.Error("创建存储桶失败", zap.Error(err), zap.String("bucket", bucketName))
			return nil, err
		}
		logger.Info("创建存储桶成功", zap.String("bucket", bucketName))
	}

	return &MinIOClient{
		client:     client,
		config:     config,
		bucketName: bucketName,
	}, nil
}

// NewMinIOClientFromEnv 从环境变量创建MinIO客户端
func NewMinIOClientFromEnv() (*MinIOClient, error) {
	config := &MinIOConfig{
		Endpoint:        getEnvOrDefault("MINIO_ENDPOINT", "localhost:9000"),
		AccessKeyID:     getEnvOrDefault("MINIO_ACCESS_KEY_ID", "minioadmin"),
		SecretAccessKey: getEnvOrDefault("MINIO_SECRET_ACCESS_KEY", "minioadmin"),
		UseSSL:          getEnvOrDefault("MINIO_USE_SSL", "false") == "true",
		BucketName:      getEnvOrDefault("MINIO_BUCKET_NAME", "cronjob-logs"),
	}

	return NewMinIOClient(config)
}

// getEnvOrDefault 获取环境变量值，如果不存在则返回默认值
func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// PutObject 上传对象到MinIO
func (m *MinIOClient) PutObject(ctx context.Context, objectName string, reader io.Reader, objectSize int64) error {
	_, err := m.client.PutObject(ctx, m.bucketName, objectName, reader, objectSize, minio.PutObjectOptions{})
	if err != nil {
		logger.Error("上传对象到MinIO失败", zap.Error(err), zap.String("object", objectName))
		return err
	}
	return nil
}

// PutObjectWithContentType 上传对象到MinIO（带Content-Type）
func (m *MinIOClient) PutObjectWithContentType(ctx context.Context, objectName string, reader io.Reader, objectSize int64, contentType string) error {
	options := minio.PutObjectOptions{
		ContentType: contentType,
	}
	_, err := m.client.PutObject(ctx, m.bucketName, objectName, reader, objectSize, options)
	if err != nil {
		logger.Error("上传对象到MinIO失败", zap.Error(err), zap.String("object", objectName), zap.String("contentType", contentType))
		return err
	}
	return nil
}

// GetObject 从MinIO获取对象
func (m *MinIOClient) GetObject(ctx context.Context, objectName string) (io.Reader, error) {
	object, err := m.client.GetObject(ctx, m.bucketName, objectName, minio.GetObjectOptions{})
	if err != nil {
		logger.Error("从MinIO获取对象失败", zap.Error(err), zap.String("object", objectName))
		return nil, err
	}
	return object, nil
}

// GetObjectAsString 从MinIO获取对象内容为字符串
func (m *MinIOClient) GetObjectAsString(ctx context.Context, objectName string) (string, error) {
	object, err := m.GetObject(ctx, objectName)
	if err != nil {
		return "", err
	}
	defer func() {
		if closer, ok := object.(io.Closer); ok {
			closer.Close()
		}
	}()

	content, err := io.ReadAll(object)
	if err != nil {
		logger.Error("读取对象内容失败", zap.Error(err), zap.String("object", objectName))
		return "", err
	}

	return string(content), nil
}

// DeleteObject 从MinIO删除对象
func (m *MinIOClient) DeleteObject(ctx context.Context, objectName string) error {
	err := m.client.RemoveObject(ctx, m.bucketName, objectName, minio.RemoveObjectOptions{})
	if err != nil {
		logger.Error("从MinIO删除对象失败", zap.Error(err), zap.String("object", objectName))
		return err
	}
	return nil
}

// ObjectExists 检查对象是否存在
func (m *MinIOClient) ObjectExists(ctx context.Context, objectName string) (bool, error) {
	_, err := m.client.StatObject(ctx, m.bucketName, objectName, minio.StatObjectOptions{})
	if err != nil {
		if minio.ToErrorResponse(err).Code == "NoSuchKey" {
			return false, nil
		}
		logger.Error("检查对象是否存在失败", zap.Error(err), zap.String("object", objectName))
		return false, err
	}
	return true, nil
}

// GetObjectInfo 获取对象信息
func (m *MinIOClient) GetObjectInfo(ctx context.Context, objectName string) (minio.ObjectInfo, error) {
	info, err := m.client.StatObject(ctx, m.bucketName, objectName, minio.StatObjectOptions{})
	if err != nil {
		logger.Error("获取对象信息失败", zap.Error(err), zap.String("object", objectName))
		return minio.ObjectInfo{}, err
	}
	return info, nil
}

// ListObjects 列出对象
func (m *MinIOClient) ListObjects(ctx context.Context, prefix string) ([]minio.ObjectInfo, error) {
	var objects []minio.ObjectInfo

	objectCh := m.client.ListObjects(ctx, m.bucketName, minio.ListObjectsOptions{
		Prefix:    prefix,
		Recursive: true,
	})

	for object := range objectCh {
		if object.Err != nil {
			logger.Error("列出对象时出错", zap.Error(object.Err))
			return nil, object.Err
		}
		objects = append(objects, object)
	}

	return objects, nil
}

// GenerateObjectName 生成对象名称
func (m *MinIOClient) GenerateObjectName(taskID string, logType string) string {
	// 格式: logs/task/{taskID}/{logType}_{timestamp}.log
	timestamp := time.Now().Format("150405")
	yearMonth := time.Now().Format("200601")
	return fmt.Sprintf("logs/%s/task/%s/%s_%s.log", yearMonth, taskID, logType, timestamp)
}

// GenerateTaskLogObjectName 生成任务日志对象名称
func (m *MinIOClient) GenerateTaskLogObjectName(taskID string) string {
	return m.GenerateObjectName(taskID, "task")
}

// GetBucketName 获取存储桶名称
func (m *MinIOClient) GetBucketName() string {
	return m.bucketName
}

// GetClient 获取MinIO客户端实例
func (m *MinIOClient) GetClient() *minio.Client {
	return m.client
}

// Close 关闭MinIO客户端连接
func (m *MinIOClient) Close() error {
	// MinIO客户端不需要显式关闭连接
	return nil
}

// HealthCheck 健康检查
func (m *MinIOClient) HealthCheck(ctx context.Context) error {
	// 尝试列出存储桶来检查连接
	_, err := m.client.ListBuckets(ctx)
	if err != nil {
		logger.Error("MinIO健康检查失败", zap.Error(err))
		return err
	}
	return nil
}

// GetPresignedURL 获取预签名URL
func (m *MinIOClient) GetPresignedURL(ctx context.Context, objectName string, expiry time.Duration) (string, error) {
	url, err := m.client.PresignedGetObject(ctx, m.bucketName, objectName, expiry, nil)
	if err != nil {
		logger.Error("获取预签名URL失败", zap.Error(err), zap.String("object", objectName))
		return "", err
	}
	return url.String(), nil
}

// CopyObject 复制对象
func (m *MinIOClient) CopyObject(ctx context.Context, srcObjectName, dstObjectName string) error {
	src := minio.CopySrcOptions{
		Bucket: m.bucketName,
		Object: srcObjectName,
	}
	dst := minio.CopyDestOptions{
		Bucket: m.bucketName,
		Object: dstObjectName,
	}

	_, err := m.client.CopyObject(ctx, dst, src)
	if err != nil {
		logger.Error("复制对象失败", zap.Error(err),
			zap.String("src", srcObjectName),
			zap.String("dst", dstObjectName))
		return err
	}
	return nil
}

// GetObjectSize 获取对象大小
func (m *MinIOClient) GetObjectSize(ctx context.Context, objectName string) (int64, error) {
	info, err := m.GetObjectInfo(ctx, objectName)
	if err != nil {
		return 0, err
	}
	return info.Size, nil
}

// IsValidObjectName 验证对象名称是否有效
func (m *MinIOClient) IsValidObjectName(objectName string) bool {
	// MinIO对象名称不能为空，不能以/开头，不能包含连续的点
	if objectName == "" || strings.HasPrefix(objectName, "/") || strings.Contains(objectName, "..") {
		return false
	}
	return true
}
