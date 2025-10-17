// Package core 核心数据模型和接口定义
//
// 包含系统中所有核心业务实体的数据模型定义
// 以及相关的数据访问接口和服务接口
package core

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/codelieche/cronjob/apiserver/pkg/utils/filters"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Workflow 工作流模板实体
//
// 定义了一个工作流模板的所有属性，包括：
// - 基本信息：名称、Code、描述、项目归属等
// - 步骤信息：编排的步骤列表（JSON格式）
// - 元数据信息：执行环境、Worker配置等
// - 统计信息：执行次数、成功/失败次数等
//
// Workflow 是一组 Task 的模板，定义了任务的执行顺序和初始参数
// 每次执行 Workflow 会创建一个 WorkflowExecute 实例
type Workflow struct {
	ID               uuid.UUID       `gorm:"size:256;primaryKey" json:"id"`                                   // 工作流唯一标识
	TeamID           *uuid.UUID      `gorm:"size:256;index:idx_workflow_team_code,priority:1" json:"team_id"` // 团队ID，用于多租户隔离（联合唯一索引：team_id+code）
	Project          string          `gorm:"size:128;index;default:default" json:"project"`                   // 所属项目，用于工作流分组管理
	Code             string          `gorm:"size:128;index:idx_workflow_team_code,priority:2" json:"code"`    // 工作流代码（英文），用于URL路由和快捷访问（联合唯一索引：team_id+code）
	Name             string          `gorm:"size:256" json:"name"`                                            // 工作流名称（友好名称）
	Description      string          `gorm:"size:512" json:"description"`                                     // 工作流描述
	Steps            json.RawMessage `gorm:"type:json" json:"steps" swaggertype:"array,object"`               // 步骤列表（JSON数组），定义工作流的执行步骤
	DefaultVariables json.RawMessage `gorm:"type:json" json:"default_variables" swaggertype:"object"`         // 默认变量（JSON对象），执行时的默认参数值，可被 initial_variables 覆盖
	Metadata         json.RawMessage `gorm:"type:json" json:"metadata" swaggertype:"object"`                  // 元数据配置，存储执行环境、Worker配置等
	IsActive         *bool           `gorm:"type:boolean;default:true" json:"is_active"`                      // 是否激活，用于控制是否可以执行
	Timeout          int             `gorm:"type:int;default:0" json:"timeout"`                               // 工作流整体超时时间（秒），0表示使用默认值（24小时）

	// 统计信息（冗余字段，提升查询性能）
	ExecuteCount  int        `gorm:"type:int;default:0" json:"execute_count"`       // 执行次数
	SuccessCount  int        `gorm:"type:int;default:0" json:"success_count"`       // 成功次数
	FailedCount   int        `gorm:"type:int;default:0" json:"failed_count"`        // 失败次数
	LastExecuteAt *time.Time `gorm:"column:last_execute_at" json:"last_execute_at"` // 最后执行时间
	LastStatus    string     `gorm:"size:40" json:"last_status"`                    // 最后执行状态

	// 🔥 Webhook 触发配置
	WebhookEnabled     *bool           `gorm:"type:boolean;default:false;index:idx_workflow_team_webhook" json:"webhook_enabled"` // Webhook是否启用（使用指针类型，便于区分false和未设置）
	WebhookToken       *string         `gorm:"size:256;default:null" json:"webhook_token,omitempty"`                              // Webhook认证Token（🔥加密存储，使用指针类型，未设置时为NULL）
	WebhookIPWhitelist json.RawMessage `gorm:"type:json" json:"webhook_ip_whitelist,omitempty" swaggertype:"array,string"`        // IP白名单（JSON数组），空表示允许所有IP
	WebhookMetadata    json.RawMessage `gorm:"type:json" json:"webhook_metadata,omitempty" swaggertype:"object"`                  // Webhook元数据（可选配置）
	WebhookURL         string          `gorm:"-" json:"webhook_url,omitempty"`                                                    // Webhook URL（只读字段，动态生成，不存储到数据库）

	// 时间戳字段
	CreatedAt time.Time      `gorm:"column:created_at;autoCreateTime" json:"created_at"` // 创建时间
	UpdatedAt time.Time      `gorm:"column:updated_at;autoUpdateTime" json:"updated_at"` // 更新时间
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`                                     // 软删除时间
	Deleted   *bool          `gorm:"type:boolean;default:false" json:"deleted"`          // 软删除标记
}

// TableName 返回数据库表名
func (Workflow) TableName() string {
	return "workflows"
}

// WorkflowStep 工作流步骤定义
//
// 定义了工作流中的单个步骤，包括：
// - 基本信息：名称、描述、执行顺序
// - 执行信息：Category（Runner类型）、Args（参数）
// - 超时配置：Timeout
// - 🔥 条件分支：Condition（条件表达式）
// - 🔥 并行执行：ParallelGroup、WaitStrategy、FailureStrategy
type WorkflowStep struct {
	// ========== 现有字段 ==========
	Order       int                    `json:"order"`                 // 步骤顺序（从1开始）
	Name        string                 `json:"name"`                  // 步骤名称
	Description string                 `json:"description,omitempty"` // 步骤描述（可选）
	Category    string                 `json:"category"`              // 任务分类（对应Task的Category，如：git/script/container）
	Args        map[string]interface{} `json:"args"`                  // 任务参数（JSON对象，支持 ${variable} 模板替换）
	Timeout     int                    `json:"timeout"`               // 超时时间（秒），0表示使用 Workflow.Timeout

	// ========== 🔥 新增字段：条件分支和并行执行 ==========

	// Condition 条件表达式（可选）
	// - 空字符串：无条件执行（默认）
	// - "success"：只在上一步成功时执行
	// - "failed"：只在上一步业务失败时执行
	// - "error"：只在上一步系统错误时执行
	// - "timeout"：只在上一步超时时执行
	// - "task_status == 'success'"：表达式条件（完整写法）
	// - "exit_code == 0 && deploy_env == 'production'"：复杂表达式
	//
	// 表达式中可以访问：
	// - Variables: 工作流变量（如 deploy_env, branch）
	// - task_status: 上一步的详细状态（success/failed/error/timeout/stopped/canceled）
	// - output: 上一步的输出（如 output.code, output.status）
	Condition string `json:"condition,omitempty"`

	// ParallelGroup 并行组 ID（可选）
	// - 空字符串：顺序执行（默认）
	// - "group_1"：并行组 ID，相同值的步骤并行执行
	// - 并行组内的所有 Task 都完成后，才执行下一批
	//
	// 注意：
	// 1. 并行组内的步骤会同时激活（Status: todo → pending）
	// 2. Worker 会并发执行这些 pending 的任务
	// 3. 通过 WaitStrategy 控制何时继续下一步
	ParallelGroup string `json:"parallel_group,omitempty"`

	// WaitStrategy 等待策略（并行执行时有效）
	// - "all"：等待所有并行任务完成（默认）
	// - "any"：任意一个完成即可
	// - "threshold:N"：完成 N 个即可（如 "threshold:2"）
	//
	// 注意：只有达到等待策略后，才会激活下一批任务
	WaitStrategy string `json:"wait_strategy,omitempty"`

	// FailureStrategy 失败策略（并行执行时有效）
	// - "continue"：某个任务失败，其他继续（默认）
	// - "abort"：某个任务失败，立即终止所有并行任务和工作流
	//
	// 注意：
	// 1. "continue" 模式下，即使有任务失败，也会等待其他任务完成
	// 2. "abort" 模式下，一旦有任务失败，立即终止整个工作流
	FailureStrategy string `json:"failure_strategy,omitempty"`
}

// GetSteps 获取解析后的步骤列表
//
// 将JSON格式的Steps字段解析为WorkflowStep数组
//
// 返回：
//   - 解析后的步骤列表
//   - 解析错误（如果有）
func (w *Workflow) GetSteps() ([]WorkflowStep, error) {
	if len(w.Steps) == 0 {
		return []WorkflowStep{}, nil
	}

	var steps []WorkflowStep
	if err := json.Unmarshal(w.Steps, &steps); err != nil {
		return nil, err
	}
	return steps, nil
}

// SetSteps 设置步骤列表
//
// 将WorkflowStep数组序列化为JSON并存储到Steps字段
//
// 参数：
//   - steps: 步骤列表
//
// 返回：
//   - 序列化错误（如果有）
func (w *Workflow) SetSteps(steps []WorkflowStep) error {
	data, err := json.Marshal(steps)
	if err != nil {
		return err
	}
	w.Steps = data
	return nil
}

// GetDefaultVariables 获取解析后的默认变量
//
// 将JSON格式的DefaultVariables字段解析为map
//
// 返回：
//   - 解析后的默认变量
//   - 解析错误（如果有）
func (w *Workflow) GetDefaultVariables() (map[string]interface{}, error) {
	if len(w.DefaultVariables) == 0 {
		return make(map[string]interface{}), nil
	}

	var variables map[string]interface{}
	if err := json.Unmarshal(w.DefaultVariables, &variables); err != nil {
		return nil, err
	}
	return variables, nil
}

// SetDefaultVariables 设置默认变量
//
// 将map序列化为JSON并存储到DefaultVariables字段
//
// 参数：
//   - variables: 默认变量
//
// 返回：
//   - 序列化错误（如果有）
func (w *Workflow) SetDefaultVariables(variables map[string]interface{}) error {
	if variables == nil {
		variables = make(map[string]interface{})
	}
	data, err := json.Marshal(variables)
	if err != nil {
		return err
	}
	w.DefaultVariables = data
	return nil
}

// GetMetadata 获取解析后的元数据
//
// 将JSON格式的Metadata字段解析为Metadata结构体
// 使用统一的 Metadata 结构
//
// 返回：
//   - 解析后的 Metadata 结构体
//   - 解析错误（如果有）
func (w *Workflow) GetMetadata() (*Metadata, error) {
	return ParseMetadata(w.Metadata)
}

// SetMetadata 设置元数据
//
// 将 Metadata 结构体序列化为JSON并存储到Metadata字段
//
// 参数：
//   - metadata: Metadata 结构体
//
// 返回：
//   - 序列化错误（如果有）
func (w *Workflow) SetMetadata(metadata *Metadata) error {
	data, err := SerializeMetadata(metadata)
	if err != nil {
		return err
	}
	w.Metadata = data
	return nil
}

// ========== 🔥 Webhook 相关方法 ==========

// GetWebhookURL 获取Webhook触发URL（动态生成）
//
// 根据配置的baseURL和当前Workflow的ID、Token生成完整的Webhook触发URL
// 采用查询参数方式传递Token，符合业界标准（GitHub/GitLab/钉钉等）
//
// 参数：
//   - baseURL: API服务器的基础URL（如：https://api.example.com）
//
// 返回：
//   - Webhook URL，如果Webhook未启用或Token为空则返回空字符串
//
// 示例：
//
//	workflow.GetWebhookURL("https://api.example.com")
//	=> "https://api.example.com/api/v1/workflow/uuid-xxx/webhook?key=token-abc123"
func (w *Workflow) GetWebhookURL(baseURL string) string {
	// 检查Webhook是否启用
	if w.WebhookEnabled == nil || !*w.WebhookEnabled {
		return ""
	}

	// 检查Token是否存在
	if w.WebhookToken == nil || *w.WebhookToken == "" {
		return ""
	}

	// 🔥 使用查询参数方式，符合业界标准
	// 格式：/api/v1/workflow/{id}/webhook?key={token}
	return fmt.Sprintf("%s/api/v1/workflow/%s/webhook?key=%s", baseURL, w.ID, *w.WebhookToken)
}

// RegenerateWebhookToken 重新生成Webhook Token
//
// 生成一个新的32字符随机Token，用于Webhook认证
// 使用 crypto/rand 生成安全的随机字符串
//
// 返回：
//   - 生成错误（如果有）
func (w *Workflow) RegenerateWebhookToken() error {
	token, err := GenerateSecureToken(32)
	if err != nil {
		return fmt.Errorf("生成Webhook Token失败: %w", err)
	}
	w.WebhookToken = &token
	return nil
}

// GetWebhookIPWhitelist 获取IP白名单列表
//
// 将JSON格式的WebhookIPWhitelist字段解析为字符串数组
//
// 返回：
//   - IP白名单列表，如果未配置则返回空数组
//   - 解析错误（如果有）
func (w *Workflow) GetWebhookIPWhitelist() ([]string, error) {
	// 如果字段为空，返回空数组
	if len(w.WebhookIPWhitelist) == 0 {
		return []string{}, nil
	}

	var whitelist []string
	if err := json.Unmarshal(w.WebhookIPWhitelist, &whitelist); err != nil {
		return nil, fmt.Errorf("解析IP白名单失败: %w", err)
	}

	return whitelist, nil
}

// SetWebhookIPWhitelist 设置IP白名单列表
//
// 将字符串数组序列化为JSON并存储到WebhookIPWhitelist字段
//
// 参数：
//   - whitelist: IP白名单列表（支持单个IP和CIDR格式）
//
// 返回：
//   - 序列化错误（如果有）
func (w *Workflow) SetWebhookIPWhitelist(whitelist []string) error {
	if whitelist == nil {
		whitelist = []string{}
	}

	data, err := json.Marshal(whitelist)
	if err != nil {
		return fmt.Errorf("序列化IP白名单失败: %w", err)
	}

	w.WebhookIPWhitelist = data
	return nil
}

// IsIPAllowed 检查IP是否在白名单中
//
// 如果未配置白名单（空数组），则允许所有IP
// 支持精确匹配和CIDR格式匹配
//
// 参数：
//   - ip: 客户端IP地址
//
// 返回：
//   - true: IP在白名单中或未配置白名单
//   - false: IP不在白名单中
func (w *Workflow) IsIPAllowed(ip string) bool {
	// 获取IP白名单
	whitelist, err := w.GetWebhookIPWhitelist()
	if err != nil {
		// 解析失败，默认允许（安全起见应该拒绝，但为了向后兼容暂时允许）
		return true
	}

	// 未配置白名单，允许所有IP
	if len(whitelist) == 0 {
		return true
	}

	// 检查IP是否匹配
	return CheckIPInWhitelist(ip, whitelist)
}

// BeforeCreate GORM钩子：创建前的处理
func (w *Workflow) BeforeCreate(tx *gorm.DB) error {
	// 1. 设置ID
	if w.ID == uuid.Nil {
		w.ID = uuid.New()
	}

	// 2. 设置默认值
	if w.IsActive == nil {
		trueValue := true
		w.IsActive = &trueValue
	}

	// 3. 设置统计信息初始值
	w.ExecuteCount = 0
	w.SuccessCount = 0
	w.FailedCount = 0

	return nil
}

// BeforeDelete 删除前设置deleted字段为True
func (w *Workflow) BeforeDelete(tx *gorm.DB) error {
	// 设置Deleted字段为true
	trueValue := true
	w.Deleted = &trueValue
	return nil
}

// WorkflowStore 工作流数据存储接口
//
// 定义了工作流的所有数据访问操作
type WorkflowStore interface {
	// Create 创建工作流
	Create(ctx context.Context, workflow *Workflow) error

	// Update 更新工作流
	Update(ctx context.Context, workflow *Workflow) error

	// Delete 删除工作流（软删除）
	Delete(ctx context.Context, id uuid.UUID) error

	// FindByID 根据ID查询工作流
	FindByID(ctx context.Context, id uuid.UUID) (*Workflow, error)

	// FindByCode 根据Code查询工作流（团队内唯一）
	FindByCode(ctx context.Context, teamID uuid.UUID, code string) (*Workflow, error)

	// List 查询工作流列表
	// 支持过滤条件：team_id、project、is_active、search（名称/描述）
	List(ctx context.Context, offset, limit int, filterActions ...filters.Filter) ([]*Workflow, error)

	// Count 统计工作流数量
	Count(ctx context.Context, filterActions ...filters.Filter) (int64, error)

	// UpdateStats 更新统计信息
	// 在WorkflowExecute完成后调用，更新执行次数和最后执行状态
	UpdateStats(ctx context.Context, id uuid.UUID, status string) error
}

// WorkflowService 工作流服务接口
//
// 定义了工作流的所有业务逻辑操作
type WorkflowService interface {
	// Create 创建工作流
	Create(ctx context.Context, workflow *Workflow) error

	// Update 更新工作流
	Update(ctx context.Context, workflow *Workflow) error

	// Delete 删除工作流
	Delete(ctx context.Context, id string) error

	// FindByID 根据ID查询工作流
	FindByID(ctx context.Context, id string) (*Workflow, error)

	// FindByCode 根据Code查询工作流
	FindByCode(ctx context.Context, teamID uuid.UUID, code string) (*Workflow, error)

	// List 查询工作流列表
	List(ctx context.Context, offset, limit int, actions ...filters.Filter) ([]*Workflow, error)

	// Count 统计工作流数量
	Count(ctx context.Context, actions ...filters.Filter) (int64, error)

	// ToggleActive 切换激活状态
	ToggleActive(ctx context.Context, id string) (*Workflow, error)

	// GetStatistics 获取工作流统计信息
	GetStatistics(ctx context.Context, id string) (map[string]interface{}, error)

	// ========== 🔥 Webhook 相关接口 ==========

	// FindByWebhookToken 根据Webhook Token验证并获取工作流
	// 🔥 通过workflow ID查询，然后解密token进行对比
	// 用于Webhook触发时验证Token并获取工作流信息
	FindByWebhookToken(ctx context.Context, id, token string) (*Workflow, error)

	// EnableWebhook 启用Webhook触发
	// 如果Token不存在会自动生成
	// 返回值：工作流对象、原始Token（仅首次生成时返回，否则为空字符串）、错误
	EnableWebhook(ctx context.Context, id string) (*Workflow, string, error)

	// DisableWebhook 禁用Webhook触发
	DisableWebhook(ctx context.Context, id string) (*Workflow, error)

	// RegenerateWebhookToken 重新生成Webhook Token
	// 返回新生成的Token
	RegenerateWebhookToken(ctx context.Context, id string) (string, error)

	// UpdateWebhookIPWhitelist 更新Webhook IP白名单
	UpdateWebhookIPWhitelist(ctx context.Context, id string, whitelist []string) error

	// DecryptWebhookToken 解密工作流的Webhook Token
	// 用于获取完整Webhook URL时解密Token
	DecryptWebhookToken(ctx context.Context, id string) (string, error)

	// CreateWebhookCronJob 一键创建Webhook定时任务
	// 自动完成以下步骤：
	// 1. 确保Webhook已启用，获取webhook_url
	// 2. 创建Credential存储webhook_url（加密存储）
	// 3. 创建CronJob使用该凭证定期触发
	//
	// 参数：
	//   - ctx: 上下文
	//   - id: 工作流ID
	//   - baseURL: API服务器的基础URL（如"http://localhost:8000"，从请求中获取）
	//   - time: cron时间表达式（7段格式，可选，默认"0 0 0 1 * * *"每月1号0点）
	//   - credentialName: 凭证名称（可选，默认"{workflow.name}:webhook"）
	//   - cronJobName: 定时任务名称（可选，默认"{workflow.name}:计划任务"）
	//   - description: 定时任务描述（可选）
	//   - isActive: 是否立即激活（默认false，建议先检查配置）
	//
	// 返回：
	//   - credential: 创建的凭证对象
	//   - cronJob: 创建的定时任务对象
	//   - error: 错误信息
	CreateWebhookCronJob(ctx context.Context, id, baseURL, time, credentialName, cronJobName, description string, isActive bool) (*Credential, *CronJob, error)
}

// ========== 🔥 Webhook 辅助函数 ==========

// GenerateSecureToken 生成安全的随机Token
//
// 使用 crypto/rand 生成指定长度的随机Token
// Token由 [a-zA-Z0-9] 字符组成
//
// 参数：
//   - length: Token长度
//
// 返回：
//   - 生成的Token字符串
//   - 错误信息
//
// 示例：
//
//	token, err := GenerateSecureToken(32)
//	=> "aB3dE5fG7hI9jK1lM3nO5pQ7rS9tU1vW"
func GenerateSecureToken(length int) (string, error) {
	// 字符集：大小写字母和数字
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

	// 生成随机字节
	randomBytes := make([]byte, length)
	_, err := rand.Read(randomBytes)
	if err != nil {
		return "", fmt.Errorf("生成随机字节失败: %w", err)
	}

	// 将随机字节映射到字符集
	token := make([]byte, length)
	for i := 0; i < length; i++ {
		token[i] = charset[int(randomBytes[i])%len(charset)]
	}

	return string(token), nil
}

// CheckIPInWhitelist 检查IP是否在白名单中
//
// 支持以下格式：
//   - 精确IP匹配：192.168.1.100
//   - CIDR格式：192.168.0.0/16, 10.0.0.0/8
//
// 参数：
//   - ip: 待检查的IP地址
//   - whitelist: IP白名单列表
//
// 返回：
//   - true: IP在白名单中
//   - false: IP不在白名单中
//
// 示例：
//
//	CheckIPInWhitelist("192.168.1.100", []string{"192.168.1.100", "10.0.0.0/8"})
//	=> true
func CheckIPInWhitelist(ip string, whitelist []string) bool {
	// 解析客户端IP
	clientIP := net.ParseIP(ip)
	if clientIP == nil {
		// IP格式无效，拒绝访问
		return false
	}

	// 遍历白名单
	for _, allowedEntry := range whitelist {
		// 检查是否为CIDR格式
		if strings.Contains(allowedEntry, "/") {
			// CIDR格式匹配
			_, subnet, err := net.ParseCIDR(allowedEntry)
			if err != nil {
				// CIDR格式无效，跳过
				continue
			}

			// 检查IP是否在子网中
			if subnet.Contains(clientIP) {
				return true
			}
		} else {
			// 精确IP匹配
			allowedIP := net.ParseIP(allowedEntry)
			if allowedIP != nil && allowedIP.Equal(clientIP) {
				return true
			}
		}
	}

	// IP不在白名单中
	return false
}
