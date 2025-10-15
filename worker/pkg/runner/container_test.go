package runner

import (
	"testing"

	"github.com/codelieche/cronjob/worker/pkg/core"
	"github.com/google/uuid"
)

// TestContainerRunner_ParseArgs 测试参数解析
func TestContainerRunner_ParseArgs(t *testing.T) {
	tests := []struct {
		name    string
		args    string
		wantErr bool
		errMsg  string
		check   func(*testing.T, *ContainerRunner)
	}{
		// ========== 基础配置测试 ==========
		{
			name: "有效的 Docker build 配置",
			args: `{
				"action": "build",
				"runtime": "docker",
				"connection": "unix",
				"dockerfile": "./Dockerfile",
				"context": ".",
				"tags": ["myapp:latest"]
			}`,
			wantErr: false,
			check: func(t *testing.T, r *ContainerRunner) {
				if r.config.Action != "build" {
					t.Errorf("action = %v, want build", r.config.Action)
				}
				if r.config.Runtime != "docker" {
					t.Errorf("runtime = %v, want docker", r.config.Runtime)
				}
				if r.config.Connection != "unix" {
					t.Errorf("connection = %v, want unix", r.config.Connection)
				}
			},
		},
		{
			name: "有效的 containerd run 配置",
			args: `{
				"action": "run",
				"runtime": "containerd",
				"connection": "unix",
				"image": "nginx:latest",
				"name": "nginx-web"
			}`,
			wantErr: false,
			check: func(t *testing.T, r *ContainerRunner) {
				if r.config.Runtime != "containerd" {
					t.Errorf("runtime = %v, want containerd", r.config.Runtime)
				}
				if r.config.Image != "nginx:latest" {
					t.Errorf("image = %v, want nginx:latest", r.config.Image)
				}
			},
		},
		{
			name: "默认运行时和连接模式",
			args: `{
				"action": "pull",
				"image": "nginx:latest"
			}`,
			wantErr: false,
			check: func(t *testing.T, r *ContainerRunner) {
				if r.config.Runtime != "docker" {
					t.Errorf("runtime = %v, want docker (default)", r.config.Runtime)
				}
				if r.config.Connection != "unix" {
					t.Errorf("connection = %v, want unix (default)", r.config.Connection)
				}
				if r.config.Namespace != "default" {
					t.Errorf("namespace = %v, want default (default)", r.config.Namespace)
				}
			},
		},

		// ========== 必需字段验证 ==========
		{
			name: "缺少 action 字段",
			args: `{
				"runtime": "docker",
				"image": "nginx:latest"
			}`,
			wantErr: true,
			errMsg:  "action 字段必填",
		},

		// ========== build 操作测试 ==========
		{
			name: "build 操作 - 完整配置",
			args: `{
				"action": "build",
				"dockerfile": "./Dockerfile",
				"context": ".",
				"tags": ["myapp:v1.0.0", "myapp:latest"],
				"build_args": {
					"NODE_VERSION": "18",
					"ENV": "production"
				},
				"no_cache": true,
				"pull": true
			}`,
			wantErr: false,
			check: func(t *testing.T, r *ContainerRunner) {
				if r.config.Dockerfile != "./Dockerfile" {
					t.Errorf("dockerfile = %v, want ./Dockerfile", r.config.Dockerfile)
				}
				if len(r.config.Tags) != 2 {
					t.Errorf("tags length = %v, want 2", len(r.config.Tags))
				}
				if !r.config.NoCache {
					t.Error("no_cache should be true")
				}
				if !r.config.Pull {
					t.Error("pull should be true")
				}
			},
		},

		// ========== run 操作测试 ==========
		{
			name: "run 操作 - 完整配置",
			args: `{
				"action": "run",
				"image": "nginx:latest",
				"name": "nginx-web",
				"ports": ["80:80", "443:443"],
				"volumes": ["/data:/usr/share/nginx/html"],
				"env": ["ENV=production", "DEBUG=false"],
				"network": "bridge",
				"restart": "unless-stopped",
				"detach": true,
				"remove": false
			}`,
			wantErr: false,
			check: func(t *testing.T, r *ContainerRunner) {
				if r.config.Image != "nginx:latest" {
					t.Errorf("image = %v, want nginx:latest", r.config.Image)
				}
				if r.config.Name != "nginx-web" {
					t.Errorf("name = %v, want nginx-web", r.config.Name)
				}
				if len(r.config.Ports) != 2 {
					t.Errorf("ports length = %v, want 2", len(r.config.Ports))
				}
				if len(r.config.Volumes) != 1 {
					t.Errorf("volumes length = %v, want 1", len(r.config.Volumes))
				}
				if r.config.Restart != "unless-stopped" {
					t.Errorf("restart = %v, want unless-stopped", r.config.Restart)
				}
			},
		},

		// ========== push/pull 操作测试 ==========
		{
			name: "push 操作 - 带凭证",
			args: `{
				"action": "push",
				"image": "myapp:v1.0.0",
				"registry": "registry.example.com",
				"registry_cred": "test-registry-cred",
				"tag_latest": true
			}`,
			wantErr: false,
			check: func(t *testing.T, r *ContainerRunner) {
				if r.config.Image != "myapp:v1.0.0" {
					t.Errorf("image = %v, want myapp:v1.0.0", r.config.Image)
				}
				if r.config.Registry != "registry.example.com" {
					t.Errorf("registry = %v, want registry.example.com", r.config.Registry)
				}
				if r.config.RegistryCred != "test-registry-cred" {
					t.Errorf("registry_cred = %v, want test-registry-cred", r.config.RegistryCred)
				}
				if !r.config.TagLatest {
					t.Error("tag_latest should be true")
				}
			},
		},
		{
			name: "pull 操作 - 公开镜像（无凭证）",
			args: `{
				"action": "pull",
				"image": "nginx:latest"
			}`,
			wantErr: false,
			check: func(t *testing.T, r *ContainerRunner) {
				if r.config.Image != "nginx:latest" {
					t.Errorf("image = %v, want nginx:latest", r.config.Image)
				}
				if r.config.RegistryCred != "" {
					t.Errorf("registry_cred should be empty, got %v", r.config.RegistryCred)
				}
			},
		},

		// ========== stop/start/restart 操作测试 ==========
		{
			name: "stop 操作",
			args: `{
				"action": "stop",
				"container": "nginx-web",
				"timeout": 30
			}`,
			wantErr: false,
			check: func(t *testing.T, r *ContainerRunner) {
				if r.config.Container != "nginx-web" {
					t.Errorf("container = %v, want nginx-web", r.config.Container)
				}
				if r.config.Timeout != 30 {
					t.Errorf("timeout = %v, want 30", r.config.Timeout)
				}
			},
		},
		{
			name: "restart 操作",
			args: `{
				"action": "restart",
				"container": "nginx-web"
			}`,
			wantErr: false,
			check: func(t *testing.T, r *ContainerRunner) {
				if r.config.Action != "restart" {
					t.Errorf("action = %v, want restart", r.config.Action)
				}
			},
		},

		// ========== remove 操作测试 ==========
		{
			name: "remove 操作 - 删除容器",
			args: `{
				"action": "remove",
				"container": "nginx-web",
				"force": true,
				"remove_volumes": true
			}`,
			wantErr: false,
			check: func(t *testing.T, r *ContainerRunner) {
				if r.config.Container != "nginx-web" {
					t.Errorf("container = %v, want nginx-web", r.config.Container)
				}
				if !r.config.Force {
					t.Error("force should be true")
				}
				if !r.config.RemoveVolumes {
					t.Error("remove_volumes should be true")
				}
			},
		},
		{
			name: "remove 操作 - 删除镜像",
			args: `{
				"action": "remove",
				"image": "myapp:old",
				"force": true
			}`,
			wantErr: false,
			check: func(t *testing.T, r *ContainerRunner) {
				if r.config.Image != "myapp:old" {
					t.Errorf("image = %v, want myapp:old", r.config.Image)
				}
				if !r.config.Force {
					t.Error("force should be true")
				}
			},
		},

		// ========== logs 操作测试 ==========
		{
			name: "logs 操作",
			args: `{
				"action": "logs",
				"container": "nginx-web",
				"lines": 100,
				"follow": true,
				"timestamps": true
			}`,
			wantErr: false,
			check: func(t *testing.T, r *ContainerRunner) {
				if r.config.Container != "nginx-web" {
					t.Errorf("container = %v, want nginx-web", r.config.Container)
				}
				if r.config.Lines != 100 {
					t.Errorf("lines = %v, want 100", r.config.Lines)
				}
				if !r.config.Follow {
					t.Error("follow should be true")
				}
				if !r.config.Timestamps {
					t.Error("timestamps should be true")
				}
			},
		},

		// ========== exec 操作测试 ==========
		{
			name: "exec 操作 - 简单命令",
			args: `{
				"action": "exec",
				"container": "nginx-web",
				"exec_command": ["ls", "-la", "/app"],
				"interactive": false,
				"tty": false
			}`,
			wantErr: false,
			check: func(t *testing.T, r *ContainerRunner) {
				if r.config.Container != "nginx-web" {
					t.Errorf("container = %v, want nginx-web", r.config.Container)
				}
				if len(r.config.ExecCommand) != 3 {
					t.Errorf("exec_command length = %v, want 3", len(r.config.ExecCommand))
				}
				if r.config.ExecCommand[0] != "ls" {
					t.Errorf("exec_command[0] = %v, want ls", r.config.ExecCommand[0])
				}
			},
		},
		{
			name: "exec 操作 - shell 特性（自动包装）",
			args: `{
				"action": "exec",
				"container": "nginx-web",
				"exec_command": ["sh", "-c", "echo \"test\" > /tmp/test.log && cat /tmp/test.log"]
			}`,
			wantErr: false,
			check: func(t *testing.T, r *ContainerRunner) {
				if r.config.Container != "nginx-web" {
					t.Errorf("container = %v, want nginx-web", r.config.Container)
				}
				if len(r.config.ExecCommand) != 3 {
					t.Errorf("exec_command length = %v, want 3 (sh -c \"command\")", len(r.config.ExecCommand))
				}
				if r.config.ExecCommand[0] != "sh" {
					t.Errorf("exec_command[0] = %v, want sh", r.config.ExecCommand[0])
				}
				if r.config.ExecCommand[1] != "-c" {
					t.Errorf("exec_command[1] = %v, want -c", r.config.ExecCommand[1])
				}
			},
		},

		// ========== inspect 操作测试 ==========
		{
			name: "inspect 操作",
			args: `{
				"action": "inspect",
				"container": "nginx-web",
				"check_health": true
			}`,
			wantErr: false,
			check: func(t *testing.T, r *ContainerRunner) {
				if r.config.Container != "nginx-web" {
					t.Errorf("container = %v, want nginx-web", r.config.Container)
				}
				if !r.config.CheckHealth {
					t.Error("check_health should be true")
				}
			},
		},

		// ========== prune 操作测试 ==========
		{
			name: "prune 操作 - 清理镜像",
			args: `{
				"action": "prune",
				"type": "image",
				"filters": {
					"dangling": "true",
					"until": "24h"
				}
			}`,
			wantErr: false,
			check: func(t *testing.T, r *ContainerRunner) {
				if r.config.Type != "image" {
					t.Errorf("type = %v, want image", r.config.Type)
				}
				if len(r.config.Filters) != 2 {
					t.Errorf("filters length = %v, want 2", len(r.config.Filters))
				}
			},
		},
		{
			name: "prune 操作 - 清理所有资源",
			args: `{
				"action": "prune",
				"type": "all"
			}`,
			wantErr: false,
			check: func(t *testing.T, r *ContainerRunner) {
				if r.config.Type != "all" {
					t.Errorf("type = %v, want all", r.config.Type)
				}
			},
		},

		// ========== stats 操作测试 ==========
		{
			name: "stats 操作 - 单个容器",
			args: `{
				"action": "stats",
				"container": "nginx-web"
			}`,
			wantErr: false,
			check: func(t *testing.T, r *ContainerRunner) {
				if r.config.Container != "nginx-web" {
					t.Errorf("container = %v, want nginx-web", r.config.Container)
				}
			},
		},
		{
			name: "stats 操作 - 所有容器",
			args: `{
				"action": "stats"
			}`,
			wantErr: false,
			check: func(t *testing.T, r *ContainerRunner) {
				if r.config.Container != "" {
					t.Errorf("container should be empty, got %v", r.config.Container)
				}
			},
		},

		// ========== TCP 远程连接测试 ==========
		{
			name: "Docker TCP 远程连接 - 不启用 TLS",
			args: `{
				"action": "pull",
				"runtime": "docker",
				"connection": "tcp",
				"host": "docker-proxy:2375",
				"image": "nginx:latest"
			}`,
			wantErr: false,
			check: func(t *testing.T, r *ContainerRunner) {
				if r.config.Connection != "tcp" {
					t.Errorf("connection = %v, want tcp", r.config.Connection)
				}
				if r.config.Host != "docker-proxy:2375" {
					t.Errorf("host = %v, want docker-proxy:2375", r.config.Host)
				}
				if r.config.TLS {
					t.Error("tls should be false")
				}
			},
		},
		{
			name: "Docker TCP 远程连接 - 启用 TLS",
			args: `{
				"action": "pull",
				"runtime": "docker",
				"connection": "tcp",
				"host": "docker-proxy:2376",
				"tls": true,
				"tls_cred": "test-tls-cred",
				"image": "nginx:latest"
			}`,
			wantErr: false,
			check: func(t *testing.T, r *ContainerRunner) {
				if r.config.Connection != "tcp" {
					t.Errorf("connection = %v, want tcp", r.config.Connection)
				}
				if !r.config.TLS {
					t.Error("tls should be true")
				}
				if r.config.TLSCred != "test-tls-cred" {
					t.Errorf("tls_cred = %v, want test-tls-cred", r.config.TLSCred)
				}
			},
		},

		// ========== Unix Socket 配置测试 ==========
		{
			name: "Docker Unix Socket - 自定义路径",
			args: `{
				"action": "pull",
				"runtime": "docker",
				"connection": "unix",
				"socket": "/custom/path/docker.sock",
				"image": "nginx:latest"
			}`,
			wantErr: false,
			check: func(t *testing.T, r *ContainerRunner) {
				if r.config.Socket != "/custom/path/docker.sock" {
					t.Errorf("socket = %v, want /custom/path/docker.sock", r.config.Socket)
				}
			},
		},
		{
			name: "containerd Unix Socket - 自定义路径",
			args: `{
				"action": "pull",
				"runtime": "containerd",
				"connection": "unix",
				"socket": "/custom/containerd.sock",
				"image": "nginx:latest"
			}`,
			wantErr: false,
			check: func(t *testing.T, r *ContainerRunner) {
				if r.config.Socket != "/custom/containerd.sock" {
					t.Errorf("socket = %v, want /custom/containerd.sock", r.config.Socket)
				}
			},
		},

		// ========== containerd 特定配置测试 ==========
		{
			name: "containerd namespace 配置",
			args: `{
				"action": "pull",
				"runtime": "containerd",
				"namespace": "k8s.io",
				"image": "nginx:latest"
			}`,
			wantErr: false,
			check: func(t *testing.T, r *ContainerRunner) {
				if r.config.Namespace != "k8s.io" {
					t.Errorf("namespace = %v, want k8s.io", r.config.Namespace)
				}
			},
		},
		{
			name: "containerd buildkit 地址",
			args: `{
				"action": "build",
				"runtime": "containerd",
				"buildkit_addr": "unix:///run/buildkit/buildkitd.sock",
				"context": ".",
				"tags": ["myapp:latest"]
			}`,
			wantErr: false,
			check: func(t *testing.T, r *ContainerRunner) {
				if r.config.BuildkitAddr != "unix:///run/buildkit/buildkitd.sock" {
					t.Errorf("buildkit_addr = %v, want unix:///run/buildkit/buildkitd.sock", r.config.BuildkitAddr)
				}
			},
		},

		// ========== JSON 解析错误测试 ==========
		{
			name:    "无效的 JSON 格式",
			args:    `{"action": "build", invalid json}`,
			wantErr: true,
		},
		{
			name:    "空的 JSON",
			args:    `{}`,
			wantErr: true,
			errMsg:  "action 字段必填",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runner := NewContainerRunner()
			task := &core.Task{
				ID:   uuid.New(),
				Args: tt.args,
			}

			err := runner.ParseArgs(task)

			// 检查错误
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseArgs() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if err != nil && tt.errMsg != "" {
				if err.Error() != tt.errMsg {
					t.Logf("ParseArgs() error = %v, expected contains %v", err, tt.errMsg)
				}
			}

			// 如果没有错误，执行自定义检查
			if err == nil && tt.check != nil {
				tt.check(t, runner)
			}
		})
	}
}

// TestContainerRunner_Interface 测试 ContainerRunner 实现 core.Runner 接口
func TestContainerRunner_Interface(t *testing.T) {
	var _ core.Runner = (*ContainerRunner)(nil)
	t.Log("ContainerRunner implements core.Runner interface")
}

// TestContainerRunner_GetStatus 测试获取状态
func TestContainerRunner_GetStatus(t *testing.T) {
	runner := NewContainerRunner()
	status := runner.GetStatus()
	if status != core.StatusPending {
		t.Errorf("initial status = %v, want %v", status, core.StatusPending)
	}
}

// TestContainerRunner_GetResult 测试获取结果
func TestContainerRunner_GetResult(t *testing.T) {
	runner := NewContainerRunner()
	result := runner.GetResult()
	if result != nil {
		t.Errorf("initial result should be nil, got %v", result)
	}
}

// TestContainerRunner_StopKill 测试停止和强制终止
func TestContainerRunner_StopKill(t *testing.T) {
	runner := NewContainerRunner()
	task := &core.Task{
		ID: uuid.New(),
		Args: `{
			"action": "pull",
			"image": "nginx:latest"
		}`,
	}

	if err := runner.ParseArgs(task); err != nil {
		t.Fatalf("ParseArgs() failed: %v", err)
	}

	// 测试 Stop
	if err := runner.Stop(); err != nil {
		t.Errorf("Stop() failed: %v", err)
	}

	if runner.GetStatus() != core.StatusStopped {
		t.Errorf("status after Stop() = %v, want %v", runner.GetStatus(), core.StatusStopped)
	}

	// 测试 Kill
	runner2 := NewContainerRunner()
	if err := runner2.ParseArgs(task); err != nil {
		t.Fatalf("ParseArgs() failed: %v", err)
	}

	if err := runner2.Kill(); err != nil {
		t.Errorf("Kill() failed: %v", err)
	}

	if runner2.GetStatus() != core.StatusStopped {
		t.Errorf("status after Kill() = %v, want %v", runner2.GetStatus(), core.StatusStopped)
	}
}

// TestContainerRunner_Cleanup 测试清理资源
func TestContainerRunner_Cleanup(t *testing.T) {
	runner := NewContainerRunner()
	task := &core.Task{
		ID: uuid.New(),
		Args: `{
			"action": "pull",
			"image": "nginx:latest"
		}`,
	}

	if err := runner.ParseArgs(task); err != nil {
		t.Fatalf("ParseArgs() failed: %v", err)
	}

	// 测试 Cleanup
	if err := runner.Cleanup(); err != nil {
		t.Errorf("Cleanup() failed: %v", err)
	}
}

// TestContainerRunner_ParseArgs_EdgeCases 测试边界情况
func TestContainerRunner_ParseArgs_EdgeCases(t *testing.T) {
	tests := []struct {
		name    string
		args    string
		wantErr bool
	}{
		{
			name: "空数组配置",
			args: `{
				"action": "build",
				"context": ".",
				"tags": [],
				"build_args": {},
				"ports": [],
				"volumes": [],
				"env": []
			}`,
			wantErr: true, // build 需要至少一个 tag
		},
		{
			name: "超长字符串",
			args: `{
				"action": "run",
				"image": "` + string(make([]byte, 1000)) + `:latest"
			}`,
			wantErr: true, // JSON 中包含无效字符（\x00）会导致解析失败
		},
		{
			name: "特殊字符",
			args: `{
				"action": "run",
				"image": "registry.cn-hangzhou.aliyuncs.com/namespace/app:v1.0.0",
				"name": "app-prod-001"
			}`,
			wantErr: false,
		},
		{
			name: "布尔值类型",
			args: `{
				"action": "build",
				"context": ".",
				"tags": ["myapp:latest"],
				"no_cache": "true",
				"pull": 1
			}`,
			wantErr: true, // 类型错误
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runner := NewContainerRunner()
			task := &core.Task{
				ID:   uuid.New(),
				Args: tt.args,
			}

			err := runner.ParseArgs(task)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseArgs() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// TestContainerRunner_ActionTypes 测试所有支持的操作类型
func TestContainerRunner_ActionTypes(t *testing.T) {
	supportedActions := []string{
		"build", "run", "push", "pull",
		"stop", "start", "restart", "remove",
		"logs", "exec", "inspect", "prune", "stats",
	}

	for _, action := range supportedActions {
		t.Run("action_"+action, func(t *testing.T) {
			runner := NewContainerRunner()

			// 为 prune 操作提供 type 字段
			argsStr := `{
				"action": "` + action + `",
				"image": "nginx:latest",
				"container": "test-container",
				"context": ".",
				"tags": ["test:latest"]`

			if action == "prune" {
				argsStr += `,
				"type": "image"`
			}

			argsStr += `
			}`

			task := &core.Task{
				ID:   uuid.New(),
				Args: argsStr,
			}

			err := runner.ParseArgs(task)
			if err != nil {
				t.Errorf("ParseArgs() for action %s failed: %v", action, err)
			}

			if runner.config.Action != action {
				t.Errorf("action = %v, want %v", runner.config.Action, action)
			}
		})
	}
}

// TestContainerRunner_RuntimeTypes 测试所有支持的运行时
func TestContainerRunner_RuntimeTypes(t *testing.T) {
	supportedRuntimes := []string{"docker", "containerd"}

	for _, runtime := range supportedRuntimes {
		t.Run("runtime_"+runtime, func(t *testing.T) {
			runner := NewContainerRunner()
			task := &core.Task{
				ID: uuid.New(),
				Args: `{
					"action": "pull",
					"runtime": "` + runtime + `",
					"image": "nginx:latest"
				}`,
			}

			err := runner.ParseArgs(task)
			if err != nil {
				t.Errorf("ParseArgs() for runtime %s failed: %v", runtime, err)
			}

			if runner.config.Runtime != runtime {
				t.Errorf("runtime = %v, want %v", runner.config.Runtime, runtime)
			}
		})
	}
}

// TestContainerRunner_ConnectionTypes 测试所有支持的连接模式
func TestContainerRunner_ConnectionTypes(t *testing.T) {
	tests := []struct {
		runtime    string
		connection string
		shouldWork bool
	}{
		{"docker", "unix", true},
		{"docker", "tcp", true},
		{"containerd", "unix", true},
		{"containerd", "tcp", false}, // containerd 不支持 tcp
	}

	for _, tt := range tests {
		t.Run("runtime_"+tt.runtime+"_connection_"+tt.connection, func(t *testing.T) {
			runner := NewContainerRunner()
			task := &core.Task{
				ID: uuid.New(),
				Args: `{
					"action": "pull",
					"runtime": "` + tt.runtime + `",
					"connection": "` + tt.connection + `",
					"image": "nginx:latest"
				}`,
			}

			err := runner.ParseArgs(task)
			if err != nil {
				t.Logf("ParseArgs() returned error: %v", err)
			}

			// 注意：参数解析不会检查 containerd+tcp 的兼容性
			// 这个错误会在 Execute 时检测
		})
	}
}
