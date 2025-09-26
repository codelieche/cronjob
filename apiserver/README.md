# 计划任务系统 API Server

分布式计划任务系统的API服务器，提供定时任务管理、工作节点管理、任务调度等功能。

## 🚀 快速开始

### 1. 构建和运行

```bash
# 构建项目
go build -o apiserver .

# 运行服务
./apiserver
```

### 2. 访问 Swagger 文档

```
http://localhost:8000/swagger/index.html
```

## 📖 Swagger 文档

### 重新生成文档

**重要**: 每次添加新的 API 接口或修改 Swagger 注释后，都需要重新生成文档：

```bash
# 安装 swag 工具（如果尚未安装）
go install github.com/swaggo/swag/cmd/swag@latest

# 重新生成 Swagger 文档
swag init

# 构建项目
go build -o apiserver .
```

### 需要重新生成的情况

- ✅ 添加新的 API 接口函数
- ✅ 修改现有的 Swagger 注释（`@Summary`、`@Description`、`@Param` 等）
- ✅ 修改数据模型结构体
- ✅ 修改路由路径（`@Router` 注释）
- ❌ 只修改业务逻辑代码（不涉及 Swagger 注释）

### 生成的文件

运行 `swag init` 会重新生成以下文件：

```
docs/
├── docs.go          # Go 代码格式的文档定义
├── swagger.json     # JSON 格式的 OpenAPI 规范
└── swagger.yaml     # YAML 格式的 OpenAPI 规范
```

> **注意**: 不要手动编辑 `docs/` 目录下的文件，它们会被 `swag init` 覆盖。

## 🔐 认证方式

系统使用 JWT Bearer Token 认证：

```
Authorization: Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...
```

## 📋 主要功能

### 核心模块

- **定时任务管理 (CronJob)** - 支持cron表达式的时间调度
- **任务执行记录 (Task)** - 记录每次任务执行的详细信息
- **工作节点管理 (Worker)** - 管理执行任务的工作节点
- **任务分类管理 (Category)** - 任务类型分类管理
- **任务日志管理 (TaskLog)** - 任务执行日志管理

### 系统功能

- **分布式锁机制** - 基于Redis的分布式锁，确保任务不重复执行
- **WebSocket实时通信** - 与Worker节点进行实时任务分发和状态同步
- **任务调度服务** - 自动根据cron表达式创建和执行任务
- **系统健康检查** - 监控系统运行状态
- **Prometheus监控** - 提供监控指标

## 🏗️ 系统架构

```
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│   Web UI/CLI    │    │   API Client    │    │   Dashboard     │
└─────────┬───────┘    └─────────┬───────┘    └─────────┬───────┘
          │                      │                      │
          └──────────────────────┼──────────────────────┘
                                 │
                    ┌─────────────┴─────────────┐
                    │     API Server            │
                    │  (计划任务系统核心)         │
                    └─────────────┬─────────────┘
                                 │
              ┌──────────────────┼──────────────────┐
              │                  │                  │
    ┌─────────┴───────┐ ┌────────┴────────┐ ┌──────┴──────┐
    │    Worker-1     │ │    Worker-2     │ │   Worker-N  │
    │  (任务执行节点)   │ │  (任务执行节点)   │ │ (任务执行节点) │
    └─────────────────┘ └─────────────────┘ └─────────────┘
```

## 📚 API 接口分组

### 定时任务管理 (cronjobs)
- `POST /api/v1/cronjob/` - 创建定时任务
- `GET /api/v1/cronjob/` - 获取定时任务列表
- `GET /api/v1/cronjob/{id}/` - 获取定时任务详情
- `PUT /api/v1/cronjob/{id}/` - 更新定时任务
- `DELETE /api/v1/cronjob/{id}/` - 删除定时任务
- `PUT /api/v1/cronjob/{id}/toggle-active/` - 切换激活状态
- `POST /api/v1/cronjob/validate-expression/` - 验证cron表达式
- `PATCH /api/v1/cronjob/{id}/` - 部分更新定时任务

### 任务执行记录 (tasks)
- `POST /api/v1/task/` - 创建任务记录
- `GET /api/v1/task/` - 获取任务记录列表
- `GET /api/v1/task/{id}/` - 获取任务记录详情
- `PUT /api/v1/task/{id}/` - 更新任务记录
- `DELETE /api/v1/task/{id}/` - 删除任务记录
- `PUT /api/v1/task/{id}/update-status/` - 更新任务状态
- `PUT /api/v1/task/{id}/update-output/` - 更新任务输出
- `PATCH /api/v1/task/{id}/` - 部分更新任务记录

### 工作节点管理 (workers)
- `POST /api/v1/worker/` - 注册工作节点
- `GET /api/v1/worker/` - 获取工作节点列表
- `GET /api/v1/worker/{id}/` - 获取工作节点详情
- `PUT /api/v1/worker/{id}/` - 更新工作节点信息
- `DELETE /api/v1/worker/{id}/` - 注销工作节点
- `GET /api/v1/worker/{id}/ping/` - 工作节点心跳

### 任务分类管理 (categories)
- `POST /api/v1/category/` - 创建分类
- `GET /api/v1/category/` - 获取分类列表
- `GET /api/v1/category/{id}/` - 获取分类详情
- `PUT /api/v1/category/{id}/` - 更新分类
- `DELETE /api/v1/category/{id}/` - 删除分类

### 系统接口
- `GET /api/v1/health/` - 系统健康检查
- `GET /metrics` - Prometheus监控指标
- `GET /api/v1/ws/task/` - WebSocket连接
- `GET /api/v1/lock/*` - 分布式锁管理

## 🛠️ 开发流程

```bash
# 1. 添加新的 API 函数并添加 Swagger 注释
# 注意：@Router 注释使用相对路径，不包含 /api/v1 前缀
# 例如：@Router /cronjob/ [post] 而不是 @Router /api/v1/cronjob/ [post]

# 2. 重新生成 Swagger 文档
swag init

# 3. 构建和测试
go build -o apiserver .
./apiserver

# 4. 访问 Swagger UI 验证
# http://localhost:8000/swagger/index.html
```

## 🔧 配置说明

系统支持以下配置：

- **数据库**: MySQL/PostgreSQL
- **缓存**: Redis (用于分布式锁)
- **日志**: 支持文件轮转
- **监控**: Prometheus 指标
- **认证**: JWT Token

## 📊 监控和运维

- **健康检查**: `GET /api/v1/health/`
- **监控指标**: `GET /metrics`
- **日志管理**: 支持结构化日志和文件轮转
- **分布式锁**: 基于Redis，防止任务重复执行

---

**端口**: 8000 | **版本**: 1.0.0 | **协议**: HTTP/WebSocket
