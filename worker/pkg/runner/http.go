package runner

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/codelieche/cronjob/worker/pkg/core"
	"github.com/codelieche/cronjob/worker/pkg/utils/logger"
	"go.uber.org/zap"
)

// HTTPConfig HTTP请求配置（v2.0 简化版）
// 注意：超时和重试使用 Task 的配置，不在这里定义
type HTTPConfig struct {
	URL            string            `json:"url"`             // 必填：请求URL
	Method         string            `json:"method"`          // 选填：请求方法，默认GET
	Headers        map[string]string `json:"headers"`         // 选填：请求头
	Query          map[string]string `json:"query"`           // 选填：URL参数
	Body           interface{}       `json:"body"`            // 选填：请求体（仅JSON对象）
	ExpectedStatus []int             `json:"expected_status"` // 选填：预期状态码，默认[200]
}

// HTTPRunner HTTP请求执行器（v2.0 简化版）
type HTTPRunner struct {
	BaseRunner // 🔥 嵌入基类

	config *HTTPConfig
	client *http.Client
}

// NewHTTPRunner 创建新的HTTPRunner
func NewHTTPRunner() *HTTPRunner {
	r := &HTTPRunner{
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
	r.InitBase() // 🔥 初始化基类
	return r
}

// ParseArgs 解析任务参数
func (r *HTTPRunner) ParseArgs(task *core.Task) error {
	r.Lock() // 🔥 使用基类方法
	defer r.Unlock()

	r.Task = task // 🔥 直接访问公共字段

	// 解析args
	var config HTTPConfig
	if err := json.Unmarshal([]byte(task.Args), &config); err != nil {
		return fmt.Errorf("解析HTTP配置失败: %w", err)
	}

	// 验证必填字段
	if config.URL == "" {
		return fmt.Errorf("URL不能为空")
	}

	// 设置默认值
	if config.Method == "" {
		config.Method = "GET"
	}
	config.Method = strings.ToUpper(config.Method)

	// 注意：不再设置默认的 ExpectedStatus
	// 如果为空，将使用 HTTP 标准（< 400 成功，>= 400 失败）

	r.config = &config

	// 设置HTTP Client超时（使用Task的Timeout）
	timeout := 30 * time.Second // 默认30秒
	if task.Timeout > 0 {
		timeout = time.Duration(task.Timeout) * time.Second
	}
	r.client.Timeout = timeout

	// 替换环境变量
	if err := r.replaceVariables(); err != nil {
		return fmt.Errorf("变量替换失败: %w", err)
	}

	// 验证配置
	if err := r.validateConfig(); err != nil {
		return fmt.Errorf("配置验证失败: %w", err)
	}

	return nil
}

// replaceVariables 替换环境变量
func (r *HTTPRunner) replaceVariables() error {
	if r.Task == nil || r.Task.Metadata == nil { // 🔥 直接访问公共字段
		return nil
	}

	metadata, err := r.Task.GetMetadata() // 🔥 直接访问公共字段
	if err != nil {
		return err
	}

	if metadata.Environment == nil {
		return nil
	}

	// 替换URL中的变量
	r.config.URL = r.replaceString(r.config.URL, metadata.Environment)

	// 替换Headers中的变量
	for key, value := range r.config.Headers {
		r.config.Headers[key] = r.replaceString(value, metadata.Environment)
	}

	// 替换Query中的变量
	for key, value := range r.config.Query {
		r.config.Query[key] = r.replaceString(value, metadata.Environment)
	}

	// 替换Body中的字符串变量
	if r.config.Body != nil {
		r.config.Body = r.replaceInValue(r.config.Body, metadata.Environment)
	}

	return nil
}

// replaceString 替换字符串中的 ${VAR} 变量
func (r *HTTPRunner) replaceString(str string, env map[string]string) string {
	result := str
	for key, value := range env {
		placeholder := fmt.Sprintf("${%s}", key)
		result = strings.ReplaceAll(result, placeholder, value)
	}
	return result
}

// replaceInValue 递归替换值中的变量
func (r *HTTPRunner) replaceInValue(value interface{}, env map[string]string) interface{} {
	switch v := value.(type) {
	case string:
		return r.replaceString(v, env)
	case map[string]interface{}:
		result := make(map[string]interface{})
		for k, val := range v {
			result[k] = r.replaceInValue(val, env)
		}
		return result
	case []interface{}:
		result := make([]interface{}, len(v))
		for i, val := range v {
			result[i] = r.replaceInValue(val, env)
		}
		return result
	default:
		return value
	}
}

// validateConfig 验证配置
func (r *HTTPRunner) validateConfig() error {
	// 验证URL
	parsedURL, err := url.Parse(r.config.URL)
	if err != nil {
		return fmt.Errorf("URL格式错误: %w", err)
	}

	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return fmt.Errorf("URL必须是HTTP或HTTPS协议")
	}

	// 验证Method（v2.0只支持4种方法）
	validMethods := map[string]bool{
		"GET": true, "POST": true, "PUT": true, "DELETE": true,
	}
	if !validMethods[r.config.Method] {
		return fmt.Errorf("不支持的HTTP方法: %s (仅支持: GET, POST, PUT, DELETE)", r.config.Method)
	}

	return nil
}

// Execute 执行HTTP请求
func (r *HTTPRunner) Execute(ctx context.Context, logChan chan<- string) (*core.Result, error) {
	r.Lock()                            // 🔥 使用基类方法
	if r.Status != core.StatusPending { // 🔥 直接访问公共字段
		r.Unlock()
		return nil, fmt.Errorf("任务状态不正确，当前状态: %s", r.Status)
	}

	r.Status = core.StatusRunning // 🔥 直接访问公共字段
	startTime := time.Now()

	// 创建可取消的上下文
	execCtx, cancel := context.WithCancel(ctx)
	r.Cancel = cancel // 🔥 直接访问公共字段
	defer cancel()

	r.Unlock() // 🔥 使用基类方法

	// 发送日志
	r.sendLog(logChan, fmt.Sprintf("开始执行HTTP请求: %s %s\n", r.config.Method, r.config.URL))

	// 执行请求（不重试，重试由Task层面的max_retry控制）
	resp, err := r.doRequest(execCtx, logChan)
	if err != nil {
		r.sendLog(logChan, fmt.Sprintf("请求失败: %v\n", err))

		endTime := time.Now()
		duration := endTime.Sub(startTime).Milliseconds()

		r.Lock()                     // 🔥 使用基类方法
		r.Status = core.StatusFailed // 🔥 直接访问公共字段
		r.Result = &core.Result{     // 🔥 直接访问公共字段
			Status:     core.StatusFailed,
			Error:      fmt.Sprintf("HTTP请求失败: %v", err),
			ExecuteLog: "请求执行失败",
			StartTime:  startTime,
			EndTime:    endTime,
			Duration:   duration,
			ExitCode:   -1,
		}
		r.Unlock() // 🔥 使用基类方法

		r.sendLog(logChan, fmt.Sprintf("✗ 请求失败: %v\n", err))
		return r.Result, err // 🔥 直接访问公共字段
	}

	// 验证响应
	if err := r.validateResponse(resp); err != nil {
		r.sendLog(logChan, fmt.Sprintf("响应验证失败: %v\n", err))

		endTime := time.Now()
		duration := endTime.Sub(startTime).Milliseconds()

		r.Lock()                     // 🔥 使用基类方法
		r.Status = core.StatusFailed // 🔥 直接访问公共字段
		r.Result = &core.Result{     // 🔥 直接访问公共字段
			Status:     core.StatusFailed,
			Error:      fmt.Sprintf("响应验证失败: %v", err),
			ExecuteLog: resp.Log,
			StartTime:  startTime,
			EndTime:    endTime,
			Duration:   duration,
			ExitCode:   resp.StatusCode,
		}
		r.Unlock() // 🔥 使用基类方法

		r.sendLog(logChan, fmt.Sprintf("✗ 验证失败: %v\n", err))
		return r.Result, err // 🔥 直接访问公共字段
	}

	// 成功
	endTime := time.Now()
	duration := endTime.Sub(startTime).Milliseconds()

	r.Lock()                      // 🔥 使用基类方法
	r.Status = core.StatusSuccess // 🔥 直接访问公共字段
	r.Result = &core.Result{      // 🔥 直接访问公共字段
		Status:     core.StatusSuccess,
		Output:     resp.Body,
		ExecuteLog: resp.Log,
		StartTime:  startTime,
		EndTime:    endTime,
		Duration:   duration,
		ExitCode:   resp.StatusCode,
	}
	r.Unlock() // 🔥 使用基类方法

	r.sendLog(logChan, fmt.Sprintf("✓ 请求成功完成 (状态码: %d, 耗时: %dms)\n", resp.StatusCode, duration))
	return r.Result, nil // 🔥 直接访问公共字段
}

// HTTPResponse HTTP响应
type HTTPResponse struct {
	StatusCode int
	Headers    http.Header
	Body       string
	Log        string
}

// doRequest 执行单次HTTP请求
func (r *HTTPRunner) doRequest(ctx context.Context, logChan chan<- string) (*HTTPResponse, error) {
	// 构建URL（添加Query参数）
	requestURL := r.config.URL
	if len(r.config.Query) > 0 {
		parsedURL, _ := url.Parse(requestURL)
		q := parsedURL.Query()
		for key, value := range r.config.Query {
			q.Add(key, value)
		}
		parsedURL.RawQuery = q.Encode()
		requestURL = parsedURL.String()
	}

	// 构建请求体
	var bodyReader io.Reader
	if r.config.Body != nil && (r.config.Method == "POST" || r.config.Method == "PUT") {
		bodyBytes, err := json.Marshal(r.config.Body)
		if err != nil {
			return nil, fmt.Errorf("序列化请求体失败: %w", err)
		}
		bodyReader = bytes.NewReader(bodyBytes)
		r.sendLog(logChan, fmt.Sprintf("请求体: %s\n", string(bodyBytes)))
	}

	// 创建HTTP请求
	req, err := http.NewRequestWithContext(ctx, r.config.Method, requestURL, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %w", err)
	}

	// 设置Headers
	for key, value := range r.config.Headers {
		req.Header.Set(key, value)
	}

	// 如果没有设置Content-Type且有Body，自动设置为application/json
	if r.config.Body != nil && req.Header.Get("Content-Type") == "" {
		req.Header.Set("Content-Type", "application/json")
	}

	// 发送请求
	r.sendLog(logChan, fmt.Sprintf("发送请求: %s %s\n", r.config.Method, requestURL))

	resp, err := r.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("请求执行失败: %w", err)
	}
	defer resp.Body.Close()

	// 读取响应体
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取响应失败: %w", err)
	}

	response := &HTTPResponse{
		StatusCode: resp.StatusCode,
		Headers:    resp.Header,
		Body:       string(bodyBytes),
		Log:        fmt.Sprintf("状态码: %d\n响应体: %s", resp.StatusCode, string(bodyBytes)),
	}

	r.sendLog(logChan, fmt.Sprintf("收到响应: 状态码=%d, 大小=%d bytes", resp.StatusCode, len(bodyBytes)))

	return response, nil
}

// validateResponse 验证响应（v2.0 简化版：只验证状态码）
func (r *HTTPRunner) validateResponse(resp *HTTPResponse) error {
	// 如果没有设置预期状态码，使用 HTTP 标准判断
	// < 400: 成功（2xx 成功，3xx 重定向也算成功）
	// >= 400: 失败（4xx 客户端错误，5xx 服务端错误）
	if len(r.config.ExpectedStatus) == 0 {
		if resp.StatusCode < 400 {
			return nil
		}
		return fmt.Errorf("HTTP请求失败: 状态码 %d (>= 400 表示错误)", resp.StatusCode)
	}

	// 如果设置了预期状态码，严格匹配
	for _, expected := range r.config.ExpectedStatus {
		if resp.StatusCode == expected {
			return nil
		}
	}

	return fmt.Errorf("状态码不符合预期: 期望%v, 实际%d", r.config.ExpectedStatus, resp.StatusCode)
}

// sendLog 发送日志
func (r *HTTPRunner) sendLog(logChan chan<- string, message string) {
	if logChan != nil {
		select {
		case logChan <- fmt.Sprintf("[HTTP] %s", message):
		default:
			// 通道已满，跳过
		}
	}

	if r.Task != nil { // 🔥 直接访问公共字段
		logger.Info("HTTP请求日志",
			zap.String("task_id", r.Task.ID.String()),
			zap.String("message", message),
		)
	}
}

// Stop 停止任务
func (r *HTTPRunner) Stop() error {
	r.Lock() // 🔥 使用基类方法
	defer r.Unlock()

	if r.Cancel != nil { // 🔥 直接访问公共字段
		r.Cancel()
		if r.Task != nil { // 🔥 直接访问公共字段
			logger.Info("HTTP请求已停止", zap.String("task_id", r.Task.ID.String()))
		}
	}

	return nil
}

// Kill 强制终止
func (r *HTTPRunner) Kill() error {
	return r.Stop() // HTTP请求Stop和Kill行为一致
}

// GetStatus, GetResult 方法继承自 BaseRunner

// Cleanup 清理资源
func (r *HTTPRunner) Cleanup() error {
	r.Lock() // 🔥 使用基类方法
	defer r.Unlock()

	if r.Cancel != nil { // 🔥 直接访问公共字段
		r.Cancel()
	}

	r.Status = core.StatusPending // 🔥 直接访问公共字段
	r.Result = nil                // 🔥 直接访问公共字段

	return nil
}

// 确保HTTPRunner实现了Runner接口
var _ core.Runner = (*HTTPRunner)(nil)
