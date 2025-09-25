package services

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/codelieche/cronjob/worker/pkg/config"
	"github.com/codelieche/cronjob/worker/pkg/core"
	"github.com/codelieche/cronjob/worker/pkg/utils/logger"
	"go.uber.org/zap"
)

// httpLocker HTTP锁服务实现
type httpLocker struct {
	baseURL    string
	httpClient *http.Client
}

// lockImpl 锁实现
type lockImpl struct {
	key        string
	value      string
	baseURL    string
	httpClient *http.Client
}

// NewLocker 创建HTTP锁服务实例
func NewLocker() core.Locker {
	return &httpLocker{
		baseURL: config.Server.ApiUrl,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// parseAPIResponse 解析API响应
func parseAPIResponse(body []byte) (*core.ApiserverResponse, error) {
	var response core.ApiserverResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("解析响应失败: %v", err)
	}
	return &response, nil
}

// checkAPIResponse 检查API响应并返回错误（如果有）
func checkAPIResponse(response *core.ApiserverResponse, defaultErrorMsg string) error {
	if response.Code != 0 {
		if response.Message != "" {
			return fmt.Errorf(response.Message)
		}
		return fmt.Errorf(defaultErrorMsg)
	}
	return nil
}

// Acquire 获取锁
func (hl *httpLocker) Acquire(ctx context.Context, key string, expire time.Duration) (core.Lock, error) {
	// 先尝试获取锁
	lock, err := hl.TryAcquire(ctx, key, expire)
	if err != nil {
		return nil, err
	}
	return lock, nil
}

// TryAcquire 尝试获取锁
func (hl *httpLocker) TryAcquire(ctx context.Context, key string, expire time.Duration) (core.Lock, error) {
	// 构建请求URL
	reqURL := fmt.Sprintf("%s/lock/acquire?key=%s&expire=%d",
		hl.baseURL,
		url.QueryEscape(key),
		int(expire.Seconds()))

	// 创建请求
	req, err := http.NewRequestWithContext(ctx, "GET", reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %v", err)
	}

	// 添加认证头（如果有）
	if config.Server.AuthToken != "" {
		req.Header.Set("Authorization", "Bearer "+config.Server.AuthToken)
	}

	// 发送请求
	resp, err := hl.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("发送请求失败: %v", err)
	}
	defer resp.Body.Close()

	// 读取响应
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取响应失败: %v", err)
	}

	// 解析响应
	response, err := parseAPIResponse(body)
	if err != nil {
		return nil, err
	}

	// 检查响应码
	if err := checkAPIResponse(response, "获取锁失败"); err != nil {
		return nil, err
	}

	// 提取data字段
	data, ok := response.Data.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("响应中缺少data字段")
	}

	// 检查是否成功
	success, ok := data["success"].(bool)
	if !ok || !success {
		message := "获取锁失败"
		if msg, exists := data["message"].(string); exists {
			message = msg
		}
		// 锁被占用是正常情况，不是错误，返回nil表示获取失败
		return nil, fmt.Errorf(message)
	}

	// 提取锁信息
	lockKey, _ := data["key"].(string)
	lockValue, _ := data["value"].(string)

	if lockKey == "" || lockValue == "" {
		return nil, fmt.Errorf("响应中缺少锁信息")
	}

	logger.Info("成功获取锁",
		zap.String("key", lockKey),
		zap.String("value", lockValue),
		zap.Duration("expire", expire))

	return &lockImpl{
		key:        lockKey,
		value:      lockValue,
		baseURL:    hl.baseURL,
		httpClient: hl.httpClient,
	}, nil
}

// ReleaseByKeyAndValue 通过key和value释放锁
func (hl *httpLocker) ReleaseByKeyAndValue(ctx context.Context, key, value string) error {
	// 构建请求URL
	reqURL := fmt.Sprintf("%s/lock/release?key=%s&value=%s",
		hl.baseURL,
		url.QueryEscape(key),
		url.QueryEscape(value))

	// 创建请求
	req, err := http.NewRequestWithContext(ctx, "GET", reqURL, nil)
	if err != nil {
		return fmt.Errorf("创建请求失败: %v", err)
	}

	// 添加认证头（如果有）
	if config.Server.AuthToken != "" {
		req.Header.Set("Authorization", "Bearer "+config.Server.AuthToken)
	}

	// 发送请求
	resp, err := hl.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("发送请求失败: %v", err)
	}
	defer resp.Body.Close()

	// 读取响应
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("读取响应失败: %v", err)
	}

	// 解析响应
	response, err := parseAPIResponse(body)
	if err != nil {
		return err
	}

	// 检查响应码
	if err := checkAPIResponse(response, "释放锁失败"); err != nil {
		return err
	}

	// 提取data字段
	data, ok := response.Data.(map[string]interface{})
	if !ok {
		return fmt.Errorf("响应中缺少data字段")
	}

	// 检查状态
	status, _ := data["status"].(string)
	if status != "success" {
		message := "释放锁失败"
		if msg, exists := data["message"].(string); exists {
			message = msg
		}
		return fmt.Errorf(message)
	}

	logger.Info("成功释放锁",
		zap.String("key", key),
		zap.String("value", value))

	return nil
}

// CheckLock 检查锁状态
func (hl *httpLocker) CheckLock(ctx context.Context, key string, value string) (bool, string, error) {
	// 构建请求URL
	reqURL := fmt.Sprintf("%s/lock/check?key=%s", hl.baseURL, url.QueryEscape(key))
	if value != "" {
		reqURL += "&value=" + url.QueryEscape(value)
	}

	// 创建请求
	req, err := http.NewRequestWithContext(ctx, "GET", reqURL, nil)
	if err != nil {
		return false, "", fmt.Errorf("创建请求失败: %v", err)
	}

	// 添加认证头（如果有）
	if config.Server.AuthToken != "" {
		req.Header.Set("Authorization", "Bearer "+config.Server.AuthToken)
	}

	// 发送请求
	resp, err := hl.httpClient.Do(req)
	if err != nil {
		return false, "", fmt.Errorf("发送请求失败: %v", err)
	}
	defer resp.Body.Close()

	// 读取响应
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return false, "", fmt.Errorf("读取响应失败: %v", err)
	}

	// 解析响应
	response, err := parseAPIResponse(body)
	if err != nil {
		return false, "", err
	}

	// 检查响应码
	if err := checkAPIResponse(response, "检查锁状态失败"); err != nil {
		return false, "", err
	}

	// 提取data字段
	data, ok := response.Data.(map[string]interface{})
	if !ok {
		return false, "", fmt.Errorf("响应中缺少data字段")
	}

	// 检查状态
	status, _ := data["status"].(string)
	if status != "success" {
		message := "检查锁状态失败"
		if msg, exists := data["message"].(string); exists {
			message = msg
		}
		return false, "", fmt.Errorf(message)
	}

	// 提取锁状态
	isLocked, _ := data["is_locked"].(bool)
	storedValue, _ := data["value"].(string)

	return isLocked, storedValue, nil
}

// RefreshLock 续租锁
func (hl *httpLocker) RefreshLock(ctx context.Context, key string, value string, expire time.Duration) error {
	// 构建请求URL
	reqURL := fmt.Sprintf("%s/lock/refresh?key=%s&value=%s&expire=%d",
		hl.baseURL,
		url.QueryEscape(key),
		url.QueryEscape(value),
		int(expire.Seconds()))

	logger.Debug("开始续租锁",
		zap.String("key", key),
		zap.String("value", value),
		zap.String("url", reqURL),
		zap.Duration("expire", expire))

	// 创建请求
	req, err := http.NewRequestWithContext(ctx, "GET", reqURL, nil)
	if err != nil {
		return fmt.Errorf("创建请求失败: %v", err)
	}

	// 添加认证头（如果有）
	if config.Server.AuthToken != "" {
		req.Header.Set("Authorization", "Bearer "+config.Server.AuthToken)
	}

	// 发送请求
	resp, err := hl.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("发送请求失败: %v", err)
	}
	defer resp.Body.Close()

	// 读取响应
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("读取响应失败: %v", err)
	}

	// 解析响应
	response, err := parseAPIResponse(body)
	if err != nil {
		return err
	}

	// 检查响应码
	if err := checkAPIResponse(response, "续租锁失败"); err != nil {
		return err
	}

	// 提取data字段
	data, ok := response.Data.(map[string]interface{})
	if !ok {
		return fmt.Errorf("响应中缺少data字段")
	}

	// 检查状态
	status, _ := data["status"].(string)
	if status != "success" {
		message := "续租锁失败"
		if msg, exists := data["message"].(string); exists {
			message = msg
		}
		return fmt.Errorf(message)
	}

	logger.Debug("成功续租锁",
		zap.String("key", key),
		zap.String("value", value),
		zap.Duration("expire", expire))

	return nil
}

// ========== Lock实现 ==========

// Key 获取锁的键名
func (l *lockImpl) Key() string {
	return l.key
}

// Value 获取锁的值
func (l *lockImpl) Value() string {
	return l.value
}

// Release 释放锁
func (l *lockImpl) Release(ctx context.Context) error {
	// 构建请求URL
	reqURL := fmt.Sprintf("%s/lock/release/?key=%s&value=%s",
		l.baseURL,
		url.QueryEscape(l.key),
		url.QueryEscape(l.value))

	// 创建请求
	req, err := http.NewRequestWithContext(ctx, "GET", reqURL, nil)
	if err != nil {
		return fmt.Errorf("创建请求失败: %v", err)
	}

	// 添加认证头（如果有）
	if config.Server.AuthToken != "" {
		req.Header.Set("Authorization", "Bearer "+config.Server.AuthToken)
	}

	// 发送请求
	resp, err := l.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("发送请求失败: %v", err)
	}
	defer resp.Body.Close()

	// 读取响应
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("读取响应失败: %v", err)
	}

	// 解析响应
	response, err := parseAPIResponse(body)
	if err != nil {
		return err
	}

	// 检查响应码
	if err := checkAPIResponse(response, "释放锁失败"); err != nil {
		return err
	}

	// 提取data字段
	data, ok := response.Data.(map[string]interface{})
	if !ok {
		return fmt.Errorf("响应中缺少data字段")
	}

	// 检查状态
	status, _ := data["status"].(string)
	if status != "success" {
		message := "释放锁失败"
		if msg, exists := data["message"].(string); exists {
			message = msg
		}
		return fmt.Errorf(message)
	}

	logger.Info("成功释放锁",
		zap.String("key", l.key),
		zap.String("value", l.value))

	return nil
}

// Refresh 续租锁
func (l *lockImpl) Refresh(ctx context.Context, expire time.Duration) error {
	// 构建请求URL
	reqURL := fmt.Sprintf("%s/lock/refresh/?key=%s&value=%s&expire=%d",
		l.baseURL,
		url.QueryEscape(l.key),
		url.QueryEscape(l.value),
		int(expire.Seconds()))

	logger.Debug("开始续租锁",
		zap.String("key", l.key),
		zap.String("value", l.value),
		zap.String("url", reqURL),
		zap.Duration("expire", expire))

	// 创建请求
	req, err := http.NewRequestWithContext(ctx, "GET", reqURL, nil)
	if err != nil {
		return fmt.Errorf("创建请求失败: %v", err)
	}

	// 添加认证头（如果有）
	if config.Server.AuthToken != "" {
		req.Header.Set("Authorization", "Bearer "+config.Server.AuthToken)
	}

	// 发送请求
	resp, err := l.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("发送请求失败: %v", err)
	}
	defer resp.Body.Close()

	// 读取响应
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("读取响应失败: %v", err)
	}

	// 解析响应
	response, err := parseAPIResponse(body)
	if err != nil {
		return err
	}

	// 检查响应码
	if err := checkAPIResponse(response, "续租锁失败"); err != nil {
		return err
	}

	// 提取data字段
	data, ok := response.Data.(map[string]interface{})
	if !ok {
		return fmt.Errorf("响应中缺少data字段")
	}

	// 检查状态
	status, _ := data["status"].(string)
	if status != "success" {
		message := "续租锁失败"
		if msg, exists := data["message"].(string); exists {
			message = msg
		}
		return fmt.Errorf(message)
	}

	logger.Debug("成功续租锁",
		zap.String("key", l.key),
		zap.String("value", l.value),
		zap.Duration("expire", expire))

	return nil
}

// AutoRefresh 自动续租锁
func (l *lockImpl) AutoRefresh(ctx context.Context, expire time.Duration, interval time.Duration) (func(), error) {
	// 创建停止通道
	stopChan := make(chan struct{})

	// 启动自动续租goroutine
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				logger.Debug("上下文取消，停止自动续租", zap.String("key", l.key))
				return
			case <-stopChan:
				logger.Debug("收到停止信号，停止自动续租", zap.String("key", l.key))
				return
			case <-ticker.C:
				// 执行续租
				if err := l.Refresh(ctx, expire); err != nil {
					logger.Error("自动续租失败",
						zap.String("key", l.key),
						zap.Error(err))
					// 续租失败，停止自动续租
					return
				}
				logger.Debug("自动续租成功",
					zap.String("key", l.key),
					zap.Duration("expire", expire),
					zap.Duration("interval", interval))
			}
		}
	}()

	// 返回停止函数
	return func() {
		close(stopChan)
	}, nil
}

// IsLocked 检查锁是否仍然有效
func (l *lockImpl) IsLocked(ctx context.Context) (bool, error) {
	// 构建请求URL
	reqURL := fmt.Sprintf("%s/lock/check?key=%s&value=%s",
		l.baseURL,
		url.QueryEscape(l.key),
		url.QueryEscape(l.value))

	// 创建请求
	req, err := http.NewRequestWithContext(ctx, "GET", reqURL, nil)
	if err != nil {
		return false, fmt.Errorf("创建请求失败: %v", err)
	}

	// 添加认证头（如果有）
	if config.Server.AuthToken != "" {
		req.Header.Set("Authorization", "Bearer "+config.Server.AuthToken)
	}

	// 发送请求
	resp, err := l.httpClient.Do(req)
	if err != nil {
		return false, fmt.Errorf("发送请求失败: %v", err)
	}
	defer resp.Body.Close()

	// 读取响应
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return false, fmt.Errorf("读取响应失败: %v", err)
	}

	// 解析响应
	response, err := parseAPIResponse(body)
	if err != nil {
		return false, err
	}

	// 检查响应码
	if err := checkAPIResponse(response, "检查锁状态失败"); err != nil {
		return false, err
	}

	// 提取data字段
	data, ok := response.Data.(map[string]interface{})
	if !ok {
		return false, fmt.Errorf("响应中缺少data字段")
	}

	// 检查状态
	status, _ := data["status"].(string)
	if status != "success" {
		message := "检查锁状态失败"
		if msg, exists := data["message"].(string); exists {
			message = msg
		}
		return false, fmt.Errorf(message)
	}

	// 提取锁状态
	isLocked, _ := data["is_locked"].(bool)
	valueMatched, _ := data["value_matched"].(bool)

	return isLocked && valueMatched, nil
}
