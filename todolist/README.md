# TodoList API Server

一个基于 Go + Gin 的简洁高效待办事项管理系统，严格按照企业级项目架构设计，可作为其他项目的模板基础。

## 🚀 快速开始

### 1. 构建和运行

```bash
# 构建项目
go build -o todolist .

# 运行服务
./todolist
```

### 2. 访问 Swagger 文档

```
http://localhost:8080/swagger/index.html
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
go build -o todolist .
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

- **待办事项管理 (TodoList)** - 完整的CRUD操作，支持状态管理
- **用户认证** - JWT和API Key双重认证支持
- **健康检查** - 系统状态监控
- **Swagger文档** - 完整的API文档

### 系统功能

- **用户隔离** - 每个用户只能访问自己的待办事项
- **状态管理** - 支持待办、进行中、已完成、已取消四种状态
- **分类标签** - 支持分类和标签管理
- **优先级** - 1-5级优先级支持
- **截止日期** - 支持设置和管理截止日期
- **统计信息** - 提供各状态的统计数据

## 🏗️ 系统架构

```
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│   Web UI/CLI    │    │   Mobile App    │    │   Dashboard     │
└─────────┬───────┘    └─────────┬───────┘    └─────────┬───────┘
          │                      │                      │
          └──────────────────────┼──────────────────────┘
                                 │
                    ┌─────────────┴─────────────┐
                    │     TodoList API          │
                    │  (待办事项管理系统)         │
                    └─────────────┬─────────────┘
                                 │
              ┌──────────────────┼──────────────────┐
              │                  │                  │
    ┌─────────┴───────┐ ┌────────┴────────┐ ┌──────┴──────┐
    │     MySQL       │ │      Redis      │ │  Auth API   │
    │   (数据存储)     │ │    (缓存)       │ │  (认证服务)  │
    └─────────────────┘ └─────────────────┘ └─────────────┘
```

## 📚 API 接口

### 待办事项管理 (todolist)
- `POST /api/v1/todolist/` - 创建待办事项
- `GET /api/v1/todolist/` - 获取待办事项列表（支持分页、过滤、搜索、排序）
- `GET /api/v1/todolist/stats/` - 获取统计信息
- `GET /api/v1/todolist/{id}/` - 获取待办事项详情
- `PUT /api/v1/todolist/{id}/` - 更新待办事项
- `DELETE /api/v1/todolist/{id}/` - 删除待办事项
- `PATCH /api/v1/todolist/{id}/` - 部分更新待办事项
- `PUT /api/v1/todolist/{id}/status/` - 更新待办事项状态

### 系统接口
- `GET /api/v1/health/` - 系统健康检查
- `GET /` - 服务状态检查

### 认证管理
- `DELETE /api/v1/auth-cache/` - 清空认证缓存（管理员）
- `GET /api/v1/auth-cache/stats/` - 获取认证缓存统计（管理员）

## 🛠️ 项目结构

```
todolist/
├── main.go                 # 程序入口
├── go.mod                  # Go 模块定义
├── go.sum                  # 依赖版本锁定
├── README.md               # 项目说明
├── .gitignore             # Git 忽略文件
├── docs/                   # Swagger 文档
├── pkg/                    # 核心代码包
│   ├── app/               # 应用程序核心
│   │   ├── app.go         # 应用启动
│   │   └── router.go      # 路由配置
│   ├── config/            # 配置管理
│   ├── core/              # 核心模型和接口
│   │   ├── todolist.go    # 待办事项模型
│   │   ├── auth.go        # 认证模型
│   │   ├── db.go          # 数据库连接
│   │   ├── redis.go       # Redis 连接
│   │   ├── errors.go      # 错误定义
│   │   └── migrate.go     # 数据库迁移
│   ├── store/             # 数据访问层
│   │   └── todolist.go    # 待办事项存储
│   ├── services/          # 业务逻辑层
│   │   ├── todolist.go    # 待办事项服务
│   │   └── auth.go        # 认证服务
│   ├── controllers/       # HTTP 处理层
│   │   ├── forms/         # 表单验证
│   │   ├── todolist.go    # 待办事项控制器
│   │   └── health.go      # 健康检查控制器
│   ├── middleware/        # 中间件
│   │   ├── auth.go        # 认证中间件
│   │   └── auth_helpers.go # 认证辅助函数
│   └── utils/             # 工具包
│       ├── controllers/   # 控制器工具
│       ├── filters/       # 过滤器工具
│       ├── logger/        # 日志工具
│       ├── tools/         # 通用工具
│       └── types/         # 类型定义
├── logs/                  # 日志文件
├── temp/                  # 临时文件
└── build/                 # 构建文件
```

## 🔧 配置说明

系统支持以下配置：

- **数据库**: MySQL/PostgreSQL
- **缓存**: Redis (用于认证缓存和会话存储)
- **日志**: 支持文件轮转
- **认证**: JWT Token 和 API Key 双重支持

## 📊 监控和运维

- **健康检查**: `GET /api/v1/health/`
- **日志管理**: 支持结构化日志和文件轮转
- **认证缓存**: 支持缓存管理和统计

## 🎯 设计理念

### 作为模板项目
- **标准架构**: 严格按照企业级项目架构设计
- **完整功能**: 包含认证、CRUD、监控等完整功能
- **易于扩展**: 可快速复制并扩展为其他业务系统
- **最佳实践**: 遵循Go和Gin的最佳实践

### 核心特性
- **用户隔离**: 通过中间件自动获取用户ID，确保数据隔离
- **统一响应**: 标准化的API响应格式
- **完整验证**: 表单验证和业务逻辑验证
- **错误处理**: 统一的错误处理机制
- **文档完整**: 完整的Swagger文档支持

## 🚀 开发流程

```bash
# 1. 添加新的 API 函数并添加 Swagger 注释
# 注意：@Router 注释使用相对路径，不包含 /api/v1 前缀
# 例如：@Router /todolist/ [post] 而不是 @Router /api/v1/todolist/ [post]

# 2. 重新生成 Swagger 文档
swag init

# 3. 构建和测试
go build -o todolist .
./todolist

# 4. 访问 Swagger UI 验证
# http://localhost:8080/swagger/index.html
```

## 📝 使用示例

### 创建待办事项

```bash
curl -X POST http://localhost:8080/api/v1/todolist/ \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer YOUR_JWT_TOKEN" \
  -d '{
    "title": "完成项目文档",
    "description": "编写完整的API文档和使用说明",
    "priority": 3,
    "category": "工作",
    "tags": "文档,重要",
    "due_date": "2024-12-31T23:59:59Z"
  }'
```

### 获取待办事项列表

```bash
curl -X GET "http://localhost:8080/api/v1/todolist/?status=pending&page=1&page_size=10" \
  -H "Authorization: Bearer YOUR_JWT_TOKEN"
```

### 更新待办事项状态

```bash
curl -X PUT http://localhost:8080/api/v1/todolist/{id}/status/ \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer YOUR_JWT_TOKEN" \
  -d '{"status": "completed"}'
```

---

**端口**: 8080 | **版本**: 1.0.0 | **协议**: HTTP | **文档**: Swagger

> 这个项目可以作为其他Web项目的模板基础，提供了完整的项目架构和最佳实践示例。
