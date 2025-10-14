package runner

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/codelieche/cronjob/worker/pkg/core"
)

// FeishuBotSender 飞书群机器人发送器
//
// 用于通过飞书群机器人 Webhook 发送消息
// API文档: https://open.feishu.cn/document/ukTMukTMukTM/ucTM5YjL3ETO24yNxkjN
type FeishuBotSender struct {
	client *http.Client
}

// Send 发送飞书群机器人消息
func (s *FeishuBotSender) Send(ctx context.Context, cred *core.Credential, config MessageConfig, logChan chan<- string) (*core.Result, error) {
	startTime := time.Now()

	// 初始化 HTTP 客户端
	if s.client == nil {
		s.client = &http.Client{Timeout: 30 * time.Second}
	}

	// 1. 解析凭证字段（webhook 类型）
	webhook := cred.MustGetString("webhook")

	logChan <- "🤖 飞书群机器人"
	logChan <- fmt.Sprintf("🔗 Webhook: %s", maskWebhook(webhook))

	// 2. 构建消息体
	var message map[string]interface{}

	switch config.ContentType {
	case "text":
		message = s.buildTextMessage(config)
	case "markdown":
		message = s.buildMarkdownMessage(config)
	default:
		message = s.buildTextMessage(config)
	}

	logChan <- fmt.Sprintf("📝 消息类型: %s", config.ContentType)

	// 3. 发送消息
	jsonData, err := json.Marshal(message)
	if err != nil {
		return nil, fmt.Errorf("序列化消息失败: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", webhook, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	logChan <- "📨 正在发送消息...\n"
	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("发送请求失败: %w", err)
	}
	defer resp.Body.Close()

	// 4. 解析响应
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取响应失败: %w", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("解析响应失败: %w", err)
	}

	// 检查错误码
	code, _ := result["code"].(float64)
	if code != 0 {
		msg, _ := result["msg"].(string)
		return nil, fmt.Errorf("飞书API错误 (code=%d): %s", int(code), msg)
	}

	endTime := time.Now()
	duration := endTime.Sub(startTime)

	logChan <- fmt.Sprintf("✅ 飞书群机器人消息发送成功（耗时: %v）", duration)

	// 5. 构建返回结果
	output := fmt.Sprintf("飞书群机器人消息已发送\n消息类型: %s\n内容长度: %d 字符",
		config.ContentType,
		len(config.Content))

	return &core.Result{
		Status:    core.StatusSuccess,
		Output:    output,
		StartTime: startTime,
		EndTime:   endTime,
	}, nil
}

// buildTextMessage 构建文本消息
func (s *FeishuBotSender) buildTextMessage(config MessageConfig) map[string]interface{} {
	content := config.Content

	// 添加 @所有人
	if config.IsAtAll {
		content = content + " <at user_id=\"all\">所有人</at>"
	}

	// 添加 @指定用户
	if len(config.AtUserIds) > 0 {
		for _, userId := range config.AtUserIds {
			content = fmt.Sprintf("<at user_id=\"%s\">@%s</at> ", userId, userId) + content
		}
	}

	return map[string]interface{}{
		"msg_type": "text",
		"content": map[string]interface{}{
			"text": content,
		},
	}
}

// buildMarkdownMessage 构建 Markdown 消息
func (s *FeishuBotSender) buildMarkdownMessage(config MessageConfig) map[string]interface{} {
	content := config.Content

	// 添加 @所有人（Markdown 格式）
	if config.IsAtAll {
		content = content + "\n<at user_id=\"all\">所有人</at>"
	}

	// 添加 @指定用户
	if len(config.AtUserIds) > 0 {
		atUsers := make([]string, 0, len(config.AtUserIds))
		for _, userId := range config.AtUserIds {
			atUsers = append(atUsers, fmt.Sprintf("<at user_id=\"%s\">@%s</at>", userId, userId))
		}
		content = strings.Join(atUsers, " ") + "\n\n" + content
	}

	return map[string]interface{}{
		"msg_type": "interactive",
		"card": map[string]interface{}{
			"elements": []map[string]interface{}{
				{
					"tag":     "markdown",
					"content": content,
				},
			},
		},
	}
}
