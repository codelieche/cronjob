# Worker - 分布式定时任务系统工作节点

## 概述

Worker是分布式定时任务系统的工作节点，负责接收和执行来自API Server的任务。Worker采用插件化的Runner架构，支持多种类型的任务执行器。

## 核心特性

- **插件化Runner**: 支持多种任务执行器，易于扩展
- **实时通信**: 通过WebSocket与API Server实时通信
- **任务管理**: 支持任务启动、停止、超时控制
- **状态上报**: 实时上报任务执行状态和结果
- **心跳保活**: 自动维护与API Server的连接

## 架构设计

### 核心组件

- **WebSocket服务**: 与API Server通信
- **任务服务**: 处理任务事件
- **Runner系统**: 执行具体任务
- **配置管理**: 系统配置和参数

### Runner系统

Runner是任务执行的核心组件，每种任务类型对应一个特定的Runner：

- `CommandRunner`: 执行系统命令和脚本
- `ScriptRunner`: 执行各种脚本文件（计划中）
- `DockerRunner`: 在Docker容器中执行任务（计划中）
- `HttpRunner`: 执行HTTP请求（计划中）

## 快速开始

### 1. 安装依赖

```bash
go mod tidy
```

### 2. 配置Worker

编辑 `config_worker.json`:

```json
{
  "worker": {
    "name": "worker-01",
    "description": "Worker节点1"
  },
  "server": {
    "api_url": "http://localhost:8080",
    "auth_token": "your-auth-token"
  },
  "websocket": {
    "ping_interval": 20
  }
}
```

### 3. 启动Worker

```bash
go run main.go
```

## Runner使用指南

### CommandRunner

CommandRunner用于执行系统命令和脚本。

#### 基本用法

```go
package main

import (
    "context"
    "github.com/codelieche/cronjob/worker/pkg/core"
    "github.com/codelieche/cronjob/worker/pkg/runner"
)

func main() {
    // 创建CommandRunner
    cmdRunner := runner.NewCommandRunner()
    
    // 解析参数
    err := cmdRunner.ParseArgs("echo", `["Hello, World!"]`)
    if err != nil {
        log.Fatal(err)
    }
    
    // 执行命令
    ctx := context.Background()
    result, err := cmdRunner.Execute(ctx)
    if err != nil {
        log.Fatal(err)
    }
    
    // 处理结果
    fmt.Printf("状态: %s\n", result.Status)
    fmt.Printf("输出: %s\n", result.Output)
    
    // 清理资源
    cmdRunner.Cleanup()
}
```

#### 参数格式

CommandRunner接受JSON格式的参数数组：

```json
{
  "command": "/bin/ls",
  "args": "[\"-la\", \"/tmp\"]"
}
```

#### 超时控制

```go
// 设置超时时间为30秒
cmdRunner.SetTimeout(30 * time.Second)
```

#### 任务停止

```go
// 优雅停止
err := cmdRunner.Stop()

// 强制终止
err := cmdRunner.Kill()
```

### 从注册表创建Runner

```go
// 创建CommandRunner
runner, err := core.CreateRunner("command")
if err != nil {
    log.Fatal(err)
}

// 列出所有可用的Runner类型
runners := core.ListRunners()
for _, r := range runners {
    fmt.Printf("可用Runner: %s\n", r)
}
```

## 自定义Runner

### 1. 实现Runner接口

```go
type CustomRunner struct {
    // 自定义字段
}

func (r *CustomRunner) ParseArgs(command string, args string) error {
    // 实现参数解析
}

func (r *CustomRunner) Execute(ctx context.Context) (*core.Result, error) {
    // 实现任务执行
}

func (r *CustomRunner) Stop() error {
    // 实现任务停止
}

func (r *CustomRunner) Kill() error {
    // 实现任务强制终止
}

func (r *CustomRunner) GetStatus() core.Status {
    // 返回当前状态
}

func (r *CustomRunner) GetResult() *core.Result {
    // 返回执行结果
}

func (r *CustomRunner) SetTimeout(timeout time.Duration) {
    // 设置超时时间
}

func (r *CustomRunner) Cleanup() error {
    // 清理资源
}
```

### 2. 注册Runner

```go
func init() {
    core.RegisterRunner("custom", func() core.Runner {
        return &CustomRunner{}
    })
}
```

## 配置说明

### Worker配置

```json
{
  "worker": {
    "name": "worker-01",           // Worker名称
    "description": "Worker节点1"    // Worker描述
  },
  "server": {
    "api_url": "http://localhost:8080",  // API Server地址
    "auth_token": "your-auth-token"      // 认证令牌
  },
  "websocket": {
    "ping_interval": 20,           // 心跳间隔（秒）
    "message_separator": "\x00223399AABB2233CC"  // 消息分隔符
  }
}
```

### 环境变量

- `WORKER_NAME`: Worker名称
- `WORKER_DESCRIPTION`: Worker描述
- `API_SERVER_URL`: API Server地址
- `API_AUTH_TOKEN`: 认证令牌
- `WEBSOCKET_PING_INTERVAL`: 心跳间隔

## 开发指南

### 项目结构

```
worker/
├── main.go                 # 程序入口
├── config_worker.json     # 配置文件
├── pkg/
│   ├── app/               # 应用层
│   ├── config/            # 配置管理
│   ├── core/              # 核心接口和模型
│   ├── services/          # 业务服务
│   ├── runner/            # Runner实现
│   └── utils/             # 工具函数
├── examples/              # 使用示例
├── docs/                  # 文档
└── tests/                 # 测试文件
```

### 运行测试

```bash
# 运行所有测试
go test ./...

# 运行特定包的测试
go test ./pkg/runner/

# 运行测试并显示覆盖率
go test -cover ./...
```

### 构建

```bash
# 构建Worker
go build -o worker main.go

# 交叉编译
GOOS=linux GOARCH=amd64 go build -o worker-linux main.go
```

## 监控和日志

### 日志级别

- `DEBUG`: 调试信息
- `INFO`: 一般信息
- `WARN`: 警告信息
- `ERROR`: 错误信息
- `FATAL`: 致命错误

### 监控指标

- 任务执行数量
- 任务成功率
- 平均执行时间
- 错误率统计
- 资源使用情况

## 故障排除

### 常见问题

1. **WebSocket连接失败**
   - 检查API Server地址是否正确
   - 检查网络连接
   - 检查认证令牌

2. **任务执行失败**
   - 检查命令路径是否正确
   - 检查参数格式是否正确
   - 查看错误日志

3. **Runner注册失败**
   - 检查Runner是否实现了所有接口方法
   - 检查注册代码是否在init函数中

### 调试模式

```bash
# 启用调试日志
export LOG_LEVEL=debug
go run main.go
```

## 贡献指南

1. Fork项目
2. 创建特性分支
3. 提交更改
4. 推送到分支
5. 创建Pull Request

## 许可证

本项目采用MIT许可证，详见LICENSE文件。
