package runner

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/codelieche/cronjob/worker/pkg/core"
)

// WechatWorkSender 企业微信应用消息发送器
//
// 用于通过企业微信应用发送消息到成员/部门/标签
// API文档: https://developer.work.weixin.qq.com/document/path/90236
type WechatWorkSender struct {
	client *http.Client
}

// WechatWorkBotSender 企业微信群机器人发送器
//
// 用于通过群机器人 Webhook 发送消息
// API文档: https://developer.work.weixin.qq.com/document/path/91770
type WechatWorkBotSender struct {
	client *http.Client
}

// Send 发送企业微信应用消息
func (s *WechatWorkSender) Send(ctx context.Context, cred *core.Credential, config MessageConfig, logChan chan<- string) (*core.Result, error) {
	startTime := time.Now()

	// 初始化 HTTP 客户端
	if s.client == nil {
		s.client = &http.Client{Timeout: 30 * time.Second}
	}

	// 1. 解析凭证字段（wechat_work 类型）
	corpId := cred.MustGetString("corp_id")
	corpSecret := cred.MustGetString("corp_secret")
	agentId := cred.MustGetInt("agent_id")

	logChan <- fmt.Sprintf("🏢 企业ID: %s", corpId)
	logChan <- fmt.Sprintf("📱 应用ID: %d", agentId)

	// 2. 获取 access_token
	logChan <- "🔑 获取 access_token..."
	token, err := s.getAccessToken(ctx, corpId, corpSecret)
	if err != nil {
		return nil, fmt.Errorf("获取access_token失败: %w", err)
	}
	logChan <- "✅ access_token 获取成功"

	// 3. 构建消息体
	msgType := "text"
	if config.ContentType == "markdown" {
		msgType = "markdown"
	}

	message := map[string]interface{}{
		"touser":  config.ToUser,  // 成员ID（多个用|分隔）
		"toparty": config.ToParty, // 部门ID
		"totag":   config.ToTag,   // 标签ID
		"msgtype": msgType,
		"agentid": agentId,
	}

	// 设置消息内容
	if msgType == "markdown" {
		message["markdown"] = map[string]string{
			"content": config.Content,
		}
	} else {
		message["text"] = map[string]string{
			"content": config.Content,
		}
	}

	logChan <- fmt.Sprintf("📤 发送对象: 成员=%s, 部门=%s, 标签=%s",
		firstNonEmpty(config.ToUser, "无"),
		firstNonEmpty(config.ToParty, "无"),
		firstNonEmpty(config.ToTag, "无"))

	// 4. 发送消息
	url := fmt.Sprintf("https://qyapi.weixin.qq.com/cgi-bin/message/send?access_token=%s", token)

	jsonData, err := json.Marshal(message)
	if err != nil {
		return nil, fmt.Errorf("序列化消息失败: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	logChan <- "📨 正在发送消息..."
	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("发送请求失败: %w", err)
	}
	defer resp.Body.Close()

	// 5. 解析响应
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取响应失败: %w", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("解析响应失败: %w", err)
	}

	// 检查错误码
	errcode, _ := result["errcode"].(float64)
	if errcode != 0 {
		errmsg, _ := result["errmsg"].(string)
		return nil, fmt.Errorf("企业微信API错误 (errcode=%d): %s", int(errcode), errmsg)
	}

	endTime := time.Now()
	duration := endTime.Sub(startTime)

	logChan <- fmt.Sprintf("✅ 企业微信消息发送成功（耗时: %v）", duration)

	// 6. 构建返回结果
	output := fmt.Sprintf("企业微信应用消息已发送\n应用ID: %d\n消息类型: %s\n内容长度: %d 字符",
		agentId,
		msgType,
		len(config.Content))

	return &core.Result{
		Status:    core.StatusSuccess,
		Output:    output,
		StartTime: startTime,
		EndTime:   endTime,
	}, nil
}

// getAccessToken 获取企业微信 access_token
func (s *WechatWorkSender) getAccessToken(ctx context.Context, corpId, corpSecret string) (string, error) {
	url := fmt.Sprintf("https://qyapi.weixin.qq.com/cgi-bin/gettoken?corpid=%s&corpsecret=%s",
		corpId, corpSecret)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return "", err
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", err
	}

	// 检查错误码
	errcode, _ := result["errcode"].(float64)
	if errcode != 0 {
		errmsg, _ := result["errmsg"].(string)
		return "", fmt.Errorf("获取token失败 (errcode=%d): %s", int(errcode), errmsg)
	}

	token, ok := result["access_token"].(string)
	if !ok || token == "" {
		return "", fmt.Errorf("access_token 为空")
	}

	return token, nil
}

// Send 发送企业微信群机器人消息
func (s *WechatWorkBotSender) Send(ctx context.Context, cred *core.Credential, config MessageConfig, logChan chan<- string) (*core.Result, error) {
	startTime := time.Now()

	// 初始化 HTTP 客户端
	if s.client == nil {
		s.client = &http.Client{Timeout: 30 * time.Second}
	}

	// 1. 解析凭证字段（webhook 类型）
	webhook := cred.MustGetString("webhook")

	logChan <- "🤖 企业微信群机器人"
	logChan <- fmt.Sprintf("🔗 Webhook: %s", maskWebhook(webhook))

	// 2. 构建消息体
	msgType := "text"
	if config.ContentType == "markdown" {
		msgType = "markdown"
	}

	message := map[string]interface{}{
		"msgtype": msgType,
	}

	// 设置消息内容
	if msgType == "markdown" {
		// Markdown 消息
		markdownContent := map[string]interface{}{
			"content": config.Content,
		}

		// ⚠️  企业微信限制：Markdown 消息不支持 @人 功能
		// 官方文档：https://developer.work.weixin.qq.com/document/path/91770
		// 只有 text 消息才支持 mentioned_list 和 mentioned_mobile_list
		if config.IsAtAll || len(config.AtUserIds) > 0 || len(config.AtMobiles) > 0 {
			logChan <- "⚠️  警告：Markdown 消息不支持 @人 功能，@设置将被忽略"
			logChan <- "💡 建议：如需 @人，请改用 Text 消息类型"
		}

		message["markdown"] = markdownContent
	} else {
		// Text 消息
		textContent := map[string]interface{}{
			"content": config.Content,
		}

		// 添加 @人 功能
		// 🔥 优先级：@所有人 > @指定用户 > @指定手机号
		if config.IsAtAll {
			// @所有人
			textContent["mentioned_list"] = []string{"@all"}
			logChan <- "👥 @所有人"
		} else {
			// @指定用户
			if len(config.AtUserIds) > 0 {
				textContent["mentioned_list"] = config.AtUserIds
				logChan <- fmt.Sprintf("👥 @用户: %v", config.AtUserIds)
			}
			// @指定手机号
			if len(config.AtMobiles) > 0 {
				textContent["mentioned_mobile_list"] = config.AtMobiles
				logChan <- fmt.Sprintf("📱 @手机: %v", config.AtMobiles)
			}
		}

		message["text"] = textContent
	}

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
	errcode, _ := result["errcode"].(float64)
	if errcode != 0 {
		errmsg, _ := result["errmsg"].(string)
		return nil, fmt.Errorf("企业微信API错误 (errcode=%d): %s", int(errcode), errmsg)
	}

	endTime := time.Now()
	duration := endTime.Sub(startTime)

	logChan <- fmt.Sprintf("✅ 企业微信群机器人消息发送成功（耗时: %v）", duration)

	// 5. 构建返回结果
	output := fmt.Sprintf("企业微信群机器人消息已发送\n消息类型: %s\n内容长度: %d 字符",
		msgType,
		len(config.Content))

	return &core.Result{
		Status:    core.StatusSuccess,
		Output:    output,
		StartTime: startTime,
		EndTime:   endTime,
	}, nil
}

// firstNonEmpty 返回第一个非空字符串
func firstNonEmpty(strs ...string) string {
	for _, s := range strs {
		if s != "" {
			return s
		}
	}
	return ""
}

// maskWebhook 脱敏 webhook 地址（只显示前后部分）
func maskWebhook(webhook string) string {
	if len(webhook) <= 50 {
		return webhook
	}
	return webhook[:30] + "..." + webhook[len(webhook)-10:]
}
