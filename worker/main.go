// Package main 计划任务系统 Worker 主程序
// 
// 这是一个分布式计划任务系统的工作节点，负责执行具体的任务，主要功能包括：
// 1. 与API Server建立连接 - 通过HTTP API和WebSocket进行通信
// 2. 接收任务执行指令 - 从API Server接收需要执行的任务
// 3. 执行具体任务 - 根据任务类型执行相应的命令或脚本
// 4. 上报执行状态 - 实时向API Server报告任务执行状态和结果
// 5. 心跳保活 - 定期向API Server发送心跳，保持连接状态
//
// 系统架构：
// - API Server: 负责任务管理、调度和状态跟踪
// - Worker: 负责具体任务的执行（当前程序）
// - Redis: 提供分布式锁和缓存
// - MySQL/PostgreSQL: 持久化存储任务和配置数据
package main

import (
	"log"

	"github.com/codelieche/cronjob/worker/pkg/app"
)

// main 程序入口点
// 
// 启动Worker节点，包括：
// 1. 创建应用实例
// 2. 初始化应用（配置、连接等）
// 3. 启动应用服务
// 4. 运行主循环（保持程序运行）
func main() {
	// 创建应用实例
	application := app.NewApp()

	// 初始化应用
	// 包括：加载配置、建立数据库连接、初始化服务等
	if err := application.Initialize(); err != nil {
		log.Fatal("初始化应用失败:", err)
	}

	// 启动应用
	// 包括：启动WebSocket连接、开始心跳、启动任务处理等
	if err := application.Start(); err != nil {
		log.Fatal("启动应用失败:", err)
	}

	// 运行应用主循环
	// 保持程序运行，处理各种事件和任务
	application.Run()
}
