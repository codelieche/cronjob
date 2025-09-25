# Controllers 控制器基础库

一个基于Gin的Web控制器基础工具库，提供统一的HTTP响应处理、错误处理、分页解析和过滤器集成功能。

## 特性

- 🎯 **统一响应格式**: 标准化所有API的响应结构
- 🚨 **统一错误处理**: 提供多种HTTP状态码的错误处理方法
- 📄 **自动分页解析**: 自动解析和验证分页参数
- 🔍 **过滤器集成**: 与filters库完美集成，提供查询过滤功能
- 📊 **审计日志**: 内置审计日志功能，记录用户操作
- ⚡ **高性能**: 优化的分页参数验证和限制
- 🛠️ **易扩展**: 支持自定义配置和中间件

## 核心组件

### 1. BaseController - 基础控制器

所有控制器都应该嵌入此结构体以获得基础功能：

```go
type BaseController struct {
    // 提供统一的HTTP响应处理、错误处理、分页解析和过滤器集成功能
}
```

### 2. 响应处理方法

#### 成功响应
- `HandleOK(c *gin.Context, data interface{})` - 200 OK响应
- `HandleCreated(c *gin.Context, data interface{})` - 201 Created响应
- `HandleNoContent(c *gin.Context)` - 204 No Content响应

#### 错误响应
- `HandleError(c *gin.Context, err error, code int)` - 通用错误响应
- `HandleError400(c *gin.Context, err error)` - 400 Bad Request响应
- `Handle401(c *gin.Context, err error)` - 401 Unauthorized响应
- `Handle404(c *gin.Context, err error)` - 404 Not Found响应
- `HandleError500(c *gin.Context, err error)` - 500 Internal Server Error响应

### 3. 分页处理

- `ParsePagination(c *gin.Context) *types.Pagination` - 解析分页参数
- 自动验证和限制分页参数，防止恶意请求

### 4. 过滤器集成

- `FilterAction(c *gin.Context, ...) []filters.Filter` - 创建过滤器动作组合
- 集成过滤、搜索、排序功能

### 5. 审计日志功能

- `SetAuditLog(c *gin.Context, key string, data interface{}, marsharl bool)` - 发送审计日志
- `LogAudit(c *gin.Context, action AuditAction, resource string, resourceID string, data interface{})` - 记录审计日志
- `LogCreateAudit`, `LogUpdateAudit`, `LogDeleteAudit`, `LogReadAudit` - 便捷的审计日志方法

## 使用示例

### 基础控制器定义

```go
package controllers

import (
    "github.com/codelieche/cronjob/apiserver/pkg/utils/controllers"
    "github.com/gin-gonic/gin"
)

// TaskController 任务控制器
type TaskController struct {
    controllers.BaseController  // 嵌入基础控制器
    service core.TaskService    // 业务服务
}

// NewTaskController 创建任务控制器实例
func NewTaskController(service core.TaskService) *TaskController {
    return &TaskController{
        service: service,
    }
}
```

### 列表接口实现

```go
// List 获取任务列表
func (controller *TaskController) List(c *gin.Context) {
    // 1. 解析分页参数
    pagination := controller.ParsePagination(c)

    // 2. 定义过滤选项
    filterOptions := []*filters.FilterOption{
        {
            QueryKey: "name",
            Column:   "name",
            Op:       filters.FILTER_EQ,
        },
        {
            QueryKey: "status",
            Column:   "status",
            Op:       filters.FILTER_IN,
        },
        {
            QueryKey: "name__contains",
            Column:   "name",
            Op:       filters.FILTER_CONTAINS,
        },
    }

    // 3. 定义搜索和排序字段
    searchFields := []string{"name", "description", "command"}
    orderingFields := []string{"created_at", "name", "status"}
    defaultOrdering := "-created_at"

    // 4. 获取过滤动作
    filterActions := controller.FilterAction(c, filterOptions, searchFields, orderingFields, defaultOrdering)

    // 5. 计算偏移量
    offset := (pagination.Page - 1) * pagination.PageSize

    // 6. 获取数据
    tasks, err := controller.service.List(c.Request.Context(), offset, pagination.PageSize, filterActions...)
    if err != nil {
        controller.HandleError(c, err, http.StatusBadRequest)
        return
    }

    // 7. 获取总数
    count, err := controller.service.Count(c.Request.Context(), filterActions...)
    if err != nil {
        controller.HandleError(c, err, http.StatusBadRequest)
        return
    }

    // 8. 构建分页结果
    result := &types.ResponseList{
        Page:     pagination.Page,
        PageSize: pagination.PageSize,
        Count:    count,
        Results:  tasks,
    }

    // 9. 返回成功响应
    controller.HandleOK(c, result)
}
```

### 创建接口实现

```go
// Create 创建任务
func (controller *TaskController) Create(c *gin.Context) {
    // 1. 处理表单
    var form forms.TaskCreateForm
    if err := c.ShouldBind(&form); err != nil {
        controller.HandleError(c, err, http.StatusBadRequest)
        return
    }

    // 2. 表单校验
    if err := form.Validate(); err != nil {
        controller.HandleError(c, err, http.StatusBadRequest)
        return
    }

    // 3. 创建对象
    task := form.ToTask()

    // 4. 调用服务创建
    createdTask, err := controller.service.Create(c.Request.Context(), task)
    if err != nil {
        if err == core.ErrConflict {
            controller.HandleError(c, err, http.StatusConflict)
        } else {
            controller.HandleError(c, err, http.StatusBadRequest)
        }
        return
    }

    // 5. 记录审计日志
    controller.LogCreateAudit(c, "task", createdTask.ID.String(), createdTask)

    // 6. 返回成功响应
    controller.HandleCreated(c, createdTask)
}
```

### 详情接口实现

```go
// Find 获取任务详情
func (controller *TaskController) Find(c *gin.Context) {
    // 1. 获取ID
    id := c.Param("id")

    // 2. 调用服务获取
    task, err := controller.service.FindByID(c.Request.Context(), id)
    if err != nil {
        if err == core.ErrNotFound {
            controller.Handle404(c, err)
        } else {
            controller.HandleError(c, err, http.StatusBadRequest)
        }
        return
    }

    // 3. 记录审计日志
    controller.LogReadAudit(c, "task", id, task)

    // 4. 返回成功响应
    controller.HandleOK(c, task)
}
```

### 删除接口实现

```go
// Delete 删除任务
func (controller *TaskController) Delete(c *gin.Context) {
    // 1. 获取ID
    id := c.Param("id")

    // 2. 调用服务删除
    err := controller.service.DeleteByID(c.Request.Context(), id)
    if err != nil {
        if err == core.ErrNotFound {
            controller.Handle404(c, err)
        } else {
            controller.HandleError(c, err, http.StatusBadRequest)
        }
        return
    }

    // 3. 记录审计日志
    controller.LogDeleteAudit(c, "task", id, map[string]interface{}{
        "deleted_at": time.Now(),
    })

    // 4. 返回成功响应
    controller.HandleNoContent(c)
}
```

## 响应格式

### 成功响应格式

```json
{
    "code": 0,
    "data": {
        // 实际数据
    },
    "message": "ok"
}
```

### 列表响应格式

```json
{
    "code": 0,
    "data": {
        "page": 1,
        "page_size": 10,
        "count": 100,
        "results": [
            // 列表数据
        ]
    },
    "message": "ok"
}
```

### 错误响应格式

```json
{
    "code": 400,
    "message": "请求参数错误"
}
```

## 分页配置

### 默认配置

```go
defaultPaginationConfig := &types.PaginationConfig{
    MaxPage:            1000,  // 最大页数限制
    PageQueryParam:     "page", // 页码参数名
    MaxPageSize:        300,   // 每页最大数据量
    PageSizeQueryParam: "page_size", // 每页大小参数名
}
```

### 自定义配置

```go
// 在应用启动时设置自定义配置
customConfig := &types.PaginationConfig{
    MaxPage:            500,
    PageQueryParam:     "p",
    MaxPageSize:        100,
    PageSizeQueryParam: "size",
}
controllers.SetPaginationConfig(customConfig)
```

## API 查询参数示例

### 分页参数
```
GET /api/tasks?page=1&page_size=20
```

### 过滤参数
```
GET /api/tasks?name=test&status=pending,completed&name__contains=task
```

### 搜索参数
```
GET /api/tasks?search=重要任务
```

### 排序参数
```
GET /api/tasks?ordering=-created_at,name
```

### 组合使用
```
GET /api/tasks?name__contains=test&status=pending&search=重要&ordering=-created_at&page=1&page_size=20
```

## 审计日志功能

### 审计日志类型

```go
// 审计操作类型
type AuditAction string

const (
    AuditActionCreate AuditAction = "create"
    AuditActionUpdate AuditAction = "update"
    AuditActionDelete AuditAction = "delete"
    AuditActionRead   AuditAction = "read"
    AuditActionLogin  AuditAction = "login"
    AuditActionLogout AuditAction = "logout"
)

// 审计日志级别
type AuditLevel int

const (
    AuditLevelInfo AuditLevel = iota
    AuditLevelWarning
    AuditLevelError
    AuditLevelCritical
)
```

### 审计日志使用

```go
// 1. 基础审计日志方法
controller.SetAuditLog(c, "create", taskData, true)

// 2. 便捷的审计日志方法
controller.LogCreateAudit(c, "task", taskID, taskData)
controller.LogUpdateAudit(c, "task", taskID, updateData)
controller.LogDeleteAudit(c, "task", taskID, deleteInfo)
controller.LogReadAudit(c, "task", taskID, taskData)

// 3. 自定义审计日志
controller.LogAudit(c, AuditActionLogin, "user", userID, loginData)
```

### 审计日志配置

#### 基础配置
```go
// 设置自定义审计服务
customAuditService := &CustomAuditService{}
controllers.SetAuditService(customAuditService)

// 获取当前审计服务
auditService := controllers.GetAuditService()
```

#### 数据库审计配置
```go
// 方式1：同步数据库审计（推荐用于开发环境）
db := gorm.Open(mysql.Open(dsn), &gorm.Config{})
hook := controllers.NewDatabaseAuditHook(db)
controllers.SetAuditHook(hook)

// 方式2：异步数据库审计（推荐用于生产环境）
hook := controllers.NewAsyncDatabaseAuditHook(db)
controllers.SetAuditHook(hook)

// 方式3：自定义钩子函数
customHook := func(ctx context.Context, log *AuditLog) error {
    // 自定义处理逻辑
    // 1. 保存到数据库
    // 2. 发送到消息队列
    // 3. 记录到文件
    // 4. 发送到外部审计系统
    return nil
}
controllers.SetAuditHook(customHook)
```

#### 审计日志查询和分析
```go
// 查询审计日志
var logs []AuditLog
db.Find(&logs)

// 查询特定用户的审计日志
db.Where("user_id = ?", "user123").Find(&logs)

// 查询特定操作的审计日志
db.Where("action = ?", "create").Find(&logs)

// 统计各种操作的数量
var stats []struct {
    Action string
    Count  int64
}
db.Model(&AuditLog{}).
    Select("action, count(*) as count").
    Group("action").
    Find(&stats)
```

### 审计日志结构

```go
type AuditLog struct {
    ID          string                 `json:"id"`           // 审计日志ID
    Action      AuditAction           `json:"action"`       // 操作类型
    Resource    string                `json:"resource"`     // 资源类型
    ResourceID  string                `json:"resource_id"`  // 资源ID
    UserID      string                `json:"user_id"`      // 用户ID
    Username    string                `json:"username"`     // 用户名
    IP          string                `json:"ip"`           // 客户端IP
    UserAgent   string                `json:"user_agent"`   // 用户代理
    RequestID   string                `json:"request_id"`   // 请求ID
    Data        map[string]interface{} `json:"data"`        // 操作数据
    Level       AuditLevel            `json:"level"`        // 日志级别
    Message     string                `json:"message"`      // 日志消息
    Timestamp   time.Time             `json:"timestamp"`    // 时间戳
    Success     bool                  `json:"success"`      // 操作是否成功
    Error       string                `json:"error"`        // 错误信息（如果有）
}
```

## 错误处理

### 自动错误处理

控制器会自动处理常见的错误类型：

- `core.ErrNotFound` → 404 Not Found
- `core.ErrConflict` → 409 Conflict
- `core.ErrBadRequest` → 400 Bad Request

### 手动错误处理

```go
// 使用通用错误处理方法
controller.HandleError(c, err, http.StatusBadRequest)

// 使用特定错误处理方法
controller.HandleError400(c, err)
controller.Handle401(c, err)
controller.Handle404(c, err)
controller.HandleError500(c, err)
```

## 最佳实践

### 1. 控制器结构

```go
type XxxController struct {
    controllers.BaseController
    service core.XxxService
}
```

### 2. 错误处理顺序

1. 参数验证错误 → `HandleError400`
2. 业务逻辑错误 → `HandleError` 或特定错误方法
3. 系统错误 → `HandleError500`

### 3. 分页使用

```go
// 总是使用ParsePagination解析分页参数
pagination := controller.ParsePagination(c)

// 计算偏移量
offset := (pagination.Page - 1) * pagination.PageSize

// 构建分页响应
result := &types.ResponseList{
    Page:     pagination.Page,
    PageSize: pagination.PageSize,
    Count:    count,
    Results:  data,
}
```

### 4. 过滤器使用

```go
// 定义过滤选项
filterOptions := []*filters.FilterOption{
    // 过滤选项定义
}

// 定义搜索和排序字段
searchFields := []string{"name", "description"}
orderingFields := []string{"created_at", "name"}
defaultOrdering := "-created_at"

// 获取过滤动作
filterActions := controller.FilterAction(c, filterOptions, searchFields, orderingFields, defaultOrdering)
```

## 注意事项

1. **统一响应格式**: 所有接口都应该使用BaseController的响应方法
2. **错误处理**: 根据错误类型选择合适的错误处理方法
3. **分页限制**: 分页参数有最大限制，防止性能问题
4. **参数验证**: 分页参数会自动验证和修正
5. **过滤器安全**: 排序字段需要预先定义，防止SQL注入

## 扩展指南

### 添加新的响应方法

```go
// 在BaseController中添加新方法
func (controller *BaseController) HandleAccepted(c *gin.Context, data interface{}) {
    r := types.Response{
        Code:    0,
        Data:    data,
        Message: "accepted",
    }
    c.JSON(http.StatusAccepted, r)
}
```

### 添加中间件支持

```go
// 在BaseController中添加中间件方法
func (controller *BaseController) WithAuth() gin.HandlerFunc {
    return func(c *gin.Context) {
        // 认证逻辑
        c.Next()
    }
}
```

## 许可证

MIT License
