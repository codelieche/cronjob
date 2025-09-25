// Package services Worker业务服务层
//
// 实现Worker节点的核心业务逻辑，包括：
// - API Server通信服务：与API Server进行HTTP通信
// - WebSocket服务：与API Server进行实时通信
// - 任务执行服务：执行具体的任务
// - 分布式锁服务：确保任务不重复执行
package services

import (
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
	ApiUrl    string       // API Server的URL地址
	AuthToken string       // 认证令牌
	client    *http.Client // HTTP客户端
}

// NewApiserverService 创建API Server通信服务实例
//
// 参数:
//   - apiUrl: API Server的URL地址
//   - authToken: 认证令牌
//
// 返回值:
//   - core.Apiserver: API Server通信服务接口
func NewApiserverService(apiUrl string, authToken string) core.Apiserver {
	return &apiserverService{
		ApiUrl:    apiUrl,
		AuthToken: authToken,
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
	if s.AuthToken != "" {
		req.Header.Set("Authorization", "Bearer "+s.AuthToken)
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
	if s.AuthToken != "" {
		req.Header.Set("Authorization", "Bearer "+s.AuthToken)
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
