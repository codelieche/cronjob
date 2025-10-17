# 监控中间件使用说明

本文档说明如何使用监控中间件来收集系统和业务指标。

## 🎯 中间件功能

### 1. PrometheusMiddleware
**基础HTTP监控中间件**，自动收集：
- HTTP请求总数（按方法、端点、状态码分类）
- HTTP请求响应时间分布
- 当前正在处理的请求数

### 2. MetricsCollectionMiddleware  
**详细业务指标中间件**，提供：
- 业务端点特定指标收集
- 慢请求检测和日志记录
- 错误请求详细分析
- 项目和分类维度的指标统计

### 3. DatabaseMetricsMiddleware
**数据库操作监控中间件**，监控：
- 数据库操作类型和表名推断
- 操作响应时间统计
- 数据库错误分类统计

## 🚀 使用方法

### 在router.go中配置中间件

```go
// 为API路由组添加监控中间件
apis.Use(middleware.PrometheusMiddleware())           // 基础HTTP监控
apis.Use(middleware.MetricsCollectionMiddleware())    // 业务指标收集
apis.Use(middleware.DatabaseMetricsMiddleware())      // 数据库监控（可选）
```

### 完整配置示例

```go
func initRouter(app *gin.Engine) {
    // 创建API v1路由组
    apis := app.Group("/api/v1")
    
    // 添加Session中间件
    apis.Use(sessions.Sessions(config.Web.SessionIDName, sstore))
    
    // 添加监控中间件（按顺序添加）
    apis.Use(middleware.PrometheusMiddleware())        // 1. 基础监控
    apis.Use(middleware.MetricsCollectionMiddleware()) // 2. 业务监控  
    apis.Use(middleware.DatabaseMetricsMiddleware())   // 3. 数据库监控
    
    // 注册业务路由...
    apis.POST("/cronjob/", cronJobController.Create)
    apis.GET("/task/", taskController.List)
    // ...
}
```

## 📊 收集的指标详情

### HTTP基础指标
- `cronjob_http_requests_total`: HTTP请求总数
- `cronjob_http_request_duration_seconds`: HTTP请求响应时间
- `cronjob_http_requests_in_flight`: 当前处理中的请求数

### 业务指标
- `cronjob_cronjob_executions_total`: CronJob执行次数
- `cronjob_task_executions_total`: 任务执行次数  
- `cronjob_task_errors_total`: 任务错误统计
- `cronjob_worker_connections_total`: Worker连接事件
- `cronjob_lock_acquisitions_total`: 分布式锁操作

### 数据库指标
- `cronjob_db_query_duration_seconds`: 数据库查询时长
- `cronjob_db_errors_total`: 数据库错误统计

## 🔧 自定义配置

### 1. 慢请求阈值调整

```go
// 在MetricsCollectionMiddleware中调整慢请求阈值
if duration > 500*time.Millisecond { // 从1秒改为500毫秒
    logger.Warn("检测到慢请求", ...)
}
```

### 2. 添加自定义业务指标

```go
// 在collectBusinessMetrics函数中添加新的端点处理
case strings.HasPrefix(endpoint, "/api/v1/custom"):
    // 自定义业务逻辑的指标收集
    monitoring.GlobalMetrics.CustomMetric.WithLabelValues(...).Inc()
```

### 3. 错误分类自定义

```go
// 在collectErrorMetrics函数中添加特定错误处理
case strings.HasPrefix(endpoint, "/api/v1/custom"):
    monitoring.GlobalMetrics.CustomErrors.WithLabelValues(
        errorType,
        getCustomContextInfo(c),
    ).Inc()
```

## 📈 监控数据示例

### Prometheus查询示例

```promql
# HTTP请求QPS
sum(rate(cronjob_http_requests_total[5m])) by (method, endpoint)

# 任务成功率
sum(rate(cronjob_task_executions_total{status="success"}[5m])) / 
sum(rate(cronjob_task_executions_total[5m])) * 100

# 慢请求统计
histogram_quantile(0.95, sum(rate(cronjob_http_request_duration_seconds_bucket[5m])) by (le))

# 错误率统计
sum(rate(cronjob_http_requests_total{status_code=~"5.."}[5m])) /
sum(rate(cronjob_http_requests_total[5m])) * 100
```

### 日志输出示例

```json
{
  "level": "warn",
  "time": "2024-01-01T10:00:00Z",
  "msg": "检测到慢请求",
  "method": "POST",
  "endpoint": "/api/v1/cronjob/",
  "duration": "2.5s",
  "status_code": 200,
  "user_agent": "curl/7.68.0",
  "remote_addr": "192.168.1.100"
}
```

## ⚠️ 注意事项

### 1. 性能影响
- 监控中间件会增加少量的响应时间（通常<1ms）
- 高QPS场景下建议监控指标的采样率
- 避免在指标标签中使用高基数值（如用户ID、任务ID等）

### 2. 内存使用
- Prometheus指标会占用内存，标签组合越多占用越大
- 建议定期重启应用释放指标内存
- 监控指标数量，避免指标爆炸

### 3. 标签设计
```go
// ✅ 好的标签设计（低基数）
monitoring.GlobalMetrics.TaskExecutions.WithLabelValues(
    category,    // 有限的几个分类
    status,      // 有限的几个状态
).Inc()

// ❌ 不好的标签设计（高基数）
monitoring.GlobalMetrics.TaskExecutions.WithLabelValues(
    taskID,      // 无限增长的任务ID
    timestamp,   // 时间戳会产生大量标签
).Inc()
```

## 🐛 故障排查

### 1. 指标未收集
- 检查中间件是否正确注册
- 确认路由路径匹配逻辑
- 查看应用日志中的错误信息

### 2. 指标值异常
- 检查业务逻辑是否正确触发指标收集
- 确认标签值是否符合预期
- 验证指标计算逻辑

### 3. 性能问题
- 监控指标收集的耗时
- 检查是否有指标标签基数过高
- 考虑使用采样或限流机制

## 🔗 相关链接

- [Prometheus指标类型](https://prometheus.io/docs/concepts/metric_types/)
- [Gin中间件开发指南](https://gin-gonic.com/docs/examples/custom-middleware/)
- [Go监控最佳实践](https://prometheus.io/docs/guides/go-application/)
