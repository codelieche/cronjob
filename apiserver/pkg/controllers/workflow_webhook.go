package controllers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/codelieche/cronjob/apiserver/pkg/controllers/forms"
	"github.com/codelieche/cronjob/apiserver/pkg/core"
	"github.com/codelieche/cronjob/apiserver/pkg/utils/controllers"
	"github.com/codelieche/cronjob/apiserver/pkg/utils/logger"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// WorkflowWebhookController Webhook触发控制器
//
// 负责处理工作流的Webhook触发相关操作，包括：
// - Webhook触发：通过HTTP请求触发工作流执行
// - Webhook配置：启用/禁用、Token管理、IP白名单等
type WorkflowWebhookController struct {
	controllers.BaseController
	workflowService        core.WorkflowService        // 工作流服务
	workflowExecuteService core.WorkflowExecuteService // 工作流执行服务
}

// NewWorkflowWebhookController 创建WorkflowWebhookController实例
func NewWorkflowWebhookController(
	workflowService core.WorkflowService,
	workflowExecuteService core.WorkflowExecuteService,
) *WorkflowWebhookController {
	return &WorkflowWebhookController{
		workflowService:        workflowService,
		workflowExecuteService: workflowExecuteService,
	}
}

// TriggerByWebhook 通过Webhook触发工作流
//
// @Summary 通过Webhook触发工作流
// @Description 使用Webhook Token触发工作流执行，无需用户认证（采用查询参数传递Token，符合业界标准）
// @Tags webhook
// @Accept json
// @Produce json
// @Param id path string true "工作流ID"
// @Param key query string true "Webhook Token（查询参数）"
// @Param body body forms.WebhookTriggerForm false "触发参数（可选）"
// @Success 200 {object} core.WorkflowExecute "触发成功，返回执行实例"
// @Failure 401 {object} core.ErrorResponse "Token无效"
// @Failure 403 {object} core.ErrorResponse "IP不在白名单或Webhook未启用"
// @Failure 404 {object} core.ErrorResponse "工作流不存在"
// @Failure 500 {object} core.ErrorResponse "服务器错误"
// @Router /workflow/{id}/webhook [post]
//
// 功能说明：
// 1. 验证 Webhook Token（从查询参数 key 获取，无需用户认证）
// 2. 检查 Webhook 是否启用
// 3. 检查客户端IP是否在白名单中
// 4. 解析请求参数（variables、metadata_override）
// 5. 调用 WorkflowExecuteService.Execute() 触发执行
//
// 安全机制：
// - Token认证：32字符随机Token（查询参数key传递）
// - IP白名单：可选的IP地址限制
// - 状态检查：只有启用Webhook的工作流才能触发
//
// URL格式（符合业界标准）：
//
//	POST /api/v1/workflow/{id}/webhook?key={token}
//
// 请求示例：
//
//	POST /api/v1/workflow/{id}/webhook?key=aB3dE5fG7hI9jK1lM3nO5pQ7rS9tU1vW
//	Content-Type: application/json
//
//	{
//	  "variables": {
//	    "branch": "main",
//	    "environment": "production"
//	  }
//	}
//
// 响应示例：
//
//	{
//	  "code": 200,
//	  "message": "Workflow triggered successfully",
//	  "data": {
//	    "id": "uuid-xxx",
//	    "workflow_id": "uuid-yyy",
//	    "status": "pending",
//	    "trigger_type": "webhook",
//	    "created_at": "2025-10-17T10:30:00Z"
//	  }
//	}
func (controller *WorkflowWebhookController) TriggerByWebhook(c *gin.Context) {
	// ========== Step 1: 解析URL参数 ==========
	// 🔥 从路径参数获取workflow_id
	workflowID := c.Param("id")
	// 🔥 从查询参数获取token（符合业界标准：GitHub/GitLab/钉钉等）
	token := c.Query("key")

	if workflowID == "" || token == "" {
		logger.Warn("Webhook触发失败：缺少必要参数",
			zap.String("workflow_id", workflowID),
			zap.Bool("has_token", token != ""))
		controller.HandleError(c, core.ErrBadRequest, http.StatusBadRequest)
		return
	}

	// 获取客户端IP
	clientIP := c.ClientIP()

	logger.Info("收到Webhook触发请求",
		zap.String("workflow_id", workflowID),
		zap.String("client_ip", clientIP),
		zap.String("user_agent", c.GetHeader("User-Agent")),
		zap.String("token", token[:4]+"****")) // 只记录Token前4位

	// ========== Step 2: 验证Token并获取Workflow ==========
	ctx := c.Request.Context()
	// 🔥 传入workflow ID和token进行验证
	workflow, err := controller.workflowService.FindByWebhookToken(ctx, workflowID, token)
	if err != nil {
		if err == core.ErrNotFound || err == core.ErrUnauthorized {
			logger.Warn("Webhook Token验证失败：Token无效或工作流不存在",
				zap.String("workflow_id", workflowID),
				zap.String("token", token[:4]+"****"),
				zap.String("client_ip", clientIP))
			controller.HandleError(c, core.ErrUnauthorized, http.StatusUnauthorized)
		} else {
			logger.Error("查询工作流失败", zap.Error(err))
			controller.HandleError(c, err, http.StatusInternalServerError)
		}
		return
	}

	// ========== Step 3: 验证Webhook是否启用 ==========
	// 🔥 不再需要验证ID是否匹配（Service层已验证）
	if workflow.WebhookEnabled == nil || !*workflow.WebhookEnabled {
		logger.Warn("Webhook已禁用",
			zap.String("workflow_id", workflowID),
			zap.String("workflow_name", workflow.Name),
			zap.String("client_ip", clientIP))
		controller.HandleError(c, core.ErrForbidden, http.StatusForbidden)
		return
	}

	// ========== Step 4: IP白名单校验 ==========
	if !workflow.IsIPAllowed(clientIP) {
		logger.Warn("IP不在白名单中",
			zap.String("workflow_id", workflowID),
			zap.String("ip", clientIP))
		controller.HandleError(c, core.ErrForbidden, http.StatusForbidden)
		return
	}

	// ========== Step 6: 解析请求体（可选） ==========
	var form forms.WebhookTriggerForm
	if err := c.ShouldBindJSON(&form); err != nil {
		// 请求体可选，解析失败不影响执行
		logger.Debug("解析请求体失败（将使用空参数）", zap.Error(err))
		form.Variables = make(map[string]interface{})
		form.MetadataOverride = make(map[string]interface{})
	}

	// 如果字段为nil，初始化为空map
	if form.Variables == nil {
		form.Variables = make(map[string]interface{})
	}
	if form.MetadataOverride == nil {
		form.MetadataOverride = make(map[string]interface{})
	}

	// ========== Step 7: 构建执行请求 ==========
	executeReq := &core.ExecuteRequest{
		WorkflowID:       workflow.ID,
		TriggerType:      "webhook", // 🔥 Webhook触发
		UserID:           nil,       // Webhook触发无用户信息
		Username:         "webhook", // 标识为Webhook触发
		InitialVariables: form.Variables,
		MetadataOverride: form.MetadataOverride,
	}

	// ========== Step 8: 执行工作流 ==========
	workflowExec, err := controller.workflowExecuteService.Execute(ctx, executeReq)
	if err != nil {
		logger.Error("执行工作流失败",
			zap.String("workflow_id", workflowID),
			zap.Error(err))
		controller.HandleError(c, err, http.StatusInternalServerError)
		return
	}

	// ========== Step 9: 返回成功响应 ==========
	logger.Info("Webhook触发成功",
		zap.String("workflow_id", workflowID),
		zap.String("execute_id", workflowExec.ID.String()),
		zap.String("client_ip", clientIP))

	// 返回执行实例
	controller.HandleOK(c, workflowExec)
}

// EnableWebhook 启用Webhook触发
//
// @Summary 启用工作流的Webhook触发功能
// @Description 启用Webhook，如果Token不存在会自动生成
// @Tags workflow
// @Accept json
// @Produce json
// @Param id path string true "工作流ID"
// @Param body body forms.WebhookToggleForm true "启用参数"
// @Success 200 {object} core.Workflow "工作流信息（包含webhook_url）"
// @Failure 400 {object} core.ErrorResponse "请求参数错误"
// @Failure 404 {object} core.ErrorResponse "工作流不存在"
// @Router /workflow/{id}/webhook/toggle [put]
// @Security BearerAuth
func (controller *WorkflowWebhookController) ToggleWebhook(c *gin.Context) {
	// ========== Step 1: 解析参数 ==========
	id := c.Param("id")
	if id == "" {
		controller.HandleError(c, core.ErrBadRequest, http.StatusBadRequest)
		return
	}

	// ========== Step 2: 解析请求体 ==========
	var form forms.WebhookToggleForm
	if err := c.ShouldBindJSON(&form); err != nil {
		logger.Warn("解析请求体失败", zap.Error(err))
		controller.HandleError(c, core.ErrBadRequest, http.StatusBadRequest)
		return
	}

	// ========== Step 3: 调用服务启用/禁用 ==========
	ctx := c.Request.Context()
	var workflow *core.Workflow
	var err error
	var plainToken string // 🔥 用于接收首次生成的原始Token

	if *form.Enabled {
		// 启用Webhook
		workflow, plainToken, err = controller.workflowService.EnableWebhook(ctx, id)
	} else {
		// 禁用Webhook
		workflow, err = controller.workflowService.DisableWebhook(ctx, id)
	}

	if err != nil {
		if err == core.ErrNotFound {
			controller.HandleError(c, err, http.StatusNotFound)
		} else {
			controller.HandleError(c, err, http.StatusInternalServerError)
		}
		return
	}

	// 🔥 Step 4: 处理Token（首次生成 or 解密已有）
	if plainToken != "" {
		// 首次生成：使用原始Token
		workflow.WebhookToken = &plainToken
		logger.Info("首次生成Webhook Token，返回原始Token给用户",
			zap.String("workflow_id", id),
			zap.String("token_preview", plainToken[:4]+"****"))
	} else if *form.Enabled && workflow.WebhookToken != nil && *workflow.WebhookToken != "" {
		// 再次启用：需要解密已有Token用于生成URL
		// 🔥 关键：不修改workflow.WebhookToken（保持脱敏），只用于生成webhookURL
		// 通过service解密token
		decryptedToken, err := controller.workflowService.DecryptWebhookToken(ctx, id)
		if err != nil {
			logger.Warn("解密Webhook Token失败，将使用脱敏URL",
				zap.Error(err),
				zap.String("workflow_id", id))
		} else {
			// 临时替换为解密的token，用于生成正确的URL
			workflow.WebhookToken = &decryptedToken
			logger.Info("再次启用Webhook，使用解密Token生成URL",
				zap.String("workflow_id", id))
		}
	}

	// ========== Step 5: 动态生成 Webhook URL ==========
	// 🔥 webhook_url 字段是只读的（gorm:"-"），需要手动设置
	// 注意：必须在处理token之后生成URL
	workflow.WebhookURL = workflow.GetWebhookURL(getBaseURL(c))

	// ========== Step 6: 返回工作流信息 ==========
	controller.HandleOK(c, workflow)
}

// RegenerateToken 重新生成Webhook Token
//
// @Summary 重新生成工作流的Webhook Token
// @Description 生成新的Webhook Token，旧Token将失效
// @Tags workflow
// @Accept json
// @Produce json
// @Param id path string true "工作流ID"
// @Success 200 {object} map[string]string "新Token信息"
// @Failure 400 {object} core.ErrorResponse "请求参数错误"
// @Failure 404 {object} core.ErrorResponse "工作流不存在"
// @Router /workflow/{id}/webhook/regenerate [post]
// @Security BearerAuth
func (controller *WorkflowWebhookController) RegenerateToken(c *gin.Context) {
	// ========== Step 1: 解析参数 ==========
	id := c.Param("id")
	if id == "" {
		controller.HandleError(c, core.ErrBadRequest, http.StatusBadRequest)
		return
	}

	// ========== Step 2: 调用服务重新生成Token ==========
	ctx := c.Request.Context()
	newToken, err := controller.workflowService.RegenerateWebhookToken(ctx, id)
	if err != nil {
		if err == core.ErrNotFound {
			controller.HandleError(c, err, http.StatusNotFound)
		} else {
			controller.HandleError(c, err, http.StatusInternalServerError)
		}
		return
	}

	// ========== Step 3: 查询更新后的工作流（获取完整URL） ==========
	workflow, err := controller.workflowService.FindByID(ctx, id)
	if err != nil {
		controller.HandleError(c, err, http.StatusInternalServerError)
		return
	}

	// ========== Step 4: 返回新Token信息 ==========
	// 注意：这里返回完整Token，因为这是Token生成的唯一时机
	// 后续查询不会再返回完整Token
	response := map[string]interface{}{
		"webhook_token":   newToken,
		"webhook_url":     workflow.GetWebhookURL(getBaseURL(c)), // 动态生成完整URL
		"webhook_enabled": workflow.WebhookEnabled,
	}

	controller.HandleOK(c, response)
}

// UpdateIPWhitelist 更新Webhook IP白名单
//
// @Summary 更新工作流的Webhook IP白名单
// @Description 设置允许触发Webhook的IP地址列表，空数组表示允许所有IP
// @Tags workflow
// @Accept json
// @Produce json
// @Param id path string true "工作流ID"
// @Param body body forms.WebhookIPWhitelistForm true "IP白名单配置"
// @Success 200 {object} core.Workflow "更新后的工作流信息"
// @Failure 400 {object} core.ErrorResponse "请求参数错误"
// @Failure 404 {object} core.ErrorResponse "工作流不存在"
// @Router /workflow/{id}/webhook/whitelist [put]
// @Security BearerAuth
func (controller *WorkflowWebhookController) UpdateIPWhitelist(c *gin.Context) {
	// ========== Step 1: 解析参数 ==========
	id := c.Param("id")
	if id == "" {
		controller.HandleError(c, core.ErrBadRequest, http.StatusBadRequest)
		return
	}

	// ========== Step 2: 解析请求体 ==========
	var form forms.WebhookIPWhitelistForm
	if err := c.ShouldBindJSON(&form); err != nil {
		logger.Warn("解析请求体失败", zap.Error(err))
		controller.HandleError(c, core.ErrBadRequest, http.StatusBadRequest)
		return
	}

	// 验证表单
	if err := form.Validate(); err != nil {
		logger.Warn("IP白名单验证失败", zap.Error(err))
		controller.HandleError(c, err, http.StatusBadRequest)
		return
	}

	// ========== Step 3: 调用服务更新白名单 ==========
	ctx := c.Request.Context()
	if err := controller.workflowService.UpdateWebhookIPWhitelist(ctx, id, form.IPWhitelist); err != nil {
		if err == core.ErrNotFound {
			controller.HandleError(c, err, http.StatusNotFound)
		} else {
			controller.HandleError(c, err, http.StatusInternalServerError)
		}
		return
	}

	// ========== Step 4: 查询更新后的工作流 ==========
	workflow, err := controller.workflowService.FindByID(ctx, id)
	if err != nil {
		controller.HandleError(c, err, http.StatusInternalServerError)
		return
	}

	// ========== Step 5: 动态生成 Webhook URL ==========
	// 🔥 webhook_url 字段是只读的（gorm:"-"），需要手动设置
	workflow.WebhookURL = workflow.GetWebhookURL(getBaseURL(c))

	// ========== Step 6: 返回工作流信息 ==========
	controller.HandleOK(c, workflow)
}

// GetWebhookInfo 获取Webhook配置信息（可选实现）
//
// @Summary 获取工作流的Webhook配置信息
// @Description 查询工作流的Webhook配置状态和统计信息
// @Tags workflow
// @Accept json
// @Produce json
// @Param id path string true "工作流ID"
// @Success 200 {object} map[string]interface{} "Webhook配置信息"
// @Failure 400 {object} core.ErrorResponse "请求参数错误"
// @Failure 404 {object} core.ErrorResponse "工作流不存在"
// @Router /workflow/{id}/webhook/info [get]
// @Security BearerAuth
func (controller *WorkflowWebhookController) GetWebhookInfo(c *gin.Context) {
	// ========== Step 1: 解析参数 ==========
	id := c.Param("id")
	if id == "" {
		controller.HandleError(c, core.ErrBadRequest, http.StatusBadRequest)
		return
	}

	// ========== Step 2: 查询工作流 ==========
	ctx := c.Request.Context()
	workflow, err := controller.workflowService.FindByID(ctx, id)
	if err != nil {
		if err == core.ErrNotFound {
			controller.HandleError(c, err, http.StatusNotFound)
		} else {
			controller.HandleError(c, err, http.StatusInternalServerError)
		}
		return
	}

	// ========== Step 3: 构建响应 ==========
	// 获取IP白名单
	ipWhitelist, _ := workflow.GetWebhookIPWhitelist()

	// 脱敏Token（只显示前4位）
	tokenMasked := ""
	if workflow.WebhookToken != nil && *workflow.WebhookToken != "" {
		if len(*workflow.WebhookToken) > 4 {
			tokenMasked = (*workflow.WebhookToken)[:4] + "****"
		} else {
			tokenMasked = "****"
		}
	}

	// 🔥 webhook_url 也要脱敏处理
	webhookURL := workflow.GetWebhookURL(getBaseURL(c))
	maskedURL := maskWebhookURL(webhookURL)

	response := map[string]interface{}{
		"workflow_id":          workflow.ID,
		"workflow_name":        workflow.Name,
		"webhook_enabled":      workflow.WebhookEnabled,
		"webhook_token_masked": tokenMasked, // 🔥 脱敏Token
		"webhook_url":          maskedURL,   // 🔥 脱敏URL
		"webhook_ip_whitelist": ipWhitelist,
		"last_execute_at":      workflow.LastExecuteAt,
		"last_status":          workflow.LastStatus,
	}

	// ========== Step 4: 返回响应 ==========
	controller.HandleOK(c, response)
}

// GetWebhookFullURL 获取完整的Webhook URL（包含完整Token）
//
// @Summary 获取完整的Webhook URL
// @Description 获取包含完整Token的Webhook URL，仅在需要时调用
// @Tags workflow
// @Accept json
// @Produce json
// @Param id path string true "工作流ID"
// @Success 200 {object} map[string]interface{} "完整的Webhook URL"
// @Failure 400 {object} core.ErrorResponse "请求参数错误"
// @Failure 404 {object} core.ErrorResponse "工作流不存在"
// @Router /workflow/{id}/webhook/url [get]
// @Security BearerAuth
func (controller *WorkflowWebhookController) GetWebhookFullURL(c *gin.Context) {
	// ========== Step 1: 解析参数 ==========
	id := c.Param("id")
	if id == "" {
		controller.HandleError(c, core.ErrBadRequest, http.StatusBadRequest)
		return
	}

	// ========== Step 2: 查询工作流（验证存在性）==========
	ctx := c.Request.Context()
	_, err := controller.workflowService.FindByID(ctx, id)
	if err != nil {
		if err == core.ErrNotFound {
			controller.HandleError(c, err, http.StatusNotFound)
		} else {
			controller.HandleError(c, err, http.StatusInternalServerError)
		}
		return
	}

	// ========== Step 3: 解密Token并生成完整URL ==========
	// 🔥 解密存储的Token
	decryptedToken, err := controller.workflowService.DecryptWebhookToken(ctx, id)
	if err != nil {
		controller.HandleError(c, err, http.StatusInternalServerError)
		return
	}

	// 🔥 使用解密后的Token构建完整URL
	baseURL := getBaseURL(c)
	fullURL := fmt.Sprintf("%s/api/v1/workflow/%s/webhook?key=%s", baseURL, id, decryptedToken)

	response := map[string]interface{}{
		"webhook_url": fullURL,
	}

	controller.HandleOK(c, response)
}

// ========== 辅助函数 ==========

// getBaseURL 获取API服务器的基础URL
//
// 从请求中动态获取协议、主机和端口
//
// 返回示例：
//   - https://api.example.com
//   - http://localhost:8080
func getBaseURL(c *gin.Context) string {
	// 优先使用X-Forwarded-Proto和X-Forwarded-Host
	scheme := c.GetHeader("X-Forwarded-Proto")
	if scheme == "" {
		if c.Request.TLS != nil {
			scheme = "https"
		} else {
			scheme = "http"
		}
	}

	host := c.GetHeader("X-Forwarded-Host")
	if host == "" {
		host = c.Request.Host
	}

	return scheme + "://" + host
}

// maskWebhookURL 脱敏Webhook URL（将Token替换为****）
func maskWebhookURL(url string) string {
	if url == "" {
		return ""
	}

	// 如果URL包含?key=xxx，将token部分替换为****
	if strings.Contains(url, "?key=") {
		parts := strings.Split(url, "?key=")
		if len(parts) == 2 {
			return parts[0] + "?key=****"
		}
	}

	return url
}

// maskToken 脱敏Token（只显示前4位）
func maskToken(token string) string {
	if token == "" {
		return ""
	}

	if len(token) > 4 {
		return token[:4] + "****"
	}

	return "****"
}

// buildTriggerSource 构建触发来源信息
//
// 记录Webhook触发的来源信息，用于审计和调试
func buildTriggerSource(c *gin.Context) map[string]interface{} {
	return map[string]interface{}{
		"ip":         c.ClientIP(),
		"user_agent": c.GetHeader("User-Agent"),
		"referrer":   c.GetHeader("Referer"),
		"request_id": c.GetHeader("X-Request-ID"),
	}
}

// logTriggerSource 记录触发来源到日志
func logTriggerSource(workflowID string, source map[string]interface{}) {
	sourceJSON, _ := json.Marshal(source)
	logger.Info("Webhook触发来源",
		zap.String("workflow_id", workflowID),
		zap.String("source", string(sourceJSON)))
}

// CreateWebhookCronJob 一键创建Webhook定时任务
//
// @Summary 一键创建Webhook定时任务
// @Description 自动为工作流的Webhook创建定时任务（需要先启用Webhook）
// @Tags webhook
// @Accept json
// @Produce json
// @Param id path string true "工作流ID"
// @Param X-TEAM-ID header string true "团队ID"
// @Param body body forms.CreateWebhookCronJobForm false "创建参数（可选）"
// @Success 200 {object} map[string]interface{} "创建成功，返回凭证和定时任务信息"
// @Failure 400 {object} core.ErrorResponse "参数错误"
// @Failure 404 {object} core.ErrorResponse "工作流不存在"
// @Failure 500 {object} core.ErrorResponse "服务器错误"
// @Router /workflow/{id}/webhook/cronjob [post]
//
// 功能说明：
// 1. 验证工作流Webhook是否已启用
// 2. 创建凭证存储webhook_url（加密存储）
// 3. 创建定时任务定期调用webhook
//
// 请求示例：
//
//	POST /api/v1/workflow/{id}/webhook/cronjob
//	X-TEAM-ID: uuid-xxx
//	Content-Type: application/json
//
//	{
//	  "time": "0 0 0 1 * * *",             // 可选，7段格式，默认每月1号0点
//	  "credential_name": "工作流A:webhook",  // 可选
//	  "cronjob_name": "工作流A:计划任务",     // 可选
//	  "description": "定期触发工作流",        // 可选
//	  "is_active": false                   // 可选，默认false（不激活）
//	}
//
// 响应示例：
//
//	{
//	  "code": 200,
//	  "message": "Webhook定时任务创建成功",
//	  "data": {
//	    "credential": {
//	      "id": "credential-uuid",
//	      "name": "工作流A:webhook",
//	      "category": "webhook"
//	    },
//	    "cronjob": {
//	      "id": "cronjob-uuid",
//	      "name": "工作流A:计划任务",
//	      "time": "0 0 1 * *",
//	      "is_active": true
//	    }
//	  }
//	}
func (controller *WorkflowWebhookController) CreateWebhookCronJob(c *gin.Context) {
	// ========== Step 1: 获取工作流ID ==========
	workflowID := c.Param("id")
	if workflowID == "" {
		controller.HandleError(c, fmt.Errorf("工作流ID不能为空"), http.StatusBadRequest)
		return
	}

	// ========== Step 2: 解析请求参数 ==========
	var form forms.CreateWebhookCronJobForm
	if err := c.ShouldBindJSON(&form); err != nil {
		logger.Warn("解析请求参数失败", zap.Error(err))
		// 如果没有body也是可以的，使用默认值
	}

	// 验证表单
	if err := form.Validate(); err != nil {
		controller.HandleError(c, err, http.StatusBadRequest)
		return
	}

	// ========== Step 3: 获取API服务器的BaseURL ==========
	// 从请求中获取协议和Host
	scheme := "http"
	if c.Request.TLS != nil {
		scheme = "https"
	}
	// 优先使用 X-Forwarded-Proto 头（反向代理场景）
	if proto := c.GetHeader("X-Forwarded-Proto"); proto != "" {
		scheme = proto
	}

	baseURL := fmt.Sprintf("%s://%s", scheme, c.Request.Host)

	// ========== Step 4: 调用Service创建Webhook定时任务 ==========
	credential, cronJob, err := controller.workflowService.CreateWebhookCronJob(
		c.Request.Context(),
		workflowID,
		baseURL,
		form.Time,
		form.CredentialName,
		form.CronJobName,
		form.Description,
		form.GetDefaultIsActive(), // 默认false（不激活）
	)

	if err != nil {
		logger.Error("创建Webhook定时任务失败",
			zap.Error(err),
			zap.String("workflow_id", workflowID))

		// 根据错误类型返回不同的状态码
		if strings.Contains(err.Error(), "未启用") {
			controller.HandleError(c, err, http.StatusBadRequest)
		} else if strings.Contains(err.Error(), "不存在") {
			controller.HandleError(c, err, http.StatusNotFound)
		} else if strings.Contains(err.Error(), "已存在") {
			// 🔥 名称冲突，返回 400 Bad Request
			controller.HandleError(c, err, http.StatusBadRequest)
		} else {
			controller.HandleError(c, err, http.StatusInternalServerError)
		}
		return
	}

	// ========== Step 5: 构建返回数据 ==========
	result := map[string]interface{}{
		"credential": map[string]interface{}{
			"id":          credential.ID.String(),
			"name":        credential.Name,
			"category":    credential.Category,
			"description": credential.Description,
			"project":     credential.Project,
			"is_active":   credential.IsActive,
			"created_at":  credential.CreatedAt,
		},
		"cronjob": map[string]interface{}{
			"id":          cronJob.ID.String(),
			"name":        cronJob.Name,
			"time":        cronJob.Time,
			"command":     cronJob.Command,
			"description": cronJob.Description,
			"category":    cronJob.Category,
			"project":     cronJob.Project,
			"is_active":   cronJob.IsActive,
			"created_at":  cronJob.CreatedAt,
		},
	}

	logger.Info("Webhook定时任务创建成功",
		zap.String("workflow_id", workflowID),
		zap.String("credential_id", credential.ID.String()),
		zap.String("cronjob_id", cronJob.ID.String()))

	controller.HandleOK(c, result)
}
