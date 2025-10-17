package services

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/codelieche/cronjob/apiserver/pkg/core"
	"github.com/codelieche/cronjob/apiserver/pkg/utils/filters"
	"github.com/codelieche/cronjob/apiserver/pkg/utils/logger"
	"github.com/codelieche/cronjob/apiserver/pkg/utils/tools"
	"github.com/codelieche/cronjob/apiserver/pkg/utils/types"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// NewWorkflowService 创建 WorkflowService 实例
func NewWorkflowService(store core.WorkflowStore) core.WorkflowService {
	return &WorkflowService{
		store:             store,
		crypto:            tools.NewCryptography(types.EncryptionKey),
		credentialService: nil, // 延迟注入
		cronJobService:    nil, // 延迟注入
	}
}

// WorkflowService 工作流服务实现
type WorkflowService struct {
	store             core.WorkflowStore
	crypto            *tools.Cryptography
	credentialService core.CredentialService // 凭证服务（用于创建Webhook凭证）
	cronJobService    core.CronJobService    // 定时任务服务（用于创建Webhook定时任务）
}

// SetCredentialService 设置凭证服务（依赖注入）
func (s *WorkflowService) SetCredentialService(credentialService core.CredentialService) {
	s.credentialService = credentialService
}

// SetCronJobService 设置定时任务服务（依赖注入）
func (s *WorkflowService) SetCronJobService(cronJobService core.CronJobService) {
	s.cronJobService = cronJobService
}

// FindByID 根据ID获取工作流
func (s *WorkflowService) FindByID(ctx context.Context, id string) (*core.Workflow, error) {
	// 解析UUID
	uuidID, err := uuid.Parse(id)
	if err != nil {
		logger.Error("parse workflow id error", zap.Error(err), zap.String("id", id))
		return nil, core.ErrBadRequest
	}

	return s.store.FindByID(ctx, uuidID)
}

// FindByCode 根据Code获取工作流（团队内唯一）
func (s *WorkflowService) FindByCode(ctx context.Context, teamID uuid.UUID, code string) (*core.Workflow, error) {
	if code == "" {
		logger.Error("workflow code is required")
		return nil, core.ErrBadRequest
	}

	workflow, err := s.store.FindByCode(ctx, teamID, code)
	if err != nil {
		if err != core.ErrNotFound {
			logger.Error("find workflow by code error", zap.Error(err), zap.String("code", code))
		}
	}
	return workflow, err
}

// Create 创建工作流
func (s *WorkflowService) Create(ctx context.Context, workflow *core.Workflow) error {
	// 验证参数
	if workflow.Name == "" {
		logger.Error("workflow name is required")
		return core.ErrBadRequest
	}

	if workflow.Code == "" {
		logger.Error("workflow code is required")
		return core.ErrBadRequest
	}

	// 验证步骤列表
	steps, err := workflow.GetSteps()
	if err != nil {
		logger.Error("parse workflow steps error", zap.Error(err))
		return core.ErrBadRequest
	}

	if len(steps) == 0 {
		logger.Error("workflow steps cannot be empty")
		return core.ErrBadRequest
	}

	// 验证步骤顺序和必需字段
	for i, step := range steps {
		if step.Order <= 0 {
			logger.Error("step order must be positive", zap.Int("index", i), zap.Int("order", step.Order))
			return core.ErrBadRequest
		}
		if step.Name == "" {
			logger.Error("step name is required", zap.Int("index", i))
			return core.ErrBadRequest
		}
		if step.Category == "" {
			logger.Error("step category is required", zap.Int("index", i))
			return core.ErrBadRequest
		}
	}

	// 检查 team_id + code 是否已存在
	if workflow.TeamID != nil && workflow.Code != "" {
		existingWorkflow, err := s.store.FindByCode(ctx, *workflow.TeamID, workflow.Code)
		if err == nil && existingWorkflow != nil {
			logger.Error("workflow code already exists",
				zap.String("code", workflow.Code),
				zap.String("team_id", workflow.TeamID.String()))
			return core.ErrConflict
		} else if err != core.ErrNotFound {
			return err
		}
	}

	// 如果指定了id，检查id是否已经存在
	if workflow.ID != uuid.Nil {
		_, err := s.FindByID(ctx, workflow.ID.String())
		if err == nil {
			logger.Error("workflow id already exists", zap.String("id", workflow.ID.String()))
			return core.ErrConflict
		} else if err != core.ErrNotFound {
			return err
		}
	} else {
		// 🔥 如果没有指定 ID，先生成一个新的 UUID（用于设置 WorkingDir）
		workflow.ID = uuid.New()
		logger.Debug("为 workflow 生成新的 UUID",
			zap.String("workflow_id", workflow.ID.String()),
			zap.String("name", workflow.Name))
	}

	// 设置默认值
	if workflow.Project == "" {
		workflow.Project = "default"
	}

	// ⭐ 设置默认 WorkingDir（如果 Metadata 中没有设置）
	metadata, err := workflow.GetMetadata()
	if err != nil {
		metadata = &core.Metadata{}
	}
	if metadata.WorkingDir == "" {
		// 默认工作目录：./workflow/:workflowID
		// 🔥 此时 workflow.ID 已经是有效的 UUID，不会是零值
		metadata.WorkingDir = fmt.Sprintf("./workflow/%s", workflow.ID.String())
		if err := workflow.SetMetadata(metadata); err != nil {
			logger.Error("设置默认 WorkingDir 失败", zap.Error(err))
			// 不影响创建流程，只记录日志
		}
		logger.Info("自动设置默认工作目录",
			zap.String("workflow_id", workflow.ID.String()),
			zap.String("working_dir", metadata.WorkingDir))
	}

	// 创建工作流
	if err := s.store.Create(ctx, workflow); err != nil {
		logger.Error("create workflow error", zap.Error(err))
		return err
	}

	logger.Info("workflow created successfully",
		zap.String("id", workflow.ID.String()),
		zap.String("code", workflow.Code),
		zap.String("name", workflow.Name))

	return nil
}

// Update 更新工作流
func (s *WorkflowService) Update(ctx context.Context, workflow *core.Workflow) error {
	if workflow.ID == uuid.Nil {
		logger.Error("workflow id is required")
		return core.ErrBadRequest
	}

	// 检查工作流是否存在
	existingWorkflow, err := s.FindByID(ctx, workflow.ID.String())
	if err != nil {
		return err
	}

	// 验证步骤列表（如果有更新）
	if len(workflow.Steps) > 0 {
		steps, err := workflow.GetSteps()
		if err != nil {
			logger.Error("parse workflow steps error", zap.Error(err))
			return core.ErrBadRequest
		}

		if len(steps) == 0 {
			logger.Error("workflow steps cannot be empty")
			return core.ErrBadRequest
		}

		// 验证步骤顺序和必需字段
		for i, step := range steps {
			if step.Order <= 0 {
				logger.Error("step order must be positive", zap.Int("index", i), zap.Int("order", step.Order))
				return core.ErrBadRequest
			}
			if step.Name == "" {
				logger.Error("step name is required", zap.Int("index", i))
				return core.ErrBadRequest
			}
			if step.Category == "" {
				logger.Error("step category is required", zap.Int("index", i))
				return core.ErrBadRequest
			}
		}
	}

	// 检查 Code 是否冲突
	if workflow.Code != "" && workflow.Code != existingWorkflow.Code {
		if workflow.TeamID != nil {
			existingByCode, err := s.store.FindByCode(ctx, *workflow.TeamID, workflow.Code)
			if err == nil && existingByCode != nil && existingByCode.ID != workflow.ID {
				logger.Error("workflow code already exists",
					zap.String("code", workflow.Code),
					zap.String("team_id", workflow.TeamID.String()))
				return core.ErrConflict
			} else if err != nil && err != core.ErrNotFound {
				return err
			}
		}
	}

	// 更新工作流
	if err := s.store.Update(ctx, workflow); err != nil {
		logger.Error("update workflow error", zap.Error(err), zap.String("id", workflow.ID.String()))
		return err
	}

	logger.Info("workflow updated successfully",
		zap.String("id", workflow.ID.String()),
		zap.String("name", workflow.Name))

	return nil
}

// Delete 删除工作流
func (s *WorkflowService) Delete(ctx context.Context, id string) error {
	// 解析UUID
	uuidID, err := uuid.Parse(id)
	if err != nil {
		logger.Error("parse workflow id error", zap.Error(err), zap.String("id", id))
		return core.ErrBadRequest
	}

	// 检查工作流是否存在
	workflow, err := s.store.FindByID(ctx, uuidID)
	if err != nil {
		return err
	}

	// 删除工作流
	if err := s.store.Delete(ctx, uuidID); err != nil {
		logger.Error("delete workflow error", zap.Error(err), zap.String("id", id))
		return err
	}

	logger.Info("workflow deleted successfully",
		zap.String("id", id),
		zap.String("name", workflow.Name))

	return nil
}

// List 查询工作流列表
func (s *WorkflowService) List(ctx context.Context, offset, limit int, actions ...filters.Filter) ([]*core.Workflow, error) {
	workflows, err := s.store.List(ctx, offset, limit, actions...)
	if err != nil {
		logger.Error("list workflows error", zap.Error(err), zap.Int("offset", offset), zap.Int("limit", limit))
		return nil, err
	}
	return workflows, nil
}

// Count 统计工作流数量
func (s *WorkflowService) Count(ctx context.Context, actions ...filters.Filter) (int64, error) {
	count, err := s.store.Count(ctx, actions...)
	if err != nil {
		logger.Error("count workflows error", zap.Error(err))
		return 0, err
	}
	return count, nil
}

// ToggleActive 切换激活状态
func (s *WorkflowService) ToggleActive(ctx context.Context, id string) (*core.Workflow, error) {
	// 解析UUID
	uuidID, err := uuid.Parse(id)
	if err != nil {
		logger.Error("parse workflow id error", zap.Error(err), zap.String("id", id))
		return nil, core.ErrBadRequest
	}

	// 获取工作流
	workflow, err := s.store.FindByID(ctx, uuidID)
	if err != nil {
		return nil, err
	}

	// 切换激活状态
	if workflow.IsActive == nil {
		falseValue := false
		workflow.IsActive = &falseValue
	} else {
		newValue := !*workflow.IsActive
		workflow.IsActive = &newValue
	}

	// 更新工作流
	if err := s.store.Update(ctx, workflow); err != nil {
		logger.Error("toggle workflow active error", zap.Error(err), zap.String("id", id))
		return nil, err
	}

	logger.Info("workflow active toggled",
		zap.String("id", id),
		zap.Bool("is_active", *workflow.IsActive))

	return workflow, nil
}

// GetStatistics 获取工作流统计信息
func (s *WorkflowService) GetStatistics(ctx context.Context, id string) (map[string]interface{}, error) {
	// 解析UUID
	uuidID, err := uuid.Parse(id)
	if err != nil {
		logger.Error("parse workflow id error", zap.Error(err), zap.String("id", id))
		return nil, core.ErrBadRequest
	}

	// 获取工作流
	workflow, err := s.store.FindByID(ctx, uuidID)
	if err != nil {
		return nil, err
	}

	// 构建统计信息
	stats := map[string]interface{}{
		"execute_count":   workflow.ExecuteCount,
		"success_count":   workflow.SuccessCount,
		"failed_count":    workflow.FailedCount,
		"last_execute_at": workflow.LastExecuteAt,
		"last_status":     workflow.LastStatus,
	}

	// 计算成功率
	if workflow.ExecuteCount > 0 {
		successRate := float64(workflow.SuccessCount) / float64(workflow.ExecuteCount) * 100
		stats["success_rate"] = fmt.Sprintf("%.2f%%", successRate)
	} else {
		stats["success_rate"] = "0.00%"
	}

	return stats, nil
}

// ========== 🔥 Webhook 相关方法 ==========

// FindByWebhookToken 根据Webhook Token验证并获取工作流
//
// 用于Webhook触发时验证Token并获取工作流信息
// 🔥 通过workflow ID查询，然后解密token进行对比
//
// 参数：
//   - ctx: 上下文
//   - id: 工作流ID
//   - token: Webhook Token（原始未加密）
//
// 返回：
//   - 工作流对象
//   - 错误信息
func (s *WorkflowService) FindByWebhookToken(ctx context.Context, id, token string) (*core.Workflow, error) {
	// 验证参数
	if id == "" || token == "" {
		logger.Error("workflow id 和 token 不能为空")
		return nil, core.ErrBadRequest
	}

	// 🔥 1. 通过ID查询workflow
	workflow, err := s.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}

	// 🔥 2. 检查Token是否存在
	if workflow.WebhookToken == nil || *workflow.WebhookToken == "" {
		logger.Error("工作流未配置Webhook Token", zap.String("workflow_id", id))
		return nil, core.ErrNotFound
	}

	// 🔥 3. 解密数据库中的Token
	decryptedToken, err := s.decryptWebhookToken(*workflow.WebhookToken)
	if err != nil {
		logger.Error("解密Webhook Token失败", zap.Error(err), zap.String("workflow_id", id))
		return nil, core.ErrUnauthorized
	}

	// 🔥 4. 对比Token
	if decryptedToken != token {
		logger.Warn("Webhook Token不匹配",
			zap.String("workflow_id", id),
			zap.String("token_preview", token[:4]+"****"))
		return nil, core.ErrUnauthorized
	}

	return workflow, nil
}

// EnableWebhook 启用Webhook触发
//
// 如果Token不存在会自动生成
//
// 参数：
//   - ctx: 上下文
//   - id: 工作流ID
//
// 返回：
//   - 更新后的工作流对象
//   - 原始Token（仅首次生成时返回，否则为空字符串）
//   - 错误信息
func (s *WorkflowService) EnableWebhook(ctx context.Context, id string) (*core.Workflow, string, error) {
	// 1. 解析UUID
	uuidID, err := uuid.Parse(id)
	if err != nil {
		logger.Error("解析工作流ID失败", zap.Error(err), zap.String("id", id))
		return nil, "", core.ErrBadRequest
	}

	// 2. 查询工作流
	workflow, err := s.store.FindByID(ctx, uuidID)
	if err != nil {
		logger.Error("查询工作流失败", zap.Error(err), zap.String("id", id))
		return nil, "", err
	}

	// 3. 如果已经启用，直接返回
	if workflow.WebhookEnabled != nil && *workflow.WebhookEnabled {
		logger.Info("Webhook已启用，无需重复操作", zap.String("id", id))
		return workflow, "", nil
	}

	// 🔥 用于记录是否生成了新Token（需要返回给用户）
	var plainTokenToReturn string

	// 4. 如果Token为空，生成Token
	if workflow.WebhookToken == nil || *workflow.WebhookToken == "" {
		// 生成原始Token
		if err := workflow.RegenerateWebhookToken(); err != nil {
			logger.Error("生成Webhook Token失败", zap.Error(err))
			return nil, "", err
		}

		// 🔥 保存原始Token（用于返回给用户）
		plainToken := *workflow.WebhookToken
		plainTokenToReturn = plainToken

		// 🔥 加密Token存储到数据库
		encryptedToken, err := s.encryptWebhookToken(plainToken)
		if err != nil {
			logger.Error("加密Webhook Token失败", zap.Error(err))
			return nil, "", err
		}
		workflow.WebhookToken = &encryptedToken

		logger.Info("生成并加密Webhook Token成功",
			zap.String("workflow_id", id),
			zap.String("token_preview", plainToken[:4]+"****"))
	}

	// 5. 启用Webhook
	trueValue := true
	workflow.WebhookEnabled = &trueValue

	// 6. 更新数据库
	if err := s.store.Update(ctx, workflow); err != nil {
		logger.Error("更新工作流失败", zap.Error(err))
		return nil, "", err
	}

	logger.Info("启用Webhook成功", zap.String("workflow_id", id))
	return workflow, plainTokenToReturn, nil
}

// DisableWebhook 禁用Webhook触发
//
// 参数：
//   - ctx: 上下文
//   - id: 工作流ID
//
// 返回：
//   - 更新后的工作流对象
//   - 错误信息
func (s *WorkflowService) DisableWebhook(ctx context.Context, id string) (*core.Workflow, error) {
	// 1. 解析UUID
	uuidID, err := uuid.Parse(id)
	if err != nil {
		logger.Error("解析工作流ID失败", zap.Error(err), zap.String("id", id))
		return nil, core.ErrBadRequest
	}

	// 2. 查询工作流
	workflow, err := s.store.FindByID(ctx, uuidID)
	if err != nil {
		logger.Error("查询工作流失败", zap.Error(err), zap.String("id", id))
		return nil, err
	}

	// 3. 如果已经禁用，直接返回
	if workflow.WebhookEnabled == nil || !*workflow.WebhookEnabled {
		logger.Info("Webhook已禁用，无需重复操作", zap.String("id", id))
		return workflow, nil
	}

	// 4. 禁用Webhook
	falseValue := false
	workflow.WebhookEnabled = &falseValue

	// 5. 更新数据库
	if err := s.store.Update(ctx, workflow); err != nil {
		logger.Error("更新工作流失败", zap.Error(err))
		return nil, err
	}

	logger.Info("禁用Webhook成功", zap.String("workflow_id", id))
	return workflow, nil
}

// RegenerateWebhookToken 重新生成Webhook Token
//
// 返回新生成的Token
//
// 参数：
//   - ctx: 上下文
//   - id: 工作流ID
//
// 返回：
//   - 新生成的Token字符串
//   - 错误信息
func (s *WorkflowService) RegenerateWebhookToken(ctx context.Context, id string) (string, error) {
	// 1. 解析UUID
	uuidID, err := uuid.Parse(id)
	if err != nil {
		logger.Error("解析工作流ID失败", zap.Error(err), zap.String("id", id))
		return "", core.ErrBadRequest
	}

	// 2. 查询工作流
	workflow, err := s.store.FindByID(ctx, uuidID)
	if err != nil {
		logger.Error("查询工作流失败", zap.Error(err), zap.String("id", id))
		return "", err
	}

	// 3. 生成新Token
	oldToken := ""
	if workflow.WebhookToken != nil {
		// 尝试解密显示旧Token预览
		if decrypted, err := s.decryptWebhookToken(*workflow.WebhookToken); err == nil && len(decrypted) >= 4 {
			oldToken = decrypted[:4] + "****"
		} else {
			oldToken = "****"
		}
	}

	// 生成原始Token
	if err := workflow.RegenerateWebhookToken(); err != nil {
		logger.Error("生成Webhook Token失败", zap.Error(err))
		return "", err
	}

	// 🔥 保存原始Token（用于返回）
	plainToken := *workflow.WebhookToken

	// 🔥 加密Token存储
	encryptedToken, err := s.encryptWebhookToken(plainToken)
	if err != nil {
		logger.Error("加密Webhook Token失败", zap.Error(err))
		return "", err
	}
	workflow.WebhookToken = &encryptedToken

	// 4. 更新数据库
	if err := s.store.Update(ctx, workflow); err != nil {
		logger.Error("更新工作流失败", zap.Error(err))
		return "", err
	}

	logger.Info("重新生成Webhook Token成功",
		zap.String("workflow_id", id),
		zap.String("old_token", oldToken),
		zap.String("new_token", plainToken[:4]+"****"))

	// 🔥 返回原始Token（未加密）
	return plainToken, nil
}

// UpdateWebhookIPWhitelist 更新Webhook IP白名单
//
// 参数：
//   - ctx: 上下文
//   - id: 工作流ID
//   - whitelist: IP白名单列表
//
// 返回：
//   - 错误信息
func (s *WorkflowService) UpdateWebhookIPWhitelist(ctx context.Context, id string, whitelist []string) error {
	// 1. 解析UUID
	uuidID, err := uuid.Parse(id)
	if err != nil {
		logger.Error("解析工作流ID失败", zap.Error(err), zap.String("id", id))
		return core.ErrBadRequest
	}

	// 2. 查询工作流
	workflow, err := s.store.FindByID(ctx, uuidID)
	if err != nil {
		logger.Error("查询工作流失败", zap.Error(err), zap.String("id", id))
		return err
	}

	// 3. 设置IP白名单
	if err := workflow.SetWebhookIPWhitelist(whitelist); err != nil {
		logger.Error("设置Webhook IP白名单失败", zap.Error(err))
		return err
	}

	// 4. 更新数据库
	if err := s.store.Update(ctx, workflow); err != nil {
		logger.Error("更新工作流失败", zap.Error(err))
		return err
	}

	logger.Info("更新Webhook IP白名单成功",
		zap.String("workflow_id", id),
		zap.Int("whitelist_count", len(whitelist)))

	return nil
}

// ========== 🔥 Webhook Token 加密/解密辅助方法 ==========

// encryptWebhookToken 加密Webhook Token
//
// 参数：
//   - token: 原始Token
//
// 返回：
//   - 加密后的Token
//   - 错误信息
func (s *WorkflowService) encryptWebhookToken(token string) (string, error) {
	encrypted, err := s.crypto.Encrypt(token)
	if err != nil {
		logger.Error("加密Webhook Token失败", zap.Error(err))
		return "", err
	}
	return encrypted, nil
}

// decryptWebhookToken 解密Webhook Token
//
// 参数：
//   - encryptedToken: 加密后的Token
//
// 返回：
//   - 原始Token
//   - 错误信息
func (s *WorkflowService) decryptWebhookToken(encryptedToken string) (string, error) {
	decrypted, err := s.crypto.Decrypt(encryptedToken)
	if err != nil {
		logger.Error("解密Webhook Token失败", zap.Error(err))
		return "", err
	}
	return decrypted, nil
}

// DecryptWebhookToken 解密工作流的Webhook Token（供Controller调用）
//
// 用于获取完整Webhook URL时解密Token
//
// 参数：
//   - ctx: 上下文
//   - id: 工作流ID
//
// 返回：
//   - 解密后的Token
//   - 错误信息
func (s *WorkflowService) DecryptWebhookToken(ctx context.Context, id string) (string, error) {
	// 1. 解析UUID
	uuidID, err := uuid.Parse(id)
	if err != nil {
		logger.Error("解析工作流ID失败", zap.Error(err), zap.String("id", id))
		return "", core.ErrBadRequest
	}

	// 2. 查询工作流
	workflow, err := s.store.FindByID(ctx, uuidID)
	if err != nil {
		logger.Error("查询工作流失败", zap.Error(err), zap.String("id", id))
		return "", err
	}

	// 3. 检查Token是否存在
	if workflow.WebhookToken == nil || *workflow.WebhookToken == "" {
		return "", fmt.Errorf("webhook token不存在")
	}

	// 4. 解密Token
	decryptedToken, err := s.decryptWebhookToken(*workflow.WebhookToken)
	if err != nil {
		return "", err
	}

	return decryptedToken, nil
}

// CreateWebhookCronJob 一键创建Webhook定时任务
//
// 自动完成以下步骤：
// 1. 确保Webhook已启用，获取webhook_url
// 2. 创建Credential存储webhook_url（加密存储）
// 3. 创建CronJob使用该凭证定期触发
//
// 参数：
//   - ctx: 上下文
//   - id: 工作流ID
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
func (s *WorkflowService) CreateWebhookCronJob(
	ctx context.Context,
	id, baseURL, time, credentialName, cronJobName, description string,
	isActive bool,
) (*core.Credential, *core.CronJob, error) {
	// ========== Step 1: 检查依赖服务是否已注入 ==========
	if s.credentialService == nil {
		logger.Error("CredentialService未注入")
		return nil, nil, fmt.Errorf("凭证服务未初始化")
	}
	if s.cronJobService == nil {
		logger.Error("CronJobService未注入")
		return nil, nil, fmt.Errorf("定时任务服务未初始化")
	}

	// ========== Step 2: 获取工作流并验证Webhook状态 ==========
	workflow, err := s.FindByID(ctx, id)
	if err != nil {
		logger.Error("查询工作流失败", zap.Error(err), zap.String("id", id))
		return nil, nil, err
	}

	// 检查Webhook是否已启用
	if workflow.WebhookEnabled == nil || !*workflow.WebhookEnabled {
		logger.Error("Webhook未启用", zap.String("workflow_id", id))
		return nil, nil, fmt.Errorf("工作流的Webhook未启用，请先启用Webhook")
	}

	// 检查Webhook Token是否存在
	if workflow.WebhookToken == nil || *workflow.WebhookToken == "" {
		logger.Error("Webhook Token不存在", zap.String("workflow_id", id))
		return nil, nil, fmt.Errorf("webhook token不存在")
	}

	// 解密Token以构建完整的webhook_url
	decryptedToken, err := s.decryptWebhookToken(*workflow.WebhookToken)
	if err != nil {
		logger.Error("解密Webhook Token失败", zap.Error(err), zap.String("workflow_id", id))
		return nil, nil, fmt.Errorf("解密Webhook Token失败: %w", err)
	}

	// 🔥 构建完整的webhook_url（使用Controller传递的baseURL）
	webhookURL := fmt.Sprintf("%s/api/v1/workflow/%s/webhook?key=%s", baseURL, workflow.ID.String(), decryptedToken)

	logger.Info("构建Webhook URL",
		zap.String("workflow_id", id),
		zap.String("base_url", baseURL),
		zap.String("webhook_url", webhookURL))

	// ========== Step 3: 处理默认值 ==========
	// 时间表达式（7段格式：秒 分 时 日 月 周 年）
	if time == "" {
		time = "0 0 0 1 * * *" // 每月1号0点0分0秒
	}

	// 凭证名称
	if credentialName == "" {
		credentialName = workflow.Name + ":webhook"
	}

	// 定时任务名称
	if cronJobName == "" {
		cronJobName = workflow.Name + ":计划任务"
	}

	// 定时任务描述
	if description == "" {
		description = fmt.Sprintf("定期触发工作流 %s 的Webhook", workflow.Name)
	}

	logger.Info("开始创建Webhook定时任务",
		zap.String("workflow_id", id),
		zap.String("workflow_name", workflow.Name),
		zap.String("credential_name", credentialName),
		zap.String("cronjob_name", cronJobName),
		zap.String("time", time),
		zap.Bool("is_active", isActive))

	// ========== Step 3.5: 检查凭证和CronJob是否已存在 ==========
	// 🔥 在创建之前检查，避免部分成功导致数据不一致

	// 检查凭证是否已存在（使用 List + 过滤器）
	credentialFilters := []filters.Filter{
		&filters.FilterOption{Column: "team_id", Value: workflow.TeamID.String(), Op: filters.FILTER_EQ},
		&filters.FilterOption{Column: "name", Value: credentialName, Op: filters.FILTER_EQ},
		&filters.FilterOption{Column: "deleted", Value: false, Op: filters.FILTER_EQ},
	}
	existingCredentials, err := s.credentialService.List(ctx, 0, 1, credentialFilters...)
	if err != nil {
		logger.Error("检查凭证是否存在失败", zap.Error(err))
		return nil, nil, fmt.Errorf("检查凭证是否存在失败: %w", err)
	}
	if len(existingCredentials) > 0 {
		logger.Warn("凭证名称已存在",
			zap.String("credential_name", credentialName),
			zap.String("existing_id", existingCredentials[0].ID.String()),
			zap.String("workflow_id", id))
		return nil, nil, fmt.Errorf("凭证名称 '%s' 已存在，请更换名称", credentialName)
	}

	// 检查CronJob是否已存在（使用 List + 过滤器）
	cronJobFilters := []filters.Filter{
		&filters.FilterOption{Column: "team_id", Value: workflow.TeamID.String(), Op: filters.FILTER_EQ},
		&filters.FilterOption{Column: "name", Value: cronJobName, Op: filters.FILTER_EQ},
		&filters.FilterOption{Column: "deleted", Value: false, Op: filters.FILTER_EQ},
	}
	existingCronJobs, err := s.cronJobService.List(ctx, 0, 1, cronJobFilters...)
	if err != nil {
		logger.Error("检查定时任务是否存在失败", zap.Error(err))
		return nil, nil, fmt.Errorf("检查定时任务是否存在失败: %w", err)
	}
	if len(existingCronJobs) > 0 {
		logger.Warn("定时任务名称已存在",
			zap.String("cronjob_name", cronJobName),
			zap.String("existing_id", existingCronJobs[0].ID.String()),
			zap.String("workflow_id", id))
		return nil, nil, fmt.Errorf("定时任务名称 '%s' 已存在，请更换名称", cronJobName)
	}

	// ========== Step 4: 创建凭证存储webhook_url ==========
	// 构建凭证的Value字段（JSON格式）
	credentialValue := map[string]interface{}{
		"webhook": webhookURL,
	}
	credentialValueBytes, err := json.Marshal(credentialValue)
	if err != nil {
		logger.Error("序列化凭证Value失败", zap.Error(err))
		return nil, nil, fmt.Errorf("序列化凭证Value失败: %w", err)
	}
	credentialValueJSON := string(credentialValueBytes)

	// 创建凭证对象
	credentialIsActive := true // 凭证始终激活（只是存储数据，不影响执行）
	credential := &core.Credential{
		TeamID:      workflow.TeamID,
		Category:    "webhook",
		Name:        credentialName,
		Description: fmt.Sprintf("工作流 %s 的Webhook触发地址", workflow.Name),
		Project:     workflow.Project,
		Value:       credentialValueJSON,
		IsActive:    &credentialIsActive,
		Metadata:    "{}", // 设置为空JSON对象，避免MySQL JSON列报错
	}

	// 调用CredentialService创建凭证（会自动加密）
	createdCredential, err := s.credentialService.Create(ctx, credential)
	if err != nil {
		logger.Error("创建凭证失败",
			zap.Error(err),
			zap.String("workflow_id", id),
			zap.String("credential_name", credentialName))
		return nil, nil, fmt.Errorf("创建凭证失败: %w", err)
	}

	logger.Info("凭证创建成功",
		zap.String("credential_id", createdCredential.ID.String()),
		zap.String("credential_name", createdCredential.Name))

	// ========== Step 5: 创建CronJob使用该凭证 ==========
	// 🔥 CronJob的命令：使用curl调用webhook（通过凭证获取URL）
	// 注意：这里需要Worker支持从凭证中读取webhook并调用
	// 简单方案：Command使用特殊标记，Worker识别后从凭证读取URL
	// 这里先用简单方案，实际可能需要扩展Worker的能力

	// 构建CronJob的Metadata（暂时不包含凭证引用，因为Metadata结构不支持）
	// 凭证信息通过Args传递
	metadata := &core.Metadata{
		Priority: 5, // 设置默认优先级
	}
	metadataJSON, err := core.SerializeMetadata(metadata)
	if err != nil {
		logger.Error("序列化CronJob Metadata失败", zap.Error(err))
		// 删除已创建的凭证
		_ = s.credentialService.DeleteByID(ctx, createdCredential.ID.String())
		return nil, nil, fmt.Errorf("序列化CronJob Metadata失败: %w", err)
	}

	// 🔥 构建CronJob的Args（HTTP Runner格式）
	httpArgs := map[string]interface{}{
		"url":    webhookURL,
		"method": "POST",
		"headers": map[string]string{
			"Content-Type": "application/json",
		},
		"expected_status": []int{200, 201}, // 期望的HTTP状态码
	}

	// 如果工作流有默认变量，将其作为示例body添加到Args中
	// 这样用户编辑CronJob时可以看到可以传递哪些变量
	if len(workflow.DefaultVariables) > 0 {
		var defaultVars map[string]interface{}
		if err := json.Unmarshal(workflow.DefaultVariables, &defaultVars); err == nil && len(defaultVars) > 0 {
			// HTTP Runner 的 body 需要是 JSON 字符串
			bodyData := map[string]interface{}{
				"initial_variables": defaultVars,
			}
			bodyJSON, _ := json.Marshal(bodyData)
			httpArgs["body"] = string(bodyJSON)

			logger.Info("添加默认变量到CronJob Args",
				zap.Int("variable_count", len(defaultVars)),
				zap.String("workflow_id", id))
		}
	}

	// 序列化Args
	argsJSON, err := json.Marshal(httpArgs)
	if err != nil {
		logger.Error("序列化CronJob Args失败", zap.Error(err))
		// 删除已创建的凭证
		_ = s.credentialService.DeleteByID(ctx, createdCredential.ID.String())
		return nil, nil, fmt.Errorf("序列化CronJob Args失败: %w", err)
	}

	// 创建CronJob对象（使用HTTP Runner）
	saveLog := true
	cronJob := &core.CronJob{
		TeamID:      workflow.TeamID,
		Project:     workflow.Project,
		Category:    "http", // 🔥 使用 http runner
		Name:        cronJobName,
		Time:        time,
		Command:     "http", // 🔥 使用 http 命令
		Args:        string(argsJSON),
		Description: description,
		IsActive:    &isActive, // 使用传入的激活状态，默认false
		SaveLog:     &saveLog,
		Timeout:     300, // 5分钟超时
		Metadata:    metadataJSON,
	}

	// 调用CronJobService创建定时任务
	createdCronJob, err := s.cronJobService.Create(ctx, cronJob)
	if err != nil {
		logger.Error("创建定时任务失败",
			zap.Error(err),
			zap.String("workflow_id", id),
			zap.String("cronjob_name", cronJobName))
		// 回滚：删除已创建的凭证
		_ = s.credentialService.DeleteByID(ctx, createdCredential.ID.String())
		return nil, nil, fmt.Errorf("创建定时任务失败: %w", err)
	}

	logger.Info("定时任务创建成功",
		zap.String("cronjob_id", createdCronJob.ID.String()),
		zap.String("cronjob_name", createdCronJob.Name),
		zap.String("time", createdCronJob.Time))

	// ========== Step 6: 返回结果 ==========
	logger.Info("Webhook定时任务创建完成",
		zap.String("workflow_id", id),
		zap.String("credential_id", createdCredential.ID.String()),
		zap.String("cronjob_id", createdCronJob.ID.String()))

	return createdCredential, createdCronJob, nil
}

// WorkflowService 接口定义（需要在 core/workflow.go 中定义）
// 这里只是实现，接口定义应该在 core 包中
