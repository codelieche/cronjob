package runner

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/codelieche/cronjob/worker/pkg/core"
	"github.com/codelieche/cronjob/worker/pkg/utils/logger"
	"github.com/containerd/containerd"
	dockerclient "github.com/docker/docker/client"
	"go.uber.org/zap"
)

// ContainerConfig 容器操作配置（运行时无关，参考 skaffold）
type ContainerConfig struct {
	Action string `json:"action"` // build/run/push/pull/stop/restart/remove/logs/exec/inspect/prune/stats

	// ========== 运行时配置 ==========
	Runtime    string `json:"runtime"`    // docker/containerd (默认 docker)
	Connection string `json:"connection"` // unix/tcp (默认 unix)

	// Unix Socket 连接（本地）
	Socket string `json:"socket"` // /var/run/docker.sock 或 /run/containerd/containerd.sock

	// TCP 连接（远程，仅 Docker）
	Host    string `json:"host"`     // docker-proxy:2376
	TLS     bool   `json:"tls"`      // 是否启用 TLS
	TLSCred string `json:"tls_cred"` // TLS 凭证 ID（key_value 类型）

	// containerd 特定
	Namespace    string `json:"namespace"`     // containerd namespace（默认 default）
	BuildkitAddr string `json:"buildkit_addr"` // buildkit 地址（镜像构建）

	// ========== build 字段 ==========
	Dockerfile string            `json:"dockerfile"` // Dockerfile 路径
	Context    string            `json:"context"`    // 构建上下文
	Tags       []string          `json:"tags"`       // 镜像标签
	BuildArgs  map[string]string `json:"build_args"` // 构建参数
	NoCache    bool              `json:"no_cache"`   // 不使用缓存
	Pull       bool              `json:"pull"`       // 拉取最新基础镜像

	// ========== run 字段 ==========
	Image      string   `json:"image"`      // 镜像名称
	Name       string   `json:"name"`       // 容器名称
	Ports      []string `json:"ports"`      // 端口映射 ["80:80", "443:443"]
	Volumes    []string `json:"volumes"`    // 卷挂载 ["/host:/container"]
	Env        []string `json:"env"`        // 环境变量 ["KEY=VALUE"]
	Network    string   `json:"network"`    // 网络
	Restart    string   `json:"restart"`    // 重启策略
	Detach     bool     `json:"detach"`     // 后台运行
	Remove     bool     `json:"remove"`     // 退出后删除
	Command    []string `json:"command"`    // 覆盖 CMD
	Entrypoint []string `json:"entrypoint"` // 覆盖 ENTRYPOINT

	// ========== push/pull 字段 ==========
	Registry     string `json:"registry"`      // 镜像仓库
	RegistryCred string `json:"registry_cred"` // Registry 凭证 ID（username_password）
	TagLatest    bool   `json:"tag_latest"`    // 同时推送 latest 标签

	// ========== stop/start/restart 字段 ==========
	Container string `json:"container"` // 容器名或 ID
	Timeout   int    `json:"timeout"`   // 停止超时（秒）

	// ========== remove 字段 ==========
	Force         bool `json:"force"`          // 强制删除
	RemoveVolumes bool `json:"remove_volumes"` // 删除关联卷

	// ========== prune 字段 ==========
	Type    string            `json:"type"`    // image/container/volume/network/all
	Filters map[string]string `json:"filters"` // dangling=true, until=24h

	// ========== logs 字段 ==========
	Lines      int  `json:"lines"`      // 显示行数
	Follow     bool `json:"follow"`     // 持续输出
	Timestamps bool `json:"timestamps"` // 显示时间戳
	Tail       int  `json:"tail"`       // 从末尾开始

	// ========== exec 字段 ==========
	ExecCommand []string `json:"exec_command"` // 要执行的命令
	Interactive bool     `json:"interactive"`  // 交互模式
	TTY         bool     `json:"tty"`          // 分配 TTY

	// ========== inspect 字段 ==========
	CheckHealth bool `json:"check_health"` // 检查健康状态
}

// ContainerRunner 容器执行器（运行时无关，参考 skaffold）
//
// 支持两种容器运行时：
// - Docker: 开发环境、容器化 Worker（支持 Unix Socket + TCP Remote）
// - containerd: 生产环境、Kubernetes 节点（仅 Unix Socket）
//
// 核心功能：
// - 镜像管理：build, pull, push, tag, remove
// - 容器管理：run, stop, start, restart, remove
// - 容器操作：logs, exec, inspect, stats
// - 系统维护：prune (image/container/volume/network)
//
// 连接模式：
// - Unix Socket: 本地高性能连接
// - TCP Remote: 容器化 Worker（类似 Jenkins Docker Plugin）
type ContainerRunner struct {
	BaseRunner // 🔥 嵌入基类

	config ContainerConfig // 容器操作配置

	// 运行时客户端（只初始化其中一个）
	dockerCli     *dockerclient.Client // Docker 客户端
	containerdCli *containerd.Client   // containerd 客户端
	// buildkitCli   *buildkit.Client     // Buildkit 客户端（containerd 构建，暂不实现）

	// TLS 证书临时目录（TCP 模式）
	tlsCertPath string
}

// NewContainerRunner 创建新的 ContainerRunner
func NewContainerRunner() *ContainerRunner {
	r := &ContainerRunner{}
	r.InitBase() // 🔥 初始化基类
	return r
}

// SetApiserver 继承自 BaseRunner

// ParseArgs 解析任务参数
func (r *ContainerRunner) ParseArgs(task *core.Task) error {
	r.Lock()
	defer r.Unlock()

	r.Task = task

	// 1. 解析 JSON 配置
	r.config = ContainerConfig{}
	if err := json.Unmarshal([]byte(task.Args), &r.config); err != nil {
		return fmt.Errorf("解析容器配置失败: %w", err)
	}

	// 2. 验证必需字段
	if r.config.Action == "" {
		return fmt.Errorf("action 字段必填")
	}

	// 3. 设置默认值
	if r.config.Runtime == "" {
		r.config.Runtime = "docker" // 默认使用 docker
	}
	if r.config.Connection == "" {
		r.config.Connection = "unix" // 默认使用 unix socket
	}
	if r.config.Namespace == "" {
		r.config.Namespace = "default" // containerd 默认 namespace
	}

	// 4. 验证运行时
	if r.config.Runtime != "docker" && r.config.Runtime != "containerd" {
		return fmt.Errorf("不支持的运行时: %s（仅支持 docker/containerd）", r.config.Runtime)
	}

	// 5. 验证连接方式
	if r.config.Connection == "tcp" && r.config.Runtime == "containerd" {
		return fmt.Errorf("containerd 不支持 TCP 连接，仅支持 Unix Socket")
	}

	// 6. 根据 action 验证必需字段
	switch r.config.Action {
	case "build":
		if r.config.Context == "" {
			return fmt.Errorf("build 操作需要指定 context")
		}
		if len(r.config.Tags) == 0 {
			return fmt.Errorf("build 操作需要至少一个 tag")
		}
	case "run":
		if r.config.Image == "" {
			return fmt.Errorf("run 操作需要指定 image")
		}
	case "push", "pull":
		if r.config.Image == "" {
			return fmt.Errorf("%s 操作需要指定 image", r.config.Action)
		}
	case "stop", "start", "restart":
		if r.config.Container == "" {
			return fmt.Errorf("%s 操作需要指定 container", r.config.Action)
		}
	case "logs", "exec", "inspect":
		if r.config.Container == "" {
			return fmt.Errorf("%s 操作需要指定 container", r.config.Action)
		}
	case "remove":
		if r.config.Container == "" && r.config.Image == "" {
			return fmt.Errorf("remove 操作需要指定 container 或 image")
		}
	case "prune":
		if r.config.Type == "" {
			return fmt.Errorf("prune 操作需要指定 type (image/container/volume/network/all)")
		}
	case "stats":
		// stats 可以不指定 container，默认显示所有容器
	default:
		return fmt.Errorf("不支持的操作: %s", r.config.Action)
	}

	return nil
}

// Execute 执行任务
func (r *ContainerRunner) Execute(ctx context.Context, logChan chan<- string) (*core.Result, error) {
	defer r.cleanup() // 清理临时文件

	r.Ctx, r.Cancel = context.WithCancel(ctx)
	r.Status = core.StatusRunning
	r.StartTime = time.Now()

	r.sendLog(logChan, "🚀 ContainerRunner 启动\n")
	r.sendLog(logChan, fmt.Sprintf("🎯 运行时: %s\n", r.config.Runtime))
	r.sendLog(logChan, fmt.Sprintf("🔌 连接: %s\n", r.config.Connection))
	r.sendLog(logChan, fmt.Sprintf("⚙️  操作: %s\n", r.config.Action))

	// 1. 初始化运行时客户端
	if err := r.initRuntime(ctx, logChan); err != nil {
		r.Result = r.buildErrorResult(err)
		return r.Result, err
	}
	defer r.closeRuntime()

	// 2. 根据运行时执行操作
	var result *core.Result
	var err error

	switch r.config.Runtime {
	case "docker":
		result, err = r.executeWithDocker(ctx, logChan)
	case "containerd":
		result, err = r.executeWithContainerd(ctx, logChan)
	default:
		err = fmt.Errorf("不支持的运行时: %s", r.config.Runtime)
		result = r.buildErrorResult(err)
	}

	if err != nil {
		r.Result = r.buildErrorResult(err)
		return r.Result, err
	}

	r.Result = result
	r.Status = core.StatusSuccess
	return r.Result, nil
}

// Stop 停止任务
func (r *ContainerRunner) Stop() error {
	r.Lock()
	defer r.Unlock()

	if r.Cancel != nil {
		r.Cancel()
	}
	r.Status = core.StatusStopped
	return nil
}

// Kill 强制终止任务
func (r *ContainerRunner) Kill() error {
	return r.Stop()
}

// GetStatus, GetResult 方法继承自 BaseRunner

// Cleanup 清理资源
func (r *ContainerRunner) Cleanup() error {
	r.cleanup()
	return nil
}

// ========== 内部辅助方法 ==========

// sendLog 发送日志到通道
func (r *ContainerRunner) sendLog(logChan chan<- string, message string) {
	if logChan != nil {
		select {
		case logChan <- message:
		case <-r.Ctx.Done():
		default:
		}
	}
	logger.Logger().Debug(strings.TrimSpace(message),
		zap.String("task_id", r.Task.ID.String()),
		zap.String("action", r.config.Action),
		zap.String("runtime", r.config.Runtime),
	)
}

// buildSuccessResult 构建成功结果
func (r *ContainerRunner) buildSuccessResult(output map[string]interface{}) *core.Result {
	outputJSON, _ := json.Marshal(output)

	return &core.Result{
		Status:     core.StatusSuccess,
		Output:     string(outputJSON),
		ExecuteLog: fmt.Sprintf("操作 %s 执行成功", r.config.Action),
		StartTime:  r.StartTime,
		EndTime:    time.Now(),
	}
}

// buildErrorResult 构建错误结果
func (r *ContainerRunner) buildErrorResult(err error) *core.Result {
	return &core.Result{
		Status:     core.StatusFailed,
		Output:     fmt.Sprintf(`{"error":"%s"}`, err.Error()),
		ExecuteLog: err.Error(),
		StartTime:  r.StartTime,
		EndTime:    time.Now(),
	}
}

// cleanup 清理临时资源
func (r *ContainerRunner) cleanup() {
	// 清理 TLS 证书临时目录
	if r.tlsCertPath != "" {
		os.RemoveAll(r.tlsCertPath)
		r.tlsCertPath = ""
	}

	// 关闭运行时客户端
	r.closeRuntime()

	// 取消上下文
	if r.Cancel != nil {
		r.Cancel()
	}
}

// closeRuntime 关闭运行时客户端
func (r *ContainerRunner) closeRuntime() {
	if r.dockerCli != nil {
		r.dockerCli.Close()
		r.dockerCli = nil
	}
	if r.containerdCli != nil {
		r.containerdCli.Close()
		r.containerdCli = nil
	}
}

// prepareTLSCerts 准备 TLS 证书文件（Docker TCP 模式）
func (r *ContainerRunner) prepareTLSCerts(logChan chan<- string) error {
	r.sendLog(logChan, "🔐 准备 TLS 证书...\n")

	// 1. 获取凭证
	cred, err := r.Apiserver.GetCredential(r.config.TLSCred)
	if err != nil {
		return fmt.Errorf("获取 TLS 凭证失败: %w", err)
	}

	if cred.Category != "key_value" {
		return fmt.Errorf("TLS 凭证类型错误，需要 key_value 类型，当前为: %s", cred.Category)
	}

	// 2. 创建临时目录
	tmpDir, err := os.MkdirTemp("", "docker-tls-*")
	if err != nil {
		return fmt.Errorf("创建临时目录失败: %w", err)
	}
	r.tlsCertPath = tmpDir

	// 3. 提取证书内容
	caCert, _ := cred.GetString("ca_cert")
	clientCert, _ := cred.GetString("client_cert")
	clientKey, _ := cred.GetString("client_key")

	// 4. 写入证书文件
	certs := map[string]string{
		"ca.pem":   caCert,
		"cert.pem": clientCert,
		"key.pem":  clientKey,
	}

	for filename, content := range certs {
		if content == "" {
			return fmt.Errorf("缺少证书: %s", filename)
		}

		path := filepath.Join(tmpDir, filename)
		if err := os.WriteFile(path, []byte(content), 0600); err != nil {
			return fmt.Errorf("写入证书文件 %s 失败: %w", filename, err)
		}
	}

	r.sendLog(logChan, fmt.Sprintf("✅ TLS 证书已准备: %s\n", tmpDir))
	return nil
}
