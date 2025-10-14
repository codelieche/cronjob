package runner

import (
	"context"
	"crypto/tls"
	"fmt"
	"strings"
	"time"

	"github.com/codelieche/cronjob/worker/pkg/core"
	"gopkg.in/gomail.v2"
)

// EmailSender 邮件发送器
//
// 使用 SMTP 协议发送邮件，支持：
// - TLS 加密连接
// - HTML 和纯文本内容
// - 多个收件人
type EmailSender struct{}

// Send 发送邮件
func (s *EmailSender) Send(ctx context.Context, cred *core.Credential, config MessageConfig, logChan chan<- string) (*core.Result, error) {
	startTime := time.Now()

	// 1. 解析凭证字段（email 类型）
	smtpHost := cred.MustGetString("smtp_host")
	smtpPort := cred.MustGetInt("smtp_port")
	username := cred.MustGetString("username")
	password := cred.MustGetString("password")

	// 可选字段：发件人名称（默认使用邮箱地址）
	fromName, ok := cred.GetString("from_name")
	if !ok || fromName == "" {
		fromName = username
	}

	// use_tls 配置（仅用于非465端口）
	useTLS, ok := cred.GetBool("use_tls")
	if !ok {
		useTLS = true // 默认使用 TLS
	}

	logChan <- fmt.Sprintf("📧 SMTP服务器: %s:%d\n", smtpHost, smtpPort)
	logChan <- fmt.Sprintf("📤 发件人: %s <%s>\n", fromName, username)
	logChan <- fmt.Sprintf("📬 收件人: %s\n", strings.Join(config.To, ", "))

	// 2. 创建邮件
	m := gomail.NewMessage()

	// 设置发件人
	m.SetHeader("From", m.FormatAddress(username, fromName))

	// 设置收件人
	m.SetHeader("To", config.To...)

	// 设置主题
	m.SetHeader("Subject", config.Subject)

	// 设置邮件内容
	contentType := "text/plain"
	if config.ContentType == "html" {
		contentType = "text/html"
	}
	m.SetBody(contentType, config.Content)

	// 3. 创建 SMTP 拨号器
	// gomail.NewDialer 会根据端口自动设置 SSL：
	// - port == 465 → SSL = true（直接SSL连接）
	// - port != 465 → SSL = false（使用STARTTLS）
	d := gomail.NewDialer(smtpHost, smtpPort, username, password)

	// 配置 TLS（跳过证书验证以解决企业邮箱证书问题）
	if useTLS {
		d.TLSConfig = &tls.Config{InsecureSkipVerify: true}
		if smtpPort == 465 {
			logChan <- "🔒 加密方式: SSL (465端口)\n"
		} else {
			logChan <- "🔒 加密方式: STARTTLS\n"
		}
	} else {
		// 禁用所有加密
		d.SSL = false
		d.TLSConfig = nil
		logChan <- "⚠️  未使用加密（不推荐）\n"
	}

	// 4. 发送邮件（带超时控制）
	logChan <- "📨 正在连接 SMTP 服务器...\n"

	// 创建发送通道
	sendDone := make(chan error, 1)

	go func() {
		sendDone <- d.DialAndSend(m)
	}()

	// 等待发送完成或超时
	select {
	case err := <-sendDone:
		if err != nil {
			return nil, fmt.Errorf("SMTP发送失败: %w", err)
		}
	case <-ctx.Done():
		return nil, fmt.Errorf("邮件发送被取消或超时")
	}

	endTime := time.Now()
	duration := endTime.Sub(startTime)

	logChan <- fmt.Sprintf("✅ 邮件发送成功（耗时: %v）", duration)

	// 5. 构建返回结果
	output := fmt.Sprintf("邮件已发送\n收件人: %s\n主题: %s\n内容长度: %d 字符",
		strings.Join(config.To, ", "),
		config.Subject,
		len(config.Content))

	return &core.Result{
		Status:    core.StatusSuccess,
		Output:    output,
		StartTime: startTime,
		EndTime:   endTime,
	}, nil
}
