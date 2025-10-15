package runner

import (
	"bufio"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"github.com/codelieche/cronjob/worker/pkg/core"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/registry"
	dockerclient "github.com/docker/docker/client"
	"github.com/docker/go-connections/nat"
)

// ========== 运行时初始化 ==========

// initRuntime 初始化运行时客户端
func (r *ContainerRunner) initRuntime(ctx context.Context, logChan chan<- string) error {
	runtime := r.config.Runtime
	if runtime == "" {
		runtime = "docker"
	}

	r.sendLog(logChan, fmt.Sprintf("🎯 容器运行时: %s\n", runtime))

	switch runtime {
	case "docker":
		return r.initDocker(ctx, logChan)
	case "containerd":
		return r.initContainerd(ctx, logChan)
	default:
		return fmt.Errorf("不支持的运行时: %s", runtime)
	}
}

// initDocker 初始化 Docker 客户端
func (r *ContainerRunner) initDocker(ctx context.Context, logChan chan<- string) error {
	var opts []dockerclient.Opt

	// 确定连接方式
	if r.config.Connection == "tcp" || r.config.Host != "" {
		// TCP 连接（远程）
		r.sendLog(logChan, fmt.Sprintf("🔌 连接远程 Docker: %s\n", r.config.Host))

		host := r.config.Host
		if !strings.HasPrefix(host, "tcp://") {
			host = fmt.Sprintf("tcp://%s", host)
		}
		opts = append(opts, dockerclient.WithHost(host))

		// TLS 配置
		if r.config.TLS {
			if err := r.prepareTLSCerts(logChan); err != nil {
				return err
			}

			opts = append(opts, dockerclient.WithTLSClientConfig(
				filepath.Join(r.tlsCertPath, "ca.pem"),
				filepath.Join(r.tlsCertPath, "cert.pem"),
				filepath.Join(r.tlsCertPath, "key.pem"),
			))
		}
	} else {
		// Unix Socket 连接（本地）
		socket := r.config.Socket
		if socket == "" {
			socket = "/var/run/docker.sock"
		}
		r.sendLog(logChan, fmt.Sprintf("🔌 连接本地 Docker: %s\n", socket))
		opts = append(opts, dockerclient.WithHost(fmt.Sprintf("unix://%s", socket)))
	}

	opts = append(opts, dockerclient.WithAPIVersionNegotiation())

	// 创建客户端
	cli, err := dockerclient.NewClientWithOpts(opts...)
	if err != nil {
		return fmt.Errorf("创建 Docker 客户端失败: %w", err)
	}
	r.dockerCli = cli

	// 验证连接
	info, err := cli.Info(ctx)
	if err != nil {
		return fmt.Errorf("Docker 连接验证失败: %w", err)
	}

	r.sendLog(logChan, fmt.Sprintf("✅ Docker 已连接: %s (%s)\n",
		info.Name, info.ServerVersion))

	return nil
}

// ========== Docker 操作执行 ==========

// executeWithDocker 使用 Docker 执行操作
func (r *ContainerRunner) executeWithDocker(ctx context.Context, logChan chan<- string) (*core.Result, error) {
	switch r.config.Action {
	case "build":
		return r.dockerBuild(ctx, logChan)
	case "run":
		return r.dockerRun(ctx, logChan)
	case "stop":
		return r.dockerStop(ctx, logChan)
	case "start":
		return r.dockerStart(ctx, logChan)
	case "restart":
		return r.dockerRestart(ctx, logChan)
	case "remove":
		return r.dockerRemove(ctx, logChan)
	case "push":
		return r.dockerPush(ctx, logChan)
	case "pull":
		return r.dockerPull(ctx, logChan)
	case "logs":
		return r.dockerLogs(ctx, logChan)
	case "exec":
		return r.dockerExec(ctx, logChan)
	case "inspect":
		return r.dockerInspect(ctx, logChan)
	case "prune":
		return r.dockerPrune(ctx, logChan)
	case "stats":
		return r.dockerStats(ctx, logChan)
	default:
		return nil, fmt.Errorf("不支持的操作: %s", r.config.Action)
	}
}

// ========== Docker 镜像操作 ==========

// dockerBuild 构建镜像
func (r *ContainerRunner) dockerBuild(ctx context.Context, logChan chan<- string) (*core.Result, error) {
	r.sendLog(logChan, "🏗️ 开始构建镜像...\n")

	// 1. 准备构建上下文
	buildContext, err := r.prepareBuildContext()
	if err != nil {
		return nil, fmt.Errorf("准备构建上下文失败: %w", err)
	}
	defer buildContext.Close()

	// 2. 配置构建选项
	buildArgs := make(map[string]*string)
	for k, v := range r.config.BuildArgs {
		val := v
		buildArgs[k] = &val
	}

	buildOptions := types.ImageBuildOptions{
		Dockerfile:  r.config.Dockerfile,
		Tags:        r.config.Tags,
		BuildArgs:   buildArgs,
		NoCache:     r.config.NoCache,
		PullParent:  r.config.Pull,
		Remove:      true,
		ForceRemove: true,
	}

	// 3. 执行构建
	resp, err := r.dockerCli.ImageBuild(ctx, buildContext, buildOptions)
	if err != nil {
		return nil, fmt.Errorf("构建失败: %w", err)
	}
	defer resp.Body.Close()

	// 4. 实时输出构建日志
	var imageID string
	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := scanner.Text()

		// 解析 JSON 格式日志
		var msg map[string]interface{}
		if err := json.Unmarshal([]byte(line), &msg); err == nil {
			if stream, ok := msg["stream"].(string); ok {
				r.sendLog(logChan, stream)
			}
			if errMsg, ok := msg["error"].(string); ok {
				return nil, fmt.Errorf("构建错误: %s", errMsg)
			}
			if aux, ok := msg["aux"].(map[string]interface{}); ok {
				if id, ok := aux["ID"].(string); ok {
					imageID = id
				}
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("读取构建日志失败: %w", err)
	}

	r.sendLog(logChan, fmt.Sprintf("✅ 构建完成: %s\n", imageID))

	// 5. 构建输出
	output := map[string]interface{}{
		"action":   "build",
		"image_id": imageID,
		"tags":     r.config.Tags,
	}

	return r.buildSuccessResult(output), nil
}

// dockerPull 拉取镜像
func (r *ContainerRunner) dockerPull(ctx context.Context, logChan chan<- string) (*core.Result, error) {
	r.sendLog(logChan, fmt.Sprintf("📥 拉取镜像: %s\n", r.config.Image))

	// 准备认证信息
	authConfig := registry.AuthConfig{}
	if r.config.RegistryCred != "" {
		auth, err := r.prepareRegistryAuth()
		if err != nil {
			return nil, err
		}
		authConfig = auth
	}

	authJSON, _ := json.Marshal(authConfig)
	encodedAuth := base64.URLEncoding.EncodeToString(authJSON)

	// 执行拉取
	resp, err := r.dockerCli.ImagePull(ctx, r.config.Image, image.PullOptions{
		RegistryAuth: encodedAuth,
	})
	if err != nil {
		return nil, fmt.Errorf("拉取镜像失败: %w", err)
	}
	defer resp.Close()

	// 输出拉取日志
	scanner := bufio.NewScanner(resp)
	for scanner.Scan() {
		line := scanner.Text()
		var msg map[string]interface{}
		if err := json.Unmarshal([]byte(line), &msg); err == nil {
			if status, ok := msg["status"].(string); ok {
				if progress, ok := msg["progress"].(string); ok {
					r.sendLog(logChan, fmt.Sprintf("%s %s\n", status, progress))
				} else {
					r.sendLog(logChan, fmt.Sprintf("%s\n", status))
				}
			}
		}
	}

	r.sendLog(logChan, "✅ 镜像拉取完成\n")

	output := map[string]interface{}{
		"action": "pull",
		"image":  r.config.Image,
	}

	return r.buildSuccessResult(output), nil
}

// dockerPush 推送镜像
func (r *ContainerRunner) dockerPush(ctx context.Context, logChan chan<- string) (*core.Result, error) {
	imageName := r.config.Image

	// 如果指定了 registry，重新打标签
	if r.config.Registry != "" {
		imageName = fmt.Sprintf("%s/%s", r.config.Registry, r.config.Image)
		r.sendLog(logChan, fmt.Sprintf("🏷️ 打标签: %s -> %s\n", r.config.Image, imageName))

		if err := r.dockerCli.ImageTag(ctx, r.config.Image, imageName); err != nil {
			return nil, fmt.Errorf("打标签失败: %w", err)
		}
	}

	r.sendLog(logChan, fmt.Sprintf("📤 推送镜像: %s\n", imageName))

	// 准备认证信息
	authConfig := registry.AuthConfig{}
	if r.config.RegistryCred != "" {
		auth, err := r.prepareRegistryAuth()
		if err != nil {
			return nil, err
		}
		authConfig = auth
	}

	authJSON, _ := json.Marshal(authConfig)
	encodedAuth := base64.URLEncoding.EncodeToString(authJSON)

	// 执行推送
	resp, err := r.dockerCli.ImagePush(ctx, imageName, image.PushOptions{
		RegistryAuth: encodedAuth,
	})
	if err != nil {
		return nil, fmt.Errorf("推送镜像失败: %w", err)
	}
	defer resp.Close()

	// 输出推送日志
	scanner := bufio.NewScanner(resp)
	for scanner.Scan() {
		line := scanner.Text()
		var msg map[string]interface{}
		if err := json.Unmarshal([]byte(line), &msg); err == nil {
			if status, ok := msg["status"].(string); ok {
				if progress, ok := msg["progress"].(string); ok {
					r.sendLog(logChan, fmt.Sprintf("%s %s\n", status, progress))
				} else {
					r.sendLog(logChan, fmt.Sprintf("%s\n", status))
				}
			}
			if errMsg, ok := msg["error"].(string); ok {
				return nil, fmt.Errorf("推送错误: %s", errMsg)
			}
		}
	}

	r.sendLog(logChan, "✅ 镜像推送完成\n")

	output := map[string]interface{}{
		"action": "push",
		"image":  imageName,
	}

	return r.buildSuccessResult(output), nil
}

// ========== Docker 容器操作 ==========

// dockerRun 运行容器
func (r *ContainerRunner) dockerRun(ctx context.Context, logChan chan<- string) (*core.Result, error) {
	r.sendLog(logChan, "🚀 启动容器...\n")

	// 1. 准备容器配置
	containerConfig := &container.Config{
		Image: r.config.Image,
		Env:   r.config.Env,
		Cmd:   r.config.Command,
	}

	if len(r.config.Entrypoint) > 0 {
		containerConfig.Entrypoint = r.config.Entrypoint
	}

	// 2. 准备主机配置
	hostConfig := &container.HostConfig{
		AutoRemove: r.config.Remove,
	}

	// 重启策略
	if r.config.Restart != "" {
		hostConfig.RestartPolicy = container.RestartPolicy{
			Name: container.RestartPolicyMode(r.config.Restart),
		}
	}

	// 端口映射
	if len(r.config.Ports) > 0 {
		portBindings, exposedPorts, err := r.parsePortBindings()
		if err != nil {
			return nil, err
		}
		hostConfig.PortBindings = portBindings
		containerConfig.ExposedPorts = exposedPorts
	}

	// 卷挂载
	if len(r.config.Volumes) > 0 {
		hostConfig.Binds = r.config.Volumes
	}

	// 网络
	if r.config.Network != "" {
		hostConfig.NetworkMode = container.NetworkMode(r.config.Network)
	}

	// 3. 创建容器
	resp, err := r.dockerCli.ContainerCreate(
		ctx,
		containerConfig,
		hostConfig,
		nil,
		nil,
		r.config.Name,
	)
	if err != nil {
		return nil, fmt.Errorf("创建容器失败: %w", err)
	}

	r.sendLog(logChan, fmt.Sprintf("📦 容器已创建: %s\n", resp.ID[:12]))

	// 4. 启动容器
	if err := r.dockerCli.ContainerStart(ctx, resp.ID, container.StartOptions{}); err != nil {
		return nil, fmt.Errorf("启动容器失败: %w", err)
	}

	r.sendLog(logChan, "✅ 容器已启动\n")

	// 5. 获取容器信息
	inspect, err := r.dockerCli.ContainerInspect(ctx, resp.ID)
	if err != nil {
		return nil, err
	}

	// 6. 构建输出
	output := map[string]interface{}{
		"action":       "run",
		"container_id": resp.ID,
		"name":         r.config.Name,
		"status":       inspect.State.Status,
	}

	return r.buildSuccessResult(output), nil
}

// dockerStop 停止容器
func (r *ContainerRunner) dockerStop(ctx context.Context, logChan chan<- string) (*core.Result, error) {
	r.sendLog(logChan, fmt.Sprintf("🛑 停止容器: %s\n", r.config.Container))

	timeout := r.config.Timeout
	var timeoutPtr *int
	if timeout > 0 {
		timeoutPtr = &timeout
	}

	if err := r.dockerCli.ContainerStop(ctx, r.config.Container, container.StopOptions{
		Timeout: timeoutPtr,
	}); err != nil {
		return nil, fmt.Errorf("停止容器失败: %w", err)
	}

	r.sendLog(logChan, "✅ 容器已停止\n")

	output := map[string]interface{}{
		"action":    "stop",
		"container": r.config.Container,
	}

	return r.buildSuccessResult(output), nil
}

// dockerStart 启动容器
func (r *ContainerRunner) dockerStart(ctx context.Context, logChan chan<- string) (*core.Result, error) {
	r.sendLog(logChan, fmt.Sprintf("▶️ 启动容器: %s\n", r.config.Container))

	if err := r.dockerCli.ContainerStart(ctx, r.config.Container, container.StartOptions{}); err != nil {
		return nil, fmt.Errorf("启动容器失败: %w", err)
	}

	r.sendLog(logChan, "✅ 容器已启动\n")

	output := map[string]interface{}{
		"action":    "start",
		"container": r.config.Container,
	}

	return r.buildSuccessResult(output), nil
}

// dockerRestart 重启容器
func (r *ContainerRunner) dockerRestart(ctx context.Context, logChan chan<- string) (*core.Result, error) {
	r.sendLog(logChan, fmt.Sprintf("🔄 重启容器: %s\n", r.config.Container))

	timeout := r.config.Timeout
	var timeoutPtr *int
	if timeout > 0 {
		timeoutPtr = &timeout
	}

	if err := r.dockerCli.ContainerRestart(ctx, r.config.Container, container.StopOptions{
		Timeout: timeoutPtr,
	}); err != nil {
		return nil, fmt.Errorf("重启容器失败: %w", err)
	}

	r.sendLog(logChan, "✅ 容器已重启\n")

	output := map[string]interface{}{
		"action":    "restart",
		"container": r.config.Container,
	}

	return r.buildSuccessResult(output), nil
}

// dockerRemove 删除容器或镜像
func (r *ContainerRunner) dockerRemove(ctx context.Context, logChan chan<- string) (*core.Result, error) {
	if r.config.Container != "" {
		// 删除容器
		r.sendLog(logChan, fmt.Sprintf("🗑️ 删除容器: %s\n", r.config.Container))

		if err := r.dockerCli.ContainerRemove(ctx, r.config.Container, container.RemoveOptions{
			Force:         r.config.Force,
			RemoveVolumes: r.config.RemoveVolumes,
		}); err != nil {
			return nil, fmt.Errorf("删除容器失败: %w", err)
		}

		r.sendLog(logChan, "✅ 容器已删除\n")

		output := map[string]interface{}{
			"action":    "remove",
			"type":      "container",
			"container": r.config.Container,
		}

		return r.buildSuccessResult(output), nil
	}

	if r.config.Image != "" {
		// 删除镜像
		r.sendLog(logChan, fmt.Sprintf("🗑️ 删除镜像: %s\n", r.config.Image))

		_, err := r.dockerCli.ImageRemove(ctx, r.config.Image, image.RemoveOptions{
			Force: r.config.Force,
		})
		if err != nil {
			return nil, fmt.Errorf("删除镜像失败: %w", err)
		}

		r.sendLog(logChan, "✅ 镜像已删除\n")

		output := map[string]interface{}{
			"action": "remove",
			"type":   "image",
			"image":  r.config.Image,
		}

		return r.buildSuccessResult(output), nil
	}

	return nil, fmt.Errorf("需要指定 container 或 image")
}

// dockerLogs 查看容器日志
func (r *ContainerRunner) dockerLogs(ctx context.Context, logChan chan<- string) (*core.Result, error) {
	r.sendLog(logChan, fmt.Sprintf("📋 查看容器日志: %s\n", r.config.Container))

	options := container.LogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Timestamps: r.config.Timestamps,
		Follow:     r.config.Follow,
	}

	if r.config.Tail > 0 {
		tail := fmt.Sprintf("%d", r.config.Tail)
		options.Tail = tail
	}

	resp, err := r.dockerCli.ContainerLogs(ctx, r.config.Container, options)
	if err != nil {
		return nil, fmt.Errorf("获取日志失败: %w", err)
	}
	defer resp.Close()

	// 读取日志
	var logs strings.Builder
	_, err = io.Copy(&logs, resp)
	if err != nil {
		return nil, fmt.Errorf("读取日志失败: %w", err)
	}

	logContent := logs.String()
	r.sendLog(logChan, logContent)
	r.sendLog(logChan, "\n✅ 日志读取完成\n")

	output := map[string]interface{}{
		"action":    "logs",
		"container": r.config.Container,
		"logs":      logContent,
	}

	return r.buildSuccessResult(output), nil
}

// dockerExec 在容器中执行命令
func (r *ContainerRunner) dockerExec(ctx context.Context, logChan chan<- string) (*core.Result, error) {
	r.sendLog(logChan, fmt.Sprintf("⚡ 执行命令: %s\n", strings.Join(r.config.ExecCommand, " ")))

	execConfig := container.ExecOptions{
		AttachStdout: true,
		AttachStderr: true,
		Cmd:          r.config.ExecCommand,
		Tty:          r.config.TTY,
	}

	// 创建 exec 实例
	execID, err := r.dockerCli.ContainerExecCreate(ctx, r.config.Container, execConfig)
	if err != nil {
		return nil, fmt.Errorf("创建 exec 失败: %w", err)
	}

	// 执行命令
	resp, err := r.dockerCli.ContainerExecAttach(ctx, execID.ID, container.ExecAttachOptions{})
	if err != nil {
		return nil, fmt.Errorf("执行命令失败: %w", err)
	}
	defer resp.Close()

	// 读取输出
	var output strings.Builder
	_, err = io.Copy(&output, resp.Reader)
	if err != nil {
		return nil, fmt.Errorf("读取输出失败: %w", err)
	}

	outputContent := output.String()
	r.sendLog(logChan, outputContent)
	r.sendLog(logChan, "\n✅ 命令执行完成\n")

	result := map[string]interface{}{
		"action":    "exec",
		"container": r.config.Container,
		"command":   r.config.ExecCommand,
		"output":    outputContent,
	}

	return r.buildSuccessResult(result), nil
}

// dockerInspect 查看容器详情
func (r *ContainerRunner) dockerInspect(ctx context.Context, logChan chan<- string) (*core.Result, error) {
	r.sendLog(logChan, fmt.Sprintf("🔍 查看容器详情: %s\n", r.config.Container))

	inspect, err := r.dockerCli.ContainerInspect(ctx, r.config.Container)
	if err != nil {
		return nil, fmt.Errorf("查看详情失败: %w", err)
	}

	// 构建输出
	output := map[string]interface{}{
		"action":       "inspect",
		"container":    r.config.Container,
		"container_id": inspect.ID,
		"name":         inspect.Name,
		"status":       inspect.State.Status,
		"image":        inspect.Config.Image,
		"created":      inspect.Created,
	}

	// 健康检查
	if r.config.CheckHealth && inspect.State.Health != nil {
		output["health"] = inspect.State.Health.Status
	}

	r.sendLog(logChan, fmt.Sprintf("✅ 状态: %s\n", inspect.State.Status))

	return r.buildSuccessResult(output), nil
}

// dockerPrune 清理资源
func (r *ContainerRunner) dockerPrune(ctx context.Context, logChan chan<- string) (*core.Result, error) {
	r.sendLog(logChan, fmt.Sprintf("🧹 清理 %s 资源...\n", r.config.Type))

	// 构建过滤器
	pruneFilters := filters.NewArgs()
	for k, v := range r.config.Filters {
		pruneFilters.Add(k, v)
	}

	var deletedCount int
	var spaceReclaimed uint64

	switch r.config.Type {
	case "image":
		report, err := r.dockerCli.ImagesPrune(ctx, pruneFilters)
		if err != nil {
			return nil, fmt.Errorf("清理镜像失败: %w", err)
		}
		deletedCount = len(report.ImagesDeleted)
		spaceReclaimed = report.SpaceReclaimed

	case "container":
		report, err := r.dockerCli.ContainersPrune(ctx, pruneFilters)
		if err != nil {
			return nil, fmt.Errorf("清理容器失败: %w", err)
		}
		deletedCount = len(report.ContainersDeleted)
		spaceReclaimed = report.SpaceReclaimed

	case "volume":
		report, err := r.dockerCli.VolumesPrune(ctx, pruneFilters)
		if err != nil {
			return nil, fmt.Errorf("清理卷失败: %w", err)
		}
		deletedCount = len(report.VolumesDeleted)
		spaceReclaimed = report.SpaceReclaimed

	case "network":
		report, err := r.dockerCli.NetworksPrune(ctx, pruneFilters)
		if err != nil {
			return nil, fmt.Errorf("清理网络失败: %w", err)
		}
		deletedCount = len(report.NetworksDeleted)

	default:
		return nil, fmt.Errorf("不支持的清理类型: %s", r.config.Type)
	}

	r.sendLog(logChan, fmt.Sprintf("✅ 清理完成: 删除 %d 个，释放 %d MB\n",
		deletedCount, spaceReclaimed/(1024*1024)))

	output := map[string]interface{}{
		"action":          "prune",
		"type":            r.config.Type,
		"deleted_count":   deletedCount,
		"space_reclaimed": spaceReclaimed,
	}

	return r.buildSuccessResult(output), nil
}

// dockerStats 查看资源统计
func (r *ContainerRunner) dockerStats(ctx context.Context, logChan chan<- string) (*core.Result, error) {
	r.sendLog(logChan, "📊 查看资源统计...\n")

	// 获取容器统计信息
	containerID := r.config.Container
	if containerID == "" {
		containerID = "" // 获取所有容器
	}

	resp, err := r.dockerCli.ContainerStats(ctx, containerID, false)
	if err != nil {
		return nil, fmt.Errorf("获取统计信息失败: %w", err)
	}
	defer resp.Body.Close()

	// 读取统计信息
	var stats types.StatsJSON
	if err := json.NewDecoder(resp.Body).Decode(&stats); err != nil {
		return nil, fmt.Errorf("解析统计信息失败: %w", err)
	}

	// 计算 CPU 和内存使用率
	cpuPercent := calculateCPUPercent(&stats)
	memoryPercent := calculateMemoryPercent(&stats)

	r.sendLog(logChan, fmt.Sprintf("CPU: %.2f%%, Memory: %.2f%%\n", cpuPercent, memoryPercent))
	r.sendLog(logChan, "✅ 统计信息获取完成\n")

	output := map[string]interface{}{
		"action":         "stats",
		"container":      containerID,
		"cpu_percent":    cpuPercent,
		"memory_percent": memoryPercent,
		"memory_usage":   stats.MemoryStats.Usage,
		"memory_limit":   stats.MemoryStats.Limit,
	}

	return r.buildSuccessResult(output), nil
}

// ========== 辅助函数 ==========

// prepareRegistryAuth 准备 Registry 认证信息
func (r *ContainerRunner) prepareRegistryAuth() (registry.AuthConfig, error) {
	cred, err := r.Apiserver.GetCredential(r.config.RegistryCred) // 🔥 直接访问公共字段
	if err != nil {
		return registry.AuthConfig{}, fmt.Errorf("获取 Registry 凭证失败: %w", err)
	}

	if cred.Category != "username_password" {
		return registry.AuthConfig{}, fmt.Errorf("Registry 凭证类型错误，需要 username_password 类型")
	}

	username, _ := cred.GetString("username")
	password, _ := cred.GetString("password")

	return registry.AuthConfig{
		Username: username,
		Password: password,
	}, nil
}

// parsePortBindings 解析端口映射
func (r *ContainerRunner) parsePortBindings() (nat.PortMap, nat.PortSet, error) {
	portBindings := nat.PortMap{}
	exposedPorts := nat.PortSet{}

	for _, portSpec := range r.config.Ports {
		// 格式: "8080:80" 或 "8080:80/tcp"
		parts := strings.Split(portSpec, ":")
		if len(parts) != 2 {
			return nil, nil, fmt.Errorf("端口映射格式错误: %s", portSpec)
		}

		hostPort := parts[0]
		containerPort := parts[1]

		// 解析容器端口
		port, err := nat.NewPort("tcp", strings.Split(containerPort, "/")[0])
		if err != nil {
			return nil, nil, fmt.Errorf("解析容器端口失败: %w", err)
		}

		exposedPorts[port] = struct{}{}
		portBindings[port] = []nat.PortBinding{
			{
				HostIP:   "0.0.0.0",
				HostPort: hostPort,
			},
		}
	}

	return portBindings, exposedPorts, nil
}

// calculateCPUPercent 计算 CPU 使用率
func calculateCPUPercent(stats *types.StatsJSON) float64 {
	cpuDelta := float64(stats.CPUStats.CPUUsage.TotalUsage - stats.PreCPUStats.CPUUsage.TotalUsage)
	systemDelta := float64(stats.CPUStats.SystemUsage - stats.PreCPUStats.SystemUsage)

	if systemDelta > 0.0 && cpuDelta > 0.0 {
		return (cpuDelta / systemDelta) * float64(len(stats.CPUStats.CPUUsage.PercpuUsage)) * 100.0
	}
	return 0.0
}

// calculateMemoryPercent 计算内存使用率
func calculateMemoryPercent(stats *types.StatsJSON) float64 {
	if stats.MemoryStats.Limit > 0 {
		return float64(stats.MemoryStats.Usage) / float64(stats.MemoryStats.Limit) * 100.0
	}
	return 0.0
}
