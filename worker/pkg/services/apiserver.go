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

// AcquireLock 获取分布式锁
//
// 参数:
//   - key: 锁的键名
//   - expire: 过期时间（秒）
//
// 返回值:
//   - lockKey: 锁的键名
//   - lockValue: 锁的值
//   - error: 获取过程中的错误
func (s *apiserverService) AcquireLock(key string, expire int) (string, string, error) {
	// 构建请求URL
	url := fmt.Sprintf("%s/lock/acquire?key=%s&expire=%d", s.ApiUrl, key, expire)

	// 创建HTTP请求
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", "", fmt.Errorf("创建HTTP请求失败: %w", err)
	}

	// 设置请求头
	req.Header.Set("Content-Type", "application/json")
	if s.ApiKey != "" {
		req.Header.Set("Authorization", "Bearer "+s.ApiKey)
	}

	// 发送请求
	resp, err := s.client.Do(req)
	if err != nil {
		return "", "", fmt.Errorf("发送HTTP请求失败: %w", err)
	}
	defer resp.Body.Close()

	// 读取响应体
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", "", fmt.Errorf("读取响应体失败: %w", err)
	}

	// 检查HTTP状态码
	if resp.StatusCode != http.StatusOK {
		return "", "", fmt.Errorf("HTTP请求失败，状态码: %d, 响应: %s", resp.StatusCode, string(body))
	}

	// 解析响应JSON
	var apiResp core.ApiserverResponse
	if err := json.Unmarshal(body, &apiResp); err != nil {
		return "", "", fmt.Errorf("解析响应JSON失败: %w", err)
	}

	// 检查API返回的code
	if apiResp.Code != 0 {
		return "", "", fmt.Errorf("API返回错误，code: %d, message: %s", apiResp.Code, apiResp.Message)
	}

	// 将data字段解析为map，直接提取key和value
	lockData, err := json.Marshal(apiResp.Data)
	if err != nil {
		return "", "", fmt.Errorf("序列化data字段失败: %w", err)
	}

	var lockResult map[string]interface{}
	if err := json.Unmarshal(lockData, &lockResult); err != nil {
		return "", "", fmt.Errorf("解析锁数据失败: %w", err)
	}

	// 检查是否成功获取锁
	if success, ok := lockResult["success"].(bool); !ok || !success {
		message := "未知错误"
		if msg, ok := lockResult["message"].(string); ok {
			message = msg
		}
		return "", "", fmt.Errorf("获取锁失败: %s", message)
	}

	// 提取key和value
	lockKey, keyOk := lockResult["key"].(string)
	lockValue, valueOk := lockResult["value"].(string)

	if !keyOk || !valueOk || lockKey == "" || lockValue == "" {
		return "", "", fmt.Errorf("获取的锁信息不完整: key=%v, value=%v", lockResult["key"], lockResult["value"])
	}

	return lockKey, lockValue, nil
}

// PingWorker 发送Worker心跳，更新is_active和last_active
//
// 参数:
//   - workerID: Worker节点ID
//
// 返回值:
//   - error: 发送过程中的错误
func (s *apiserverService) PingWorker(workerID string) error {
	// 构建请求URL
	url := fmt.Sprintf("%s/worker/%s/ping/", s.ApiUrl, workerID)

	// 创建HTTP请求
	req, err := http.NewRequest("PUT", url, nil)
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

	// 读取响应体
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("读取响应体失败: %w", err)
	}

	// 检查HTTP状态码
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP请求失败，状态码: %d, 响应: %s", resp.StatusCode, string(body))
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

	return nil
}

// GetCredential 根据凭证ID获取凭证（已解密的明文）
//
// 参数:
//   - credentialID: 凭证ID
//
// 返回值:
//   - *core.Credential: 凭证信息（包含解密后的明文value）
//   - error: 获取过程中的错误
//
// 使用示例:
//
//	// 在 MessageRunner 中获取邮件配置
//	cred, err := apiserver.GetCredential("uuid-xxx")
//	if err != nil {
//	    return fmt.Errorf("获取凭证失败: %w", err)
//	}
//
//	// 使用凭证值
//	smtpHost := cred.MustGetString("smtp_host")
//	smtpPort := cred.MustGetInt("smtp_port")
//	username := cred.MustGetString("username")
//	password := cred.MustGetString("password")
func (s *apiserverService) GetCredential(credentialID string) (*core.Credential, error) {
	// 构建请求URL - 调用解密接口获取明文
	// 注意：ApiUrl 已包含 /api/v1，无需再添加 /v1
	url := fmt.Sprintf("%s/credentials/%s/decrypt/", s.ApiUrl, credentialID)

	// 创建HTTP请求（POST方法调用解密接口）
	req, err := http.NewRequest("POST", url, nil)
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

	// 读取响应体
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取响应体失败: %w", err)
	}

	// 检查HTTP状态码
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP请求失败，状态码: %d, 响应: %s", resp.StatusCode, string(body))
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

	// 将data字段解析为Credential对象
	credentialData, err := json.Marshal(apiResp.Data)
	if err != nil {
		return nil, fmt.Errorf("序列化data字段失败: %w", err)
	}

	var credential core.Credential
	if err := json.Unmarshal(credentialData, &credential); err != nil {
		return nil, fmt.Errorf("解析Credential数据失败: %w", err)
	}

	// 验证凭证是否可用
	if !credential.IsActive {
		return nil, fmt.Errorf("凭证已被禁用: %s", credential.Name)
	}

	return &credential, nil
}
