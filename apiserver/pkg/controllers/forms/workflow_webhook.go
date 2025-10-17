package forms

import (
	"fmt"
	"net"
	"strings"
)

// WebhookTriggerForm Webhook触发表单
//
// 通过HTTP请求触发工作流时的请求体
// 支持传递初始变量和元数据覆盖
type WebhookTriggerForm struct {
	// Variables 初始变量（可选）
	// 会与工作流的 DefaultVariables 合并
	// 相同key时，Variables的值优先级更高
	Variables map[string]interface{} `json:"variables" form:"variables"`

	// MetadataOverride 元数据覆盖（可选）
	// 可以覆盖工作流模板中的 Metadata 配置
	// 例如：临时修改 working_dir、环境变量等
	MetadataOverride map[string]interface{} `json:"metadata_override" form:"metadata_override"`
}

// Validate 验证表单
func (form *WebhookTriggerForm) Validate() error {
	// Variables 和 MetadataOverride 都是可选的，不需要验证
	return nil
}

// WebhookToggleForm Webhook启用/禁用表单
//
// 用于切换工作流的Webhook触发功能
type WebhookToggleForm struct {
	// Enabled Webhook是否启用（使用指针类型，避免false被当作零值）
	Enabled *bool `json:"enabled" form:"enabled" binding:"required"`
}

// Validate 验证表单
func (form *WebhookToggleForm) Validate() error {
	// 验证 enabled 不为 nil
	if form.Enabled == nil {
		return fmt.Errorf("enabled 字段不能为空")
	}
	return nil
}

// WebhookRegenerateTokenForm Webhook重新生成Token表单
//
// 用于重新生成Webhook Token（无需参数）
type WebhookRegenerateTokenForm struct {
	// 空表单，仅用于接口规范
}

// Validate 验证表单
func (form *WebhookRegenerateTokenForm) Validate() error {
	return nil
}

// WebhookIPWhitelistForm Webhook IP白名单配置表单
//
// 用于配置允许触发Webhook的IP地址列表
type WebhookIPWhitelistForm struct {
	// IPWhitelist IP白名单列表
	// 支持格式：
	//   - 单个IP：192.168.1.100
	//   - CIDR格式：192.168.0.0/16, 10.0.0.0/8
	// 空数组表示允许所有IP
	IPWhitelist []string `json:"ip_whitelist" form:"ip_whitelist" binding:"required"`
}

// Validate 验证表单
func (form *WebhookIPWhitelistForm) Validate() error {
	// 验证IP格式
	for _, ip := range form.IPWhitelist {
		// 去除空格
		ip = strings.TrimSpace(ip)
		if ip == "" {
			continue
		}

		// 检查是否为CIDR格式
		if strings.Contains(ip, "/") {
			// 验证CIDR格式
			if !isValidCIDR(ip) {
				return fmt.Errorf("无效的CIDR格式: %s", ip)
			}
		} else {
			// 验证IP格式
			if !isValidIP(ip) {
				return fmt.Errorf("无效的IP地址: %s", ip)
			}
		}
	}

	return nil
}

// isValidIP 验证IP地址格式
func isValidIP(ip string) bool {
	// 尝试解析IP地址
	parsedIP := net.ParseIP(ip)
	return parsedIP != nil
}

// isValidCIDR 验证CIDR格式
func isValidCIDR(cidr string) bool {
	// 尝试解析CIDR
	_, _, err := net.ParseCIDR(cidr)
	return err == nil
}

// CreateWebhookCronJobForm 一键创建Webhook定时任务表单
//
// 用于为工作流的Webhook创建定时任务
// 自动执行以下步骤：
// 1. 确保Webhook已启用，获取webhook_url
// 2. 创建Credential存储webhook_url
// 3. 创建CronJob使用该凭证
type CreateWebhookCronJobForm struct {
	// Time cron时间表达式（7段格式，可选）
	// 默认值："0 0 0 1 * * *"（每月1号0点0分0秒执行）
	// 格式：秒 分 时 日 月 星期 年
	// 示例：
	//   - "0 0 0 1 * * *"   - 每月1号0点
	//   - "0 0 0 * * * *"   - 每天0点
	//   - "0 0 */6 * * * *" - 每6小时
	//   - "0 0 0 * * 1 *"   - 每周一0点
	Time string `json:"time" form:"time"`

	// CredentialName 凭证名称（可选）
	// 默认值："{workflow.name}:webhook"
	CredentialName string `json:"credential_name" form:"credential_name"`

	// CronJobName 定时任务名称（可选）
	// 默认值："{workflow.name}:计划任务"
	CronJobName string `json:"cronjob_name" form:"cronjob_name"`

	// Description 定时任务描述（可选）
	Description string `json:"description" form:"description"`

	// IsActive 是否立即激活定时任务（可选）
	// 默认值：false（不激活，需要手动启用）
	// true: 创建后立即生效
	// false: 创建后不生效，需要手动启用（建议先检查配置）
	IsActive *bool `json:"is_active" form:"is_active"`
}

// Validate 验证表单
func (form *CreateWebhookCronJobForm) Validate() error {
	// Time字段是可选的，如果提供需要验证格式
	// 这里简单验证，不能为空白字符串
	if form.Time != "" {
		form.Time = strings.TrimSpace(form.Time)
		if form.Time == "" {
			return fmt.Errorf("time 不能为空白字符串")
		}
	}

	return nil
}

// GetDefaultTime 获取默认时间表达式（7段格式）
func (form *CreateWebhookCronJobForm) GetDefaultTime() string {
	if form.Time == "" {
		return "0 0 0 1 * * *" // 每月1号0点0分0秒
	}
	return form.Time
}

// GetDefaultIsActive 获取默认激活状态
func (form *CreateWebhookCronJobForm) GetDefaultIsActive() bool {
	if form.IsActive == nil {
		return false // 默认不激活，需要用户检查后手动启用
	}
	return *form.IsActive
}

// GetDefaultCredentialName 获取默认凭证名称
func (form *CreateWebhookCronJobForm) GetDefaultCredentialName(workflowName string) string {
	if form.CredentialName == "" {
		return workflowName + ":webhook"
	}
	return form.CredentialName
}

// GetDefaultCronJobName 获取默认定时任务名称
func (form *CreateWebhookCronJobForm) GetDefaultCronJobName(workflowName string) string {
	if form.CronJobName == "" {
		return workflowName + ":计划任务"
	}
	return form.CronJobName
}
