// Package services Worker业务服务层
//
// 实现Worker节点的核心业务逻辑，包括：
// - API Server通信服务：与API Server进行HTTP通信
// - WebSocket服务：与API Server进行实时通信
// - 任务执行服务：执行具体的任务
// - 分布式锁服务：确保任务不重复执行
package services

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/codelieche/cronjob/worker/pkg/core"
)

// apiserverService API Server通信服务实现
//
// 负责与API Server进行HTTP通信，包括：
// - 获取分类信息
// - 注册Worker节点
// - 发送心跳
// - 其他API调用
type apiserverService struct {
	ApiUrl string       // API Server的URL地址
	ApiKey string       // 认证令牌
	client *http.Client // HTTP客户端
}

// NewApiserverService 创建API Server通信服务实例
//
// 参数:
//   - apiUrl: API Server的URL地址
//   - apiKey: 认证令牌
//
// 返回值:
//   - core.Apiserver: API Server通信服务接口
func NewApiserverService(apiUrl string, apiKey string) core.Apiserver {
	return &apiserverService{
		ApiUrl: apiUrl,
		ApiKey: apiKey,
		client: &http.Client{
			Timeout: 30 * time.Second, // 设置30秒超时
		},
	}
}

// GetCategory 根据分类编码获取分类信息
//
// 参数:
//   - categoryCode: 分类编码
//
// 返回值:
//   - *core.Category: 分类信息对象
//   - error: 获取过程中的错误
func (s *apiserverService) GetCategory(categoryCode string) (*core.Category, error) {
	// 构建请求URL
	url := fmt.Sprintf("%s/category/%s/", s.ApiUrl, categoryCode)

	// 创建HTTP请求
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("创建HTTP请求失败: %w", err)
	}

	// 设置请求头
	req.Header.Set("Content-Type", "application/json")
	if s.ApiKey != "" {
		req.Header.Set("Authorization", "Bearer "+s.ApiKey)
	}

	// 发送请求
	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("发送HTTP请求失败: %w", err)
	}
	defer resp.Body.Close()

	// 检查HTTP状态码
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP请求失败，状态码: %d", resp.StatusCode)
	}

	// 读取响应体
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取响应体失败: %w", err)
	}

	// 解析响应JSON
	var apiResp core.ApiserverResponse
	if err := json.Unmarshal(body, &apiResp); err != nil {
		return nil, fmt.Errorf("解析响应JSON失败: %w", err)
	}

	// 检查API返回的code
	if apiResp.Code != 0 {
		return nil, fmt.Errorf("API返回错误，code: %d, message: %s", apiResp.Code, apiResp.Message)
	}

	// 将data字段解析为Category对象
	categoryData, err := json.Marshal(apiResp.Data)
	if err != nil {
		return nil, fmt.Errorf("序列化data字段失败: %w", err)
	}

	var category core.Category
	if err := json.Unmarshal(categoryData, &category); err != nil {
		return nil, fmt.Errorf("解析Category数据失败: %w", err)
	}

	return &category, nil
}

// GetTask 根据任务ID获取任务详情
//
// 参数:
//   - taskID: 任务ID
//
// 返回值:
//   - *core.Task: 任务详情对象
//   - error: 获取过程中的错误
func (s *apiserverService) GetTask(taskID string) (*core.Task, error) {
	// 构建请求URL
	url := fmt.Sprintf("%s/task/%s/", s.ApiUrl, taskID)

	// 创建HTTP请求
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("创建HTTP请求失败: %w", err)
	}

	// 设置请求头
	req.Header.Set("Content-Type", "application/json")
	if s.ApiKey != "" {
		req.Header.Set("Authorization", "Bearer "+s.ApiKey)
	}

	// 发送请求
	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("发送HTTP请求失败: %w", err)
	}
	defer resp.Body.Close()

	// 检查HTTP状态码
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP请求失败，状态码: %d", resp.StatusCode)
	}

	// 读取响应体
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取响应体失败: %w", err)
	}

	// 解析响应JSON
	var apiResp core.ApiserverResponse
	if err := json.Unmarshal(body, &apiResp); err != nil {
		return nil, fmt.Errorf("解析响应JSON失败: %w", err)
	}

	// 检查API返回的code
	if apiResp.Code != 0 {
		return nil, fmt.Errorf("API返回错误，code: %d, message: %s", apiResp.Code, apiResp.Message)
	}

	// 将data字段解析为Task对象
	taskData, err := json.Marshal(apiResp.Data)
	if err != nil {
		return nil, fmt.Errorf("序列化data字段失败: %w", err)
	}

	var task core.Task
	if err := json.Unmarshal(taskData, &task); err != nil {
		return nil, fmt.Errorf("解析Task数据失败: %w", err)
	}

	return &task, nil
}

// AppendTaskLog 追加/创建任务日志
//
// 参数:
//   - taskID: 任务ID
//   - logs: 要追加的日志内容
//
// 返回值:
//   - error: 追加过程中的错误
func (s *apiserverService) AppendTaskLog(taskID string, content string) error {
	// 构建请求URL
	url := fmt.Sprintf("%s/tasklog/%s/append/", s.ApiUrl, taskID)

	// 构建请求体数据
	requestBody := map[string]string{
		"storage": "", // 日志存储类型，还是由后端自己决定，客户端不填写了
		"content": content,
	}

	// 将请求体转换为JSON
	jsonData, err := json.Marshal(requestBody)
	if err != nil {
		return fmt.Errorf("序列化请求体失败: %w", err)
	}

	// 创建HTTP请求
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("创建HTTP请求失败: %w", err)
	}

	// 设置请求头
	req.Header.Set("Content-Type", "application/json")
	if s.ApiKey != "" {
		req.Header.Set("Authorization", "Bearer "+s.ApiKey)
	}

	// 发送请求
	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("发送HTTP请求失败: %w", err)
	}
	defer resp.Body.Close()

	// 检查HTTP状态码
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP请求失败，状态码: %d", resp.StatusCode)
	}

	// 读取响应体
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("读取响应体失败: %w", err)
	}

	// 解析响应JSON
	var apiResp core.ApiserverResponse
	if err := json.Unmarshal(body, &apiResp); err != nil {
		return fmt.Errorf("解析响应JSON失败: %w", err)
	}

	// 检查API返回的code
	if apiResp.Code != 0 {
		return fmt.Errorf("API返回错误，code: %d, message: %s", apiResp.Code, apiResp.Message)
	}

	// 将data字段解析为map，用于验证
	dataMap, err := json.Marshal(apiResp.Data)
	if err != nil {
		return fmt.Errorf("序列化data字段失败: %w", err)
	}

	var resultMap map[string]interface{}
	if err := json.Unmarshal(dataMap, &resultMap); err != nil {
		return fmt.Errorf("解析data字段失败: %w", err)
	}

	// 验证返回结果
	// 1. 检查size是否大于0
	size, ok := resultMap["size"].(float64)
	if !ok || size <= 0 {
		return fmt.Errorf("日志追加失败，size不合法: %v", resultMap["size"])
	}

	return nil
}
