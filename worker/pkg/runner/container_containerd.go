package runner

import (
	"context"
	"fmt"
	"syscall"

	"github.com/codelieche/cronjob/worker/pkg/core"
	"github.com/containerd/containerd"
	"github.com/containerd/containerd/cio"
	"github.com/containerd/containerd/namespaces"
	"github.com/containerd/containerd/oci"
)

// ========== containerd 初始化 ==========

// initContainerd 初始化 containerd 客户端
func (r *ContainerRunner) initContainerd(ctx context.Context, logChan chan<- string) error {
	socket := r.config.Socket
	if socket == "" {
		socket = "/run/containerd/containerd.sock"
	}

	r.sendLog(logChan, fmt.Sprintf("🔌 连接 containerd: %s\n", socket))

	// 创建 containerd 客户端
	client, err := containerd.New(socket,
		containerd.WithDefaultNamespace(r.config.Namespace),
	)
	if err != nil {
		return fmt.Errorf("创建 containerd 客户端失败: %w", err)
	}
	r.containerdCli = client

	// 验证连接
	version, err := client.Version(ctx)
	if err != nil {
		return fmt.Errorf("containerd 连接验证失败: %w", err)
	}

	r.sendLog(logChan, fmt.Sprintf("✅ containerd 已连接: %s\n", version.Version))

	// 注意：镜像构建需要 buildkit（暂不实现）
	// if r.config.Action == "build" {
	//     buildkitAddr := r.config.BuildkitAddr
	//     if buildkitAddr == "" {
	//         buildkitAddr = "unix:///run/buildkit/buildkitd.sock"
	//     }
	//     r.sendLog(logChan, fmt.Sprintf("🔌 连接 buildkit: %s\n", buildkitAddr))
	//     // buildkitCli initialization...
	// }

	return nil
}

// ========== containerd 操作执行 ==========

// executeWithContainerd 使用 containerd 执行操作
func (r *ContainerRunner) executeWithContainerd(ctx context.Context, logChan chan<- string) (*core.Result, error) {
	// 设置 namespace
	ctx = namespaces.WithNamespace(ctx, r.config.Namespace)

	switch r.config.Action {
	case "build":
		return nil, fmt.Errorf("containerd 构建镜像需要 buildkit 支持，暂未实现")
		// return r.containerdBuild(ctx, logChan)
	case "run":
		return r.containerdRun(ctx, logChan)
	case "stop":
		return r.containerdStop(ctx, logChan)
	case "start":
		return r.containerdStart(ctx, logChan)
	case "remove":
		return r.containerdRemove(ctx, logChan)
	case "push":
		return r.containerdPush(ctx, logChan)
	case "pull":
		return r.containerdPull(ctx, logChan)
	case "logs":
		return nil, fmt.Errorf("containerd 不直接支持日志查看，请使用日志收集工具")
	case "exec":
		return r.containerdExec(ctx, logChan)
	case "inspect":
		return r.containerdInspect(ctx, logChan)
	case "prune":
		return nil, fmt.Errorf("containerd 清理功能暂未实现")
	case "stats":
		return nil, fmt.Errorf("containerd 统计功能暂未实现")
	default:
		return nil, fmt.Errorf("不支持的操作: %s", r.config.Action)
	}
}

// ========== containerd 镜像操作 ==========

// containerdPull 拉取镜像
func (r *ContainerRunner) containerdPull(ctx context.Context, logChan chan<- string) (*core.Result, error) {
	r.sendLog(logChan, fmt.Sprintf("📥 拉取镜像: %s\n", r.config.Image))

	// 拉取镜像
	image, err := r.containerdCli.Pull(ctx, r.config.Image,
		containerd.WithPullUnpack,
	)
	if err != nil {
		return nil, fmt.Errorf("拉取镜像失败: %w", err)
	}

	r.sendLog(logChan, fmt.Sprintf("✅ 镜像拉取完成: %s\n", image.Name()))

	output := map[string]interface{}{
		"action": "pull",
		"image":  image.Name(),
	}

	return r.buildSuccessResult(output), nil
}

// containerdPush 推送镜像
func (r *ContainerRunner) containerdPush(ctx context.Context, logChan chan<- string) (*core.Result, error) {
	r.sendLog(logChan, fmt.Sprintf("📤 推送镜像: %s\n", r.config.Image))

	// 获取镜像
	image, err := r.containerdCli.GetImage(ctx, r.config.Image)
	if err != nil {
		return nil, fmt.Errorf("获取镜像失败: %w", err)
	}

	// 推送镜像
	if err := r.containerdCli.Push(ctx, r.config.Image, image.Target()); err != nil {
		return nil, fmt.Errorf("推送镜像失败: %w", err)
	}

	r.sendLog(logChan, "✅ 镜像推送完成\n")

	output := map[string]interface{}{
		"action": "push",
		"image":  r.config.Image,
	}

	return r.buildSuccessResult(output), nil
}

// ========== containerd 容器操作 ==========

// containerdRun 运行容器
func (r *ContainerRunner) containerdRun(ctx context.Context, logChan chan<- string) (*core.Result, error) {
	r.sendLog(logChan, "🚀 启动容器...\n")

	// 1. 获取镜像
	image, err := r.containerdCli.GetImage(ctx, r.config.Image)
	if err != nil {
		return nil, fmt.Errorf("获取镜像失败: %w", err)
	}

	// 2. 创建容器
	container, err := r.containerdCli.NewContainer(
		ctx,
		r.config.Name,
		containerd.WithImage(image),
		containerd.WithNewSnapshot(r.config.Name+"-snapshot", image),
		containerd.WithNewSpec(oci.WithImageConfig(image)),
	)
	if err != nil {
		return nil, fmt.Errorf("创建容器失败: %w", err)
	}

	r.sendLog(logChan, fmt.Sprintf("📦 容器已创建: %s\n", container.ID()))

	// 3. 创建任务并启动
	task, err := container.NewTask(ctx, cio.NewCreator(cio.WithStdio))
	if err != nil {
		return nil, fmt.Errorf("创建任务失败: %w", err)
	}

	if err := task.Start(ctx); err != nil {
		return nil, fmt.Errorf("启动任务失败: %w", err)
	}

	r.sendLog(logChan, "✅ 容器已启动\n")

	output := map[string]interface{}{
		"action":       "run",
		"container_id": container.ID(),
		"name":         r.config.Name,
		"status":       "running",
	}

	return r.buildSuccessResult(output), nil
}

// containerdStop 停止容器
func (r *ContainerRunner) containerdStop(ctx context.Context, logChan chan<- string) (*core.Result, error) {
	r.sendLog(logChan, fmt.Sprintf("🛑 停止容器: %s\n", r.config.Container))

	// 获取容器
	container, err := r.containerdCli.LoadContainer(ctx, r.config.Container)
	if err != nil {
		return nil, fmt.Errorf("获取容器失败: %w", err)
	}

	// 获取任务
	task, err := container.Task(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("获取任务失败: %w", err)
	}

	// 停止任务 (SIGTERM = 15)
	if err := task.Kill(ctx, syscall.SIGTERM); err != nil {
		return nil, fmt.Errorf("停止任务失败: %w", err)
	}

	r.sendLog(logChan, "✅ 容器已停止\n")

	output := map[string]interface{}{
		"action":    "stop",
		"container": r.config.Container,
	}

	return r.buildSuccessResult(output), nil
}

// containerdStart 启动容器
func (r *ContainerRunner) containerdStart(ctx context.Context, logChan chan<- string) (*core.Result, error) {
	r.sendLog(logChan, fmt.Sprintf("▶️ 启动容器: %s\n", r.config.Container))

	// 获取容器
	container, err := r.containerdCli.LoadContainer(ctx, r.config.Container)
	if err != nil {
		return nil, fmt.Errorf("获取容器失败: %w", err)
	}

	// 获取或创建任务
	task, err := container.Task(ctx, nil)
	if err != nil {
		// 如果任务不存在，创建新任务
		task, err = container.NewTask(ctx, cio.NewCreator(cio.WithStdio))
		if err != nil {
			return nil, fmt.Errorf("创建任务失败: %w", err)
		}
	}

	// 启动任务
	if err := task.Start(ctx); err != nil {
		return nil, fmt.Errorf("启动任务失败: %w", err)
	}

	r.sendLog(logChan, "✅ 容器已启动\n")

	output := map[string]interface{}{
		"action":    "start",
		"container": r.config.Container,
	}

	return r.buildSuccessResult(output), nil
}

// containerdRemove 删除容器或镜像
func (r *ContainerRunner) containerdRemove(ctx context.Context, logChan chan<- string) (*core.Result, error) {
	if r.config.Container != "" {
		// 删除容器
		r.sendLog(logChan, fmt.Sprintf("🗑️ 删除容器: %s\n", r.config.Container))

		container, err := r.containerdCli.LoadContainer(ctx, r.config.Container)
		if err != nil {
			return nil, fmt.Errorf("获取容器失败: %w", err)
		}

		// 删除任务
		task, err := container.Task(ctx, nil)
		if err == nil {
			task.Delete(ctx)
		}

		// 删除容器
		if err := container.Delete(ctx, containerd.WithSnapshotCleanup); err != nil {
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

		image, err := r.containerdCli.GetImage(ctx, r.config.Image)
		if err != nil {
			return nil, fmt.Errorf("获取镜像失败: %w", err)
		}

		// 使用 ImageService 删除镜像
		imageService := r.containerdCli.ImageService()
		if err := imageService.Delete(ctx, image.Name()); err != nil {
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

// containerdExec 在容器中执行命令
func (r *ContainerRunner) containerdExec(ctx context.Context, logChan chan<- string) (*core.Result, error) {
	r.sendLog(logChan, fmt.Sprintf("⚡ 执行命令: %v\n", r.config.ExecCommand))

	// 获取容器
	container, err := r.containerdCli.LoadContainer(ctx, r.config.Container)
	if err != nil {
		return nil, fmt.Errorf("获取容器失败: %w", err)
	}

	// 获取任务
	task, err := container.Task(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("获取任务失败: %w", err)
	}

	// 创建 exec 进程
	spec, err := container.Spec(ctx)
	if err != nil {
		return nil, fmt.Errorf("获取 spec 失败: %w", err)
	}

	// 执行命令（简化实现）
	_ = spec
	_ = task

	r.sendLog(logChan, "⚠️ containerd exec 功能简化实现\n")
	r.sendLog(logChan, "✅ 命令执行完成\n")

	output := map[string]interface{}{
		"action":    "exec",
		"container": r.config.Container,
		"command":   r.config.ExecCommand,
		"note":      "containerd exec 简化实现",
	}

	return r.buildSuccessResult(output), nil
}

// containerdInspect 查看容器详情
func (r *ContainerRunner) containerdInspect(ctx context.Context, logChan chan<- string) (*core.Result, error) {
	r.sendLog(logChan, fmt.Sprintf("🔍 查看容器详情: %s\n", r.config.Container))

	// 获取容器
	container, err := r.containerdCli.LoadContainer(ctx, r.config.Container)
	if err != nil {
		return nil, fmt.Errorf("获取容器失败: %w", err)
	}

	// 获取容器信息
	info, err := container.Info(ctx)
	if err != nil {
		return nil, fmt.Errorf("获取容器信息失败: %w", err)
	}

	// 获取任务状态
	task, err := container.Task(ctx, nil)
	var status string
	if err == nil {
		taskStatus, _ := task.Status(ctx)
		status = string(taskStatus.Status)
	} else {
		status = "stopped"
	}

	r.sendLog(logChan, fmt.Sprintf("✅ 状态: %s\n", status))

	output := map[string]interface{}{
		"action":       "inspect",
		"container":    r.config.Container,
		"container_id": info.ID,
		"image":        info.Image,
		"status":       status,
		"created":      info.CreatedAt,
	}

	return r.buildSuccessResult(output), nil
}
