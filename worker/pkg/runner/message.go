package runner

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/codelieche/cronjob/worker/pkg/core"
)

// MessageRunner 消息发送 Runner
//
// 支持多种消息发送渠道：
// - email: SMTP 邮件
// - wechat_work: 企业微信应用消息
// - wechat_work_bot: 企业微信群机器人
// - feishu_bot: 飞书群机器人
type MessageRunner struct {
	task      *core.Task
	config    MessageConfig
	apiserver core.Apiserver // API Server 客户端（用于获取凭证）
	startTime time.Time
	endTime   time.Time
	status    core.Status
	result    *core.Result
	cancel    context.CancelFunc
	mutex     sync.RWMutex // 保护并发访问
}

// MessageConfig 消息配置
type MessageConfig struct {
	Type         string   `json:"type"`          // 消息类型：email/wechat_work/wechat_work_bot/feishu_bot
	CredentialID string   `json:"credential_id"` // 凭证ID（引用凭证管理）
	To           []string `json:"to"`            // 接收人列表（邮件地址或用户ID）
	Subject      string   `json:"subject"`       // 邮件主题（email专用）
	Content      string   `json:"content"`       // 消息内容
	ContentType  string   `json:"content_type"`  // 内容类型：text/markdown/html（默认text）

	// 企业微信应用专用
	ToUser  string `json:"to_user,omitempty"`  // 成员ID列表（用|分隔）
	ToParty string `json:"to_party,omitempty"` // 部门ID列表（用|分隔）
	ToTag   string `json:"to_tag,omitempty"`   // 标签ID列表（用|分隔）

	// 机器人 @人 专用
	AtMobiles []string `json:"at_mobiles,omitempty"`  // @的手机号列表
	AtUserIds []string `json:"at_user_ids,omitempty"` // @的用户ID列表
	IsAtAll   bool     `json:"is_at_all,omitempty"`   // 是否@所有人
}

// MessageSender 消息发送器接口
//
// 不同的消息类型实现各自的发送逻辑
type MessageSender interface {
	// Send 发送消息
	//
	// 参数：
	//   - ctx: 上下文（支持超时和取消）
	//   - cred: 凭证信息（已解密）
	//   - config: 消息配置
	//   - logChan: 日志通道
	//
	// 返回：
	//   - *core.Result: 执行结果
	//   - error: 错误信息
	Send(ctx context.Context, cred *core.Credential, config MessageConfig, logChan chan<- string) (*core.Result, error)
}

// NewMessageRunner 创建新的 MessageRunner
func NewMessageRunner() *MessageRunner {
	return &MessageRunner{
		status: core.StatusPending,
	}
}

// ParseArgs 解析任务参数
func (r *MessageRunner) ParseArgs(task *core.Task) error {
	r.task = task

	// 解析 args（JSON 字符串）
	if err := json.Unmarshal([]byte(task.Args), &r.config); err != nil {
		return fmt.Errorf("解析消息配置失败: %w", err)
	}

	// 验证必填字段
	if r.config.Type == "" {
		return fmt.Errorf("消息类型（type）不能为空")
	}

	if r.config.CredentialID == "" {
		return fmt.Errorf("凭证ID（credential_id）不能为空")
	}

	if r.config.Content == "" {
		return fmt.Errorf("消息内容（content）不能为空")
	}

	// 设置默认内容类型
	if r.config.ContentType == "" {
		r.config.ContentType = "text"
	}

	// 根据消息类型验证特定字段
	switch r.config.Type {
	case "email":
		if len(r.config.To) == 0 {
			return fmt.Errorf("邮件接收人（to）不能为空")
		}
		if r.config.Subject == "" {
			return fmt.Errorf("邮件主题（subject）不能为空")
		}
	case "wechat_work":
		// 企业微信应用至少需要一个接收目标
		if r.config.ToUser == "" && r.config.ToParty == "" && r.config.ToTag == "" {
			return fmt.Errorf("企业微信应用消息至少需要指定一个接收目标（to_user/to_party/to_tag）")
		}
	case "wechat_work_bot", "feishu_bot":
		// 机器人类型不需要额外验证
	default:
		return fmt.Errorf("不支持的消息类型: %s", r.config.Type)
	}

	return nil
}

// Execute 执行消息发送
func (r *MessageRunner) Execute(ctx context.Context, logChan chan<- string) (*core.Result, error) {
	// 创建可取消的上下文
	ctx, cancel := context.WithCancel(ctx)
	r.cancel = cancel
	defer cancel()

	r.startTime = time.Now()
	r.status = core.StatusRunning

	// 构建初始结果
	r.result = &core.Result{
		Status:    core.StatusRunning,
		StartTime: r.startTime,
	}

	logChan <- fmt.Sprintf("📤 开始发送 %s 消息\n", r.getTypeLabel())
	logChan <- fmt.Sprintf("📋 消息内容长度: %d 字符\n", len(r.config.Content))

	// 1. 检查 apiserver 是否已注入
	if r.apiserver == nil {
		err := fmt.Errorf("apiserver 未初始化，无法获取凭证")
		logChan <- fmt.Sprintf("❌ %v\n", err)
		return r.buildErrorResult("内部错误", err), err
	}

	// 2. 获取凭证
	logChan <- "🔐 获取凭证...\n"
	logChan <- fmt.Sprintf("🔑 凭证ID: %s\n", r.config.CredentialID)
	cred, err := r.apiserver.GetCredential(r.config.CredentialID)
	if err != nil {
		logChan <- fmt.Sprintf("❌ 获取凭证失败: %v\n", err)
		return r.buildErrorResult("获取凭证失败", err), err
	}
	logChan <- fmt.Sprintf("✅ 成功获取凭证: %s (类型: %s)\n", cred.Name, cred.Category)

	// 2. 验证凭证类型
	expectedCategory := r.getExpectedCredentialCategory()
	if cred.Category != expectedCategory {
		err := fmt.Errorf("凭证类型不匹配：期望 %s，实际 %s\n", expectedCategory, cred.Category)
		logChan <- fmt.Sprintf("❌ %v", err)
		return r.buildErrorResult("凭证类型错误", err), err
	}

	// 3. 根据类型选择发送器
	sender, err := r.getSender()
	if err != nil {
		logChan <- fmt.Sprintf("❌ %v", err)
		return r.buildErrorResult("消息类型错误", err), err
	}

	// 4. 发送消息
	logChan <- "📨 正在发送消息...\n"
	result, err := sender.Send(ctx, cred, r.config, logChan)
	if err != nil {
		logChan <- fmt.Sprintf("❌ 发送失败: %v\n", err)
		return r.buildErrorResult("消息发送失败", err), err
	}

	// 5. 更新结果
	r.endTime = time.Now()
	result.EndTime = r.endTime
	result.Status = core.StatusSuccess
	r.status = core.StatusSuccess
	r.result = result

	logChan <- fmt.Sprintf("✅ 消息发送成功（耗时: %v）\n", r.endTime.Sub(r.startTime))

	return result, nil
}

// Stop 停止任务
func (r *MessageRunner) Stop() error {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	if r.cancel != nil {
		r.cancel()
		r.status = core.StatusStopped
	}
	return nil
}

// Kill 强制终止任务（对于消息发送，Stop和Kill效果相同）
func (r *MessageRunner) Kill() error {
	return r.Stop()
}

// GetStatus 获取任务状态
func (r *MessageRunner) GetStatus() core.Status {
	r.mutex.RLock()
	defer r.mutex.RUnlock()
	return r.status
}

// GetResult 获取执行结果
func (r *MessageRunner) GetResult() *core.Result {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	if r.result == nil {
		return &core.Result{
			Status:    r.status,
			StartTime: r.startTime,
			EndTime:   r.endTime,
		}
	}
	return r.result
}

// Cleanup 清理资源
func (r *MessageRunner) Cleanup() error {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	if r.cancel != nil {
		r.cancel()
	}

	r.status = core.StatusPending
	r.result = nil

	return nil
}

// SetApiserver 设置API Server客户端（依赖注入）
func (r *MessageRunner) SetApiserver(apiserver core.Apiserver) {
	r.apiserver = apiserver
}

// getSender 根据消息类型获取对应的发送器
func (r *MessageRunner) getSender() (MessageSender, error) {
	switch r.config.Type {
	case "email":
		return &EmailSender{}, nil
	case "wechat_work":
		return &WechatWorkSender{}, nil
	case "wechat_work_bot":
		return &WechatWorkBotSender{}, nil
	case "feishu_bot":
		return &FeishuBotSender{}, nil
	default:
		return nil, fmt.Errorf("不支持的消息类型: %s", r.config.Type)
	}
}

// getExpectedCredentialCategory 获取期望的凭证类型
func (r *MessageRunner) getExpectedCredentialCategory() string {
	switch r.config.Type {
	case "email":
		return "email"
	case "wechat_work":
		return "wechat_work"
	case "wechat_work_bot", "feishu_bot":
		return "webhook"
	default:
		return ""
	}
}

// getTypeLabel 获取消息类型的显示名称
func (r *MessageRunner) getTypeLabel() string {
	switch r.config.Type {
	case "email":
		return "邮件"
	case "wechat_work":
		return "企业微信应用消息"
	case "wechat_work_bot":
		return "企业微信群机器人"
	case "feishu_bot":
		return "飞书群机器人"
	default:
		return r.config.Type
	}
}

// buildErrorResult 构建错误结果
func (r *MessageRunner) buildErrorResult(message string, err error) *core.Result {
	r.endTime = time.Now()
	r.status = core.StatusError

	output := fmt.Sprintf("%s: %v", message, err)

	return &core.Result{
		Status:    core.StatusError,
		Output:    output,
		StartTime: r.startTime,
		EndTime:   r.endTime,
	}
}

// 确保MessageRunner实现了Runner接口
var _ core.Runner = (*MessageRunner)(nil)
