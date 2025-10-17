// Package services 注册服务
//
// 提供注册相关的业务逻辑，包括：
// - 向usercenter注册权限和角色
// - 向usercenter注册平台配置
package services

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/codelieche/cronjob/apiserver/pkg/config"
	"github.com/codelieche/cronjob/apiserver/pkg/core"
	"github.com/codelieche/cronjob/apiserver/pkg/utils/logger"
	"go.uber.org/zap"
)

// registryService 注册服务实现
type registryService struct {
	httpClient      *http.Client
	authConfig      *config.AuthConfig
	categoryService core.CategoryService
}

// NewRegistryService 创建注册服务实例
func NewRegistryService() core.RegistryService {
	return &registryService{
		httpClient: &http.Client{
			Timeout: config.Auth.Timeout,
			Transport: &http.Transport{
				MaxIdleConns:        config.Auth.MaxIdleConns,
				MaxIdleConnsPerHost: 5,
				IdleConnTimeout:     config.Auth.IdleConnTimeout,
			},
		},
		authConfig: config.Auth,
	}
}

// NewRegistryServiceWithCategory 创建带CategoryService的注册服务实例
func NewRegistryServiceWithCategory(categoryService core.CategoryService) core.RegistryService {
	return &registryService{
		httpClient: &http.Client{
			Timeout: config.Auth.Timeout,
			Transport: &http.Transport{
				MaxIdleConns:        config.Auth.MaxIdleConns,
				MaxIdleConnsPerHost: 5,
				IdleConnTimeout:     config.Auth.IdleConnTimeout,
			},
		},
		authConfig:      config.Auth,
		categoryService: categoryService,
	}
}

// RegisterPermissions 注册权限和角色到用户中心
func (s *registryService) RegisterPermissions(ctx context.Context, req *core.PermissionRegistryRequest) (*core.PermissionRegistryResponse, error) {
	url := fmt.Sprintf("%s/registry/permissions/", s.authConfig.ApiUrl)

	// 序列化请求数据
	jsonData, err := json.Marshal(req)
	if err != nil {
		logger.Error("序列化权限注册请求失败", zap.Error(err))
		return nil, fmt.Errorf("序列化请求数据失败: %w", err)
	}

	// 发送HTTP请求
	resp, err := s.sendHTTPRequest(ctx, "POST", url, jsonData)
	if err != nil {
		return nil, fmt.Errorf("发送权限注册请求失败: %w", err)
	}

	// 解析响应
	var response core.PermissionRegistryResponse
	if err := json.Unmarshal(resp, &response); err != nil {
		logger.Error("解析权限注册响应失败", zap.Error(err), zap.String("response", string(resp)))
		return nil, fmt.Errorf("解析响应数据失败: %w", err)
	}

	logger.Info("权限注册成功",
		zap.String("system_code", req.SystemCode),
		zap.Int("permissions", len(req.Permissions)),
		zap.Int("roles", len(req.Roles)),
	)

	return &response, nil
}

// RegisterPlatforms 注册平台到用户中心
func (s *registryService) RegisterPlatforms(ctx context.Context, req *core.PlatformRegistryRequest) (*core.PlatformRegistryResponse, error) {
	url := fmt.Sprintf("%s/registry/platforms/", s.authConfig.ApiUrl)

	// 序列化请求数据
	jsonData, err := json.Marshal(req)
	if err != nil {
		logger.Error("序列化平台注册请求失败", zap.Error(err))
		return nil, fmt.Errorf("序列化请求数据失败: %w", err)
	}

	// 发送HTTP请求
	resp, err := s.sendHTTPRequest(ctx, "POST", url, jsonData)
	if err != nil {
		return nil, fmt.Errorf("发送平台注册请求失败: %w", err)
	}

	// 解析响应
	var response core.PlatformRegistryResponse
	if err := json.Unmarshal(resp, &response); err != nil {
		logger.Error("解析平台注册响应失败", zap.Error(err), zap.String("response", string(resp)))
		return nil, fmt.Errorf("解析响应数据失败: %w", err)
	}

	logger.Info("平台注册成功",
		zap.String("system_code", req.SystemCode),
		zap.Int("platforms", len(req.Platforms)),
	)

	return &response, nil
}

// sendHTTPRequest 发送HTTP请求（带重试机制）
func (s *registryService) sendHTTPRequest(ctx context.Context, method, url string, data []byte) ([]byte, error) {
	var lastErr error

	for i := 0; i <= s.authConfig.MaxRetries; i++ {
		if i > 0 {
			// 等待重试延迟
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(s.authConfig.RetryInterval * time.Duration(i)):
			}

			logger.Warn("重试HTTP请求",
				zap.String("method", method),
				zap.String("url", url),
				zap.Int("attempt", i+1),
				zap.Error(lastErr),
			)
		}

		// 创建HTTP请求
		req, err := http.NewRequestWithContext(ctx, method, url, bytes.NewBuffer(data))
		if err != nil {
			lastErr = fmt.Errorf("创建HTTP请求失败: %w", err)
			continue
		}

		// 设置请求头
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("User-Agent", "todolist/1.0")

		// 如果配置了API Key，添加到请求头
		if s.authConfig.ApiKey != "" {
			req.Header.Set("Authorization", "Bearer "+s.authConfig.ApiKey)
		}

		// 发送请求
		resp, err := s.httpClient.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("发送HTTP请求失败: %w", err)
			continue
		}

		// 读取响应体
		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()

		if err != nil {
			lastErr = fmt.Errorf("读取响应体失败: %w", err)
			continue
		}

		// 检查HTTP状态码
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			lastErr = fmt.Errorf("HTTP请求失败，状态码: %d, 响应: %s", resp.StatusCode, string(body))

			// 对于客户端错误（4xx），不进行重试
			if resp.StatusCode >= 400 && resp.StatusCode < 500 {
				break
			}
			continue
		}

		// 请求成功
		logger.Debug("HTTP请求成功",
			zap.String("method", method),
			zap.String("url", url),
			zap.Int("status_code", resp.StatusCode),
			zap.Int("attempt", i+1),
		)

		return body, nil
	}

	logger.Error("HTTP请求最终失败",
		zap.String("method", method),
		zap.String("url", url),
		zap.Int("max_attempts", s.authConfig.MaxRetries+1),
		zap.Error(lastErr),
	)

	return nil, lastErr
}

// getSystemCategories 获取系统分类定义
//
// 核心Runner类型分类：
// - default: 默认分类，使用命令执行器
// - command: 命令执行器，执行Shell命令
// - http: HTTP请求执行器，用于API调用
// - script: 脚本执行器，支持Python/Shell/JavaScript等脚本
// - database: 数据库操作执行器，支持MySQL/PostgreSQL/MongoDB等
// - message: 消息发送执行器，支持邮件/钉钉/企业微信/短信等
// - file: 文件操作执行器，支持文件清理/备份/压缩/统计等操作
// - git: Git 仓库操作执行器，支持智能 sync（clone/pull）
// - approval: 审批流程执行器，支持人工审批和AI自动审批
func (s *registryService) getSystemCategories() []*core.Category {
	return []*core.Category{
		{
			Code:        "default",
			Name:        "默认分类",
			Description: "默认分类，使用命令执行器（CommandRunner）执行Shell命令。适用于简单的命令行任务。",
			Setup:       "", // TODO: 后续补充初始化脚本
			Teardown:    "", // TODO: 后续补充销毁脚本
			Check:       "", // TODO: 后续补充检查脚本
		},
		{
			Code:        "command",
			Name:        "命令执行",
			Description: "命令执行器，使用bash执行Shell命令。支持管道、重定向等Shell特性，适合系统运维任务。",
			Setup:       "", // TODO: 后续补充初始化脚本
			Teardown:    "", // TODO: 后续补充销毁脚本
			Check:       "", // TODO: 后续补充检查脚本
		},
		{
			Code:        "http",
			Name:        "HTTP请求",
			Description: "HTTP请求执行器，支持HTTP/HTTPS请求。适用于API调用、Webhook触发、健康检查等场景。支持GET/POST/PUT/DELETE等方法，可配置Headers、Query参数、Body、超时、重试等。",
			Setup:       "", // TODO: 后续补充初始化脚本（如：检查网络连接）
			Teardown:    "", // TODO: 后续补充销毁脚本（如：清理临时文件）
			Check:       "", // TODO: 后续补充检查脚本（如：验证HTTP服务可用性）
		},
		{
			Code:        "script",
			Name:        "脚本执行",
			Description: "脚本执行器，支持Python、Shell、JavaScript、Ruby等脚本语言。可配置脚本内容、运行环境、依赖包等。",
			Setup:       "", // TODO: 后续补充初始化脚本（如：安装依赖包）
			Teardown:    "", // TODO: 后续补充销毁脚本（如：清理虚拟环境）
			Check:       "", // TODO: 后续补充检查脚本（如：检查解释器版本）
		},
		{
			Code:        "database",
			Name:        "数据库操作",
			Description: "数据库操作执行器，支持MySQL、PostgreSQL、MongoDB、Redis等数据库。可执行SQL查询、数据备份、数据同步等操作。",
			Setup:       "", // TODO: 后续补充初始化脚本（如：建立数据库连接）
			Teardown:    "", // TODO: 后续补充销毁脚本（如：关闭数据库连接）
			Check:       "", // TODO: 后续补充检查脚本（如：测试数据库连接）
		},
		{
			Code:        "message",
			Name:        "消息发送",
			Description: "消息发送执行器，支持邮件、钉钉、企业微信、Slack、短信等多种消息渠道。适用于通知提醒、告警推送等场景。",
			Setup:       "", // TODO: 后续补充初始化脚本（如：验证消息服务配置）
			Teardown:    "", // TODO: 后续补充销毁脚本（如：关闭消息连接）
			Check:       "", // TODO: 后续补充检查脚本（如：测试消息发送能力）
		},
		{
			Code:        "file",
			Name:        "文件操作",
			Description: "文件操作执行器，支持本地和远程文件操作（纯SSH）。提供清理（cleanup）、备份（backup）、压缩（compress）、统计（stat）等功能。支持时间/大小筛选、DryRun模式、路径安全验证等特性。",
			Setup:       "", // TODO: 后续补充初始化脚本（如：检查文件系统权限）
			Teardown:    "", // TODO: 后续补充销毁脚本（如：清理临时文件）
			Check:       "", // TODO: 后续补充检查脚本（如：验证路径可访问性）
		},
		{
			Code:        "git",
			Name:        "Git 操作",
			Description: "Git 仓库操作执行器，自动拉取代码到任务工作目录。支持智能 sync（自动判断 clone/pull）、SSH 私钥认证、清空模式等特性。极简配置（只需 URL + 分支 + 凭证），适合 CI/CD 流程中的代码拉取场景。",
			Setup:       "",          // TODO: 后续补充初始化脚本
			Teardown:    "",          // TODO: 后续补充销毁脚本
			Check:       "which git", // 检查 git 命令
		},
		{
			Code:        "container",
			Name:        "容器操作",
			Description: "容器操作执行器，支持 Docker 和 containerd 运行时。提供镜像构建/推送/拉取、容器运行/停止/重启、日志查看、命令执行等功能。支持本地 Unix Socket 和远程 TCP 连接（TLS 认证），适合 CI/CD 镜像构建、容器部署、运维管理等场景。",
			Setup:       "",               // TODO: 后续补充初始化脚本
			Teardown:    "",               // TODO: 后续补充销毁脚本
			Check:       "docker version", // 检查 docker 命令
		},
		{
			Code:        "approval",
			Name:        "审批流程",
			Description: "审批流程执行器，支持人工审批和AI自动审批。在工作流中插入审批节点，实现流程控制和风险把控。支持单人审批/多人会签、超时自动处理、审批记录追溯等功能。可配置审批人员（团队成员）、AI智能体、审批策略等。适用于生产发布审批、资源变更审批、代码合并审批等场景。",
			Setup:       "", // 无需初始化脚本
			Teardown:    "", // 无需清理脚本
			Check:       "", // 无需环境检查
		},
	}
}

// RegisterCategories 注册系统分类
func (s *registryService) RegisterCategories(ctx context.Context) error {
	if s.categoryService == nil {
		logger.Error("CategoryService未初始化，无法注册分类")
		return fmt.Errorf("CategoryService未初始化")
	}

	logger.Info("开始注册系统分类")

	// 获取系统分类定义
	categories := s.getSystemCategories()

	// 逐个注册或更新分类
	for _, category := range categories {
		// 检查分类是否已存在
		existing, err := s.categoryService.FindByCode(ctx, category.Code)
		if err != nil && err != core.ErrNotFound {
			logger.Error("查询分类失败",
				zap.String("code", category.Code),
				zap.Error(err),
			)
			continue
		}

		if existing != nil {
			// 更新现有分类
			existing.Name = category.Name
			existing.Description = category.Description
			existing.Setup = category.Setup
			existing.Teardown = category.Teardown
			existing.Check = category.Check

			_, err = s.categoryService.Update(ctx, existing)
			if err != nil {
				logger.Error("更新分类失败",
					zap.String("code", category.Code),
					zap.Error(err),
				)
			} else {
				logger.Info("分类更新成功",
					zap.String("code", category.Code),
					zap.String("name", category.Name),
				)
			}
		} else {
			// 创建新分类
			_, err = s.categoryService.Create(ctx, category)
			if err != nil {
				logger.Error("创建分类失败",
					zap.String("code", category.Code),
					zap.Error(err),
				)
			} else {
				logger.Info("分类创建成功",
					zap.String("code", category.Code),
					zap.String("name", category.Name),
				)
			}
		}
	}

	logger.Info("系统分类注册完成", zap.Int("total", len(categories)))
	return nil
}
