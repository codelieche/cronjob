# Filters 查询过滤器库

一个基于GORM的查询过滤器库，提供统一的接口来处理RESTful API中的过滤、搜索和排序功能。

## 特性

- 🎯 **统一接口**: 所有过滤器实现相同的`Filter`接口
- 🔍 **多字段搜索**: 支持在多个字段中进行模糊搜索
- 📊 **灵活排序**: 支持多字段排序，升序/降序
- 🔗 **自动关联**: 自动处理表关联查询
- ⚡ **高性能**: 批量处理过滤器，避免重复JOIN
- 🛠️ **易扩展**: 支持自定义操作符和表达式

## 核心组件

### 1. FilterOption - 过滤器选项
定义单个过滤条件的所有必要信息。

```go
type FilterOption struct {
    QueryKey  string      // 查询参数名
    Column    string      // 数据库列名
    Value     interface{} // 过滤值
    Op        int         // 操作符类型
    AllowNull bool        // 是否允许空值
}
```

### 2. 支持的操作符

| 操作符 | 常量 | 说明 | SQL示例 |
|--------|------|------|---------|
| 等于 | `FILTER_EQ` | 精确匹配 | `column = 'value'` |
| 不等于 | `FILTER_NEQ` | 不匹配 | `column != 'value'` |
| 包含 | `FILTER_CONTAINS` | 包含字符串 | `column LIKE '%value%'` |
| 不区分大小写包含 | `FILTER_ICONTAINS` | 不区分大小写包含 | `column ILIKE '%value%'` |
| 大于 | `FILTER_GT` | 数值比较 | `column > value` |
| 大于等于 | `FILTER_GTE` | 数值比较 | `column >= value` |
| 小于 | `FILTER_LT` | 数值比较 | `column < value` |
| 小于等于 | `FILTER_LTE` | 数值比较 | `column <= value` |
| 模糊匹配 | `FILTER_LIKE` | 模糊查询 | `column LIKE 'pattern'` |
| 在列表中 | `FILTER_IN` | 多值匹配 | `column IN (value1, value2)` |

### 3. 过滤器类型

- **FilterAction**: 组合多个FilterOption
- **SearchAction**: 多字段模糊搜索
- **Ordering**: 多字段排序

## 使用示例

### 基础用法

```go
package main

import (
    "github.com/codelieche/cronjob/apiserver/pkg/utils/filters"
    "gorm.io/gorm"
)

func main() {
    // 1. 定义过滤选项
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
            QueryKey: "created_after",
            Column:   "created_at",
            Op:       filters.FILTER_GTE,
        },
        {
            QueryKey: "name__contains",
            Column:   "name",
            Op:       filters.FILTER_CONTAINS,
        },
    }

    // 2. 定义搜索字段
    searchFields := []string{"name", "description", "command"}

    // 3. 定义排序字段
    orderingFields := []string{"created_at", "name", "status"}
    defaultOrdering := "-created_at"

    // 4. 在控制器中使用
    filterActions := controller.FilterAction(c, filterOptions, searchFields, orderingFields, defaultOrdering)
    
    // 5. 应用到查询
    var tasks []Task
    query := db.Model(&Task{})
    for _, action := range filterActions {
        query = action.Filter(query)
    }
    query.Find(&tasks)
}
```

### 在Gin控制器中的完整示例

```go
func (controller *TaskController) List(c *gin.Context) {
    // 1. 解析分页参数
    pagination := controller.ParsePagination(c)

    // 2. 定义过滤选项
    filterOptions := []*filters.FilterOption{
        {
            QueryKey: "id",
            Column:   "id",
            Op:       filters.FILTER_EQ,
        },
        {
            QueryKey: "project",
            Column:   "project",
            Op:       filters.FILTER_EQ,
        },
        {
            QueryKey: "status",
            Column:   "status",
            Op:       filters.FILTER_EQ,
        },
        {
            QueryKey: "name__contains",
            Column:   "name",
            Op:       filters.FILTER_CONTAINS,
        },
        {
            QueryKey: "start_time",
            Column:   "created_at",
            Op:       filters.FILTER_GTE,
        },
        {
            QueryKey: "end_time",
            Column:   "created_at",
            Op:       filters.FILTER_LTE,
        },
    }

    // 3. 定义搜索字段
    searchFields := []string{"name", "description", "command"}

    // 4. 定义排序字段
    orderingFields := []string{"created_at", "time_plan", "name", "status"}
    defaultOrdering := "-created_at"

    // 5. 获取过滤动作
    filterActions := controller.FilterAction(c, filterOptions, searchFields, orderingFields, defaultOrdering)

    // 6. 计算偏移量
    offset := (pagination.Page - 1) * pagination.PageSize

    // 7. 获取任务列表
    tasks, err := controller.service.List(c.Request.Context(), offset, pagination.PageSize, filterActions...)
    if err != nil {
        controller.HandleError(c, err, http.StatusBadRequest)
        return
    }

    // 8. 获取总数
    count, err := controller.service.Count(c.Request.Context(), filterActions...)
    if err != nil {
        controller.HandleError(c, err, http.StatusBadRequest)
        return
    }

    // 9. 构建分页结果
    result := &types.ResponseList{
        Page:     pagination.Page,
        PageSize: pagination.PageSize,
        Count:    count,
        Results:  tasks,
    }

    // 10. 返回结果
    controller.HandleOK(c, result)
}
```

## API 查询参数示例

### 过滤参数
```
GET /api/tasks?name=test&status=pending,completed&created_after=2023-01-01&name__contains=task
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

## 高级功能

### 表关联查询

```go
filterOptions := []*filters.FilterOption{
    {
        QueryKey: "user_name",
        Column:   "users.name",  // 使用 table.column 格式
        Op:       filters.FILTER_EQ,
    },
    {
        QueryKey: "category_name",
        Column:   "categories.name",
        Op:       filters.FILTER_CONTAINS,
    },
}
```

### 自定义表达式

```go
// 在 expression.go 中定义自定义表达式
type CustomExpression struct {
    Column string
    Value  interface{}
}

func (c CustomExpression) Build(builder clause.Builder) {
    builder.WriteQuoted(c.Column)
    builder.WriteString(" MATCH ")
    builder.AddVar(builder, c.Value)
}
```

### 空值处理

```go
filterOptions := []*filters.FilterOption{
    {
        QueryKey:  "optional_field",
        Column:    "optional_field",
        Op:        filters.FILTER_EQ,
        AllowNull: true,  // 允许空值
    },
}
```

## 性能优化

1. **批量处理**: 所有过滤器一次性应用到查询中
2. **智能JOIN**: 自动避免重复JOIN同一张表
3. **条件优化**: 空值自动跳过，减少无效查询
4. **索引友好**: 生成的SQL对数据库索引友好

## 扩展指南

### 添加新的操作符

1. 在 `filter.go` 中添加常量
2. 在 `ClauseExpressionMap` 中添加映射
3. 实现对应的GORM子句表达式

```go
const FILTER_REGEX = 10

var ClauseExpressionMap = map[int]NewClauseExpressionFunc{
    // ... 现有映射
    FILTER_REGEX: func(column string, value interface{}) clause.Expression {
        return &RegexExpression{Column: column, Value: value}
    },
}
```

### 添加新的过滤器类型

1. 实现 `Filter` 接口
2. 添加相应的构造函数
3. 在 `FilterAction` 中集成

```go
type CustomFilter struct {
    // 自定义字段
}

func (c *CustomFilter) Filter(db *gorm.DB) *gorm.DB {
    // 自定义过滤逻辑
    return db
}
```

## 注意事项

1. **SQL注入防护**: 所有参数都通过GORM的参数化查询处理
2. **字段白名单**: 排序字段需要预先定义，防止SQL注入
3. **性能考虑**: 大量数据时建议添加适当的数据库索引
4. **空值处理**: 默认忽略空值，需要时设置 `AllowNull: true`

## 许可证

MIT License
