// Package services 注册服务
//
// 提供注册相关的业务逻辑，包括：
// - 向usercenter注册权限和角色
// - 向usercenter注册平台配置
package services

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/codelieche/cronjob/apiserver/pkg/config"
	"github.com/codelieche/cronjob/apiserver/pkg/core"
	"github.com/codelieche/cronjob/apiserver/pkg/utils/logger"
	"go.uber.org/zap"
)

// registryService 注册服务实现
type registryService struct {
	httpClient *http.Client
	authConfig *config.AuthConfig
}

// NewRegistryService 创建注册服务实例
func NewRegistryService() core.RegistryService {
	return &registryService{
		httpClient: &http.Client{
			Timeout: config.Auth.Timeout,
			Transport: &http.Transport{
				MaxIdleConns:        config.Auth.MaxIdleConns,
				MaxIdleConnsPerHost: 5,
				IdleConnTimeout:     config.Auth.IdleConnTimeout,
			},
		},
		authConfig: config.Auth,
	}
}

// RegisterPermissions 注册权限和角色到用户中心
func (s *registryService) RegisterPermissions(ctx context.Context, req *core.PermissionRegistryRequest) (*core.PermissionRegistryResponse, error) {
	url := fmt.Sprintf("%s/registry/permissions/", s.authConfig.ApiUrl)

	// 序列化请求数据
	jsonData, err := json.Marshal(req)
	if err != nil {
		logger.Error("序列化权限注册请求失败", zap.Error(err))
		return nil, fmt.Errorf("序列化请求数据失败: %w", err)
	}

	// 发送HTTP请求
	resp, err := s.sendHTTPRequest(ctx, "POST", url, jsonData)
	if err != nil {
		return nil, fmt.Errorf("发送权限注册请求失败: %w", err)
	}

	// 解析响应
	var response core.PermissionRegistryResponse
	if err := json.Unmarshal(resp, &response); err != nil {
		logger.Error("解析权限注册响应失败", zap.Error(err), zap.String("response", string(resp)))
		return nil, fmt.Errorf("解析响应数据失败: %w", err)
	}

	logger.Info("权限注册成功",
		zap.String("system_code", req.SystemCode),
		zap.Int("permissions", len(req.Permissions)),
		zap.Int("roles", len(req.Roles)),
	)

	return &response, nil
}

// RegisterPlatforms 注册平台到用户中心
func (s *registryService) RegisterPlatforms(ctx context.Context, req *core.PlatformRegistryRequest) (*core.PlatformRegistryResponse, error) {
	url := fmt.Sprintf("%s/registry/platforms/", s.authConfig.ApiUrl)

	// 序列化请求数据
	jsonData, err := json.Marshal(req)
	if err != nil {
		logger.Error("序列化平台注册请求失败", zap.Error(err))
		return nil, fmt.Errorf("序列化请求数据失败: %w", err)
	}

	// 发送HTTP请求
	resp, err := s.sendHTTPRequest(ctx, "POST", url, jsonData)
	if err != nil {
		return nil, fmt.Errorf("发送平台注册请求失败: %w", err)
	}

	// 解析响应
	var response core.PlatformRegistryResponse
	if err := json.Unmarshal(resp, &response); err != nil {
		logger.Error("解析平台注册响应失败", zap.Error(err), zap.String("response", string(resp)))
		return nil, fmt.Errorf("解析响应数据失败: %w", err)
	}

	logger.Info("平台注册成功",
		zap.String("system_code", req.SystemCode),
		zap.Int("platforms", len(req.Platforms)),
	)

	return &response, nil
}

// sendHTTPRequest 发送HTTP请求（带重试机制）
func (s *registryService) sendHTTPRequest(ctx context.Context, method, url string, data []byte) ([]byte, error) {
	var lastErr error

	for i := 0; i <= s.authConfig.MaxRetries; i++ {
		if i > 0 {
			// 等待重试延迟
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(s.authConfig.RetryInterval * time.Duration(i)):
			}

			logger.Warn("重试HTTP请求",
				zap.String("method", method),
				zap.String("url", url),
				zap.Int("attempt", i+1),
				zap.Error(lastErr),
			)
		}

		// 创建HTTP请求
		req, err := http.NewRequestWithContext(ctx, method, url, bytes.NewBuffer(data))
		if err != nil {
			lastErr = fmt.Errorf("创建HTTP请求失败: %w", err)
			continue
		}

		// 设置请求头
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("User-Agent", "todolist/1.0")

		// 如果配置了API Key，添加到请求头
		if s.authConfig.ApiKey != "" {
			req.Header.Set("Authorization", "Bearer "+s.authConfig.ApiKey)
		}

		// 发送请求
		resp, err := s.httpClient.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("发送HTTP请求失败: %w", err)
			continue
		}

		// 读取响应体
		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()

		if err != nil {
			lastErr = fmt.Errorf("读取响应体失败: %w", err)
			continue
		}

		// 检查HTTP状态码
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			lastErr = fmt.Errorf("HTTP请求失败，状态码: %d, 响应: %s", resp.StatusCode, string(body))

			// 对于客户端错误（4xx），不进行重试
			if resp.StatusCode >= 400 && resp.StatusCode < 500 {
				break
			}
			continue
		}

		// 请求成功
		logger.Debug("HTTP请求成功",
			zap.String("method", method),
			zap.String("url", url),
			zap.Int("status_code", resp.StatusCode),
			zap.Int("attempt", i+1),
		)

		return body, nil
	}

	logger.Error("HTTP请求最终失败",
		zap.String("method", method),
		zap.String("url", url),
		zap.Int("max_attempts", s.authConfig.MaxRetries+1),
		zap.Error(lastErr),
	)

	return nil, lastErr
}
