# CronJob 分布式定时任务系统

基于Go语言开发的分布式定时任务系统，支持高并发任务调度和执行。

## 架构组件

- **apiserver/**: API服务器，负责任务管理、调度和WebSocket通信
- **worker/**: 工作节点，负责接收和执行具体任务
- **deploy/**: 部署配置文件，包含Docker和监控配置

## 快速启动

### 1. 启动API Server
```bash
cd apiserver
go build && ./apiserver
```

### 2. 启动Worker节点
```bash
cd worker  
go build && ./worker
```

### 3. 监控面板
```bash
cd deploy
docker-compose -f docker-compose.monitoring.yml up -d
```

## 访问地址

- **API接口**: http://localhost:8080
- **健康检查**: http://localhost:8080/api/v1/health/
- **监控指标**: http://localhost:8080/metrics
- **Grafana面板**: http://localhost:3000 (admin/***** */)

## 环境要求

- Go 1.21+
- MySQL/PostgreSQL
- Redis