// Package main TodoList API Server 主程序
//
// 这是一个简洁高效的待办事项管理系统API服务器，提供以下核心功能：
// 1. 待办事项管理 (TodoList) - 支持增删改查操作
// 2. 用户认证 - 支持JWT和API Key两种认证方式
// 3. 健康检查 - 提供系统状态监控
// 4. Swagger文档 - 完整的API文档支持
//
// 系统架构：
// - API Server: 负责待办事项管理和用户认证
// - MySQL/PostgreSQL: 持久化存储待办事项数据
// - Redis: 提供认证缓存和会话存储
//
// 设计理念：
// - 严格按照apiserver项目布局设计
// - 标准的CRUD操作模式
// - 完整的认证和权限控制
// - 可作为其他项目的模板基础

// @title           TodoList API
// @version         1.0.0
// @description     简洁高效的待办事项管理系统API服务器，提供完整的CRUD操作和用户认证功能
// @termsOfService  http://swagger.io/terms/

// @contact.name   API Support
// @contact.url    http://www.example.com/support
// @contact.email  support@example.com

// @license.name  Apache 2.0
// @license.url   http://www.apache.org/licenses/LICENSE-2.0.html

// @BasePath  /api/v1

// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
// @description JWT token, format: Bearer {token}

// @securityDefinitions.apikey TeamAuth
// @in header
// @name X-TEAM-ID
// @description Team ID for team-scoped operations (optional)
package main

import (
	"github.com/codelieche/todolist/pkg/app"
	"github.com/codelieche/todolist/pkg/utils/logger"
)

// main 程序入口点
// 启动API服务器，包括：
// 1. 初始化日志系统
// 2. 启动Web服务器
// 3. 初始化数据库连接和迁移
// 4. 配置路由和中间件
func main() {
	logger.Info("TodoList API Server 启动中...")

	// 注意：权限和平台配置已统一由 usercenter 管理

	app.Run()
	logger.Info("TodoList API Server 已停止")
}
