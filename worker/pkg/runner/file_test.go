package runner

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/codelieche/cronjob/worker/pkg/core"
	"github.com/google/uuid"
)

// TestFileRunner_ParseArgs 测试参数解析
func TestFileRunner_ParseArgs(t *testing.T) {
	tests := []struct {
		name    string
		args    string
		wantErr bool
		errMsg  string
	}{
		{
			name: "有效的cleanup配置（本地）",
			args: `{
				"action": "cleanup",
				"path": "/tmp",
				"pattern": "*.tmp",
				"older_than": "7d",
				"dry_run": true
			}`,
			wantErr: false,
		},
		{
			name: "有效的cleanup配置（远程）",
			args: `{
				"action": "cleanup",
				"host": "192.168.1.100",
				"port": 22,
				"credential": "test-cred-id",
				"username": "root",
				"path": "/var/log/app",
				"pattern": "*.log",
				"older_than": "7d"
			}`,
			wantErr: false,
		},
		{
			name: "有效的backup配置",
			args: `{
				"action": "backup",
				"source": "/tmp/test",
				"target": "/tmp/backup",
				"compress": true
			}`,
			wantErr: false,
		},
		{
			name: "有效的compress配置",
			args: `{
				"action": "compress",
				"source": "/tmp/test"
			}`,
			wantErr: false,
		},
		{
			name: "有效的stat配置",
			args: `{
				"action": "stat",
				"path": "/tmp",
				"limit": 10
			}`,
			wantErr: false,
		},
		{
			name: "不支持的操作类型",
			args: `{
				"action": "invalid",
				"path": "/tmp"
			}`,
			wantErr: true,
			errMsg:  "不支持的操作类型",
		},
		{
			name: "cleanup缺少path",
			args: `{
				"action": "cleanup",
				"pattern": "*.log"
			}`,
			wantErr: true,
			errMsg:  "path 不能为空",
		},
		{
			name: "cleanup缺少pattern",
			args: `{
				"action": "cleanup",
				"path": "/tmp"
			}`,
			wantErr: true,
			errMsg:  "pattern 不能为空",
		},
		{
			name: "backup缺少source",
			args: `{
				"action": "backup",
				"target": "/tmp/backup"
			}`,
			wantErr: true,
			errMsg:  "source 不能为空",
		},
		{
			name: "backup缺少target",
			args: `{
				"action": "backup",
				"source": "/tmp/test"
			}`,
			wantErr: true,
			errMsg:  "target 不能为空",
		},
		{
			name: "远程模式缺少credential",
			args: `{
				"action": "cleanup",
				"host": "192.168.1.100",
				"path": "/var/log",
				"pattern": "*.log"
			}`,
			wantErr: true,
			errMsg:  "credential 不能为空",
		},
		{
			name: "远程模式默认端口和用户名",
			args: `{
				"action": "cleanup",
				"host": "192.168.1.100",
				"credential": "test-cred-id",
				"path": "/var/log",
				"pattern": "*.log"
			}`,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runner := NewFileRunner()
			task := &core.Task{
				ID:       uuid.New(),
				Category: "file",
				Args:     tt.args,
			}

			err := runner.ParseArgs(task)

			if tt.wantErr {
				if err == nil {
					t.Errorf("ParseArgs() 期望错误，但没有错误")
					return
				}
				if tt.errMsg != "" && !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("ParseArgs() 错误信息 = %v, 期望包含 %v", err, tt.errMsg)
				}
			} else {
				if err != nil {
					t.Errorf("ParseArgs() 不期望错误，但得到 = %v", err)
				}

				// 验证远程模式的默认值
				if runner.config.Host != "" {
					if runner.config.Port == 0 {
						t.Error("远程模式应该设置默认端口")
					}
					if runner.config.Username == "" {
						t.Error("远程模式应该设置默认用户名")
					}
				}
			}
		})
	}
}

// TestFileRunner_ValidatePath 测试路径验证
func TestFileRunner_ValidatePath(t *testing.T) {
	tests := []struct {
		name    string
		path    string
		isLocal bool // 是否本地模式
		wantErr bool
		errMsg  string
	}{
		{
			name:    "空路径",
			path:    "",
			isLocal: true,
			wantErr: true,
			errMsg:  "路径不能为空",
		},
		{
			name:    "禁止的根目录",
			path:    "/",
			isLocal: true,
			wantErr: true,
			errMsg:  "禁止操作系统目录",
		},
		{
			name:    "禁止的/etc目录",
			path:    "/etc",
			isLocal: true,
			wantErr: true,
			errMsg:  "禁止操作系统目录",
		},
		{
			name:    "禁止的/usr目录",
			path:    "/usr/bin",
			isLocal: true,
			wantErr: true,
			errMsg:  "禁止操作系统目录",
		},
		{
			name:    "允许的/tmp目录",
			path:    "/tmp",
			isLocal: true,
			wantErr: false,
		},
		{
			name:    "本地模式：不在白名单的路径",
			path:    "/home/user/test",
			isLocal: true,
			wantErr: true,
			errMsg:  "路径不在白名单中",
		},
		{
			name:    "远程模式：不检查路径存在性",
			path:    "/var/log/app",
			isLocal: false,
			wantErr: false, // 远程模式不验证路径是否存在
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runner := NewFileRunner()
			runner.config.Action = "cleanup"
			runner.config.Path = tt.path

			if !tt.isLocal {
				runner.config.Host = "test-host" // 设置为远程模式
			}

			err := runner.validatePath(tt.path)

			if tt.wantErr {
				if err == nil {
					t.Errorf("validatePath() 期望错误，但没有错误")
					return
				}
				if tt.errMsg != "" && !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("validatePath() 错误信息 = %v, 期望包含 %v", err, tt.errMsg)
				}
			} else {
				if err != nil {
					t.Errorf("validatePath() 不期望错误，但得到 = %v", err)
				}
			}
		})
	}
}

// TestParseOlderThan 测试时间解析
func TestParseOlderThan(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    time.Duration
		wantErr bool
	}{
		{
			name:    "7天",
			input:   "7d",
			want:    7 * 24 * time.Hour,
			wantErr: false,
		},
		{
			name:    "30天",
			input:   "30d",
			want:    30 * 24 * time.Hour,
			wantErr: false,
		},
		{
			name:    "2小时",
			input:   "2h",
			want:    2 * time.Hour,
			wantErr: false,
		},
		{
			name:    "30分钟",
			input:   "30m",
			want:    30 * time.Minute,
			wantErr: false,
		},
		{
			name:    "无效格式",
			input:   "invalid",
			wantErr: true,
		},
		{
			name:    "无效单位",
			input:   "7x",
			wantErr: true,
		},
		{
			name:    "空字符串",
			input:   "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseOlderThan(tt.input)

			if tt.wantErr {
				if err == nil {
					t.Errorf("parseOlderThan() 期望错误，但没有错误")
				}
			} else {
				if err != nil {
					t.Errorf("parseOlderThan() 不期望错误，但得到 = %v", err)
					return
				}
				if got != tt.want {
					t.Errorf("parseOlderThan() = %v, want %v", got, tt.want)
				}
			}
		})
	}
}

// TestParseLargerThan 测试大小解析
func TestParseLargerThan(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    int64
		wantErr bool
	}{
		{
			name:    "1KB",
			input:   "1K",
			want:    1024,
			wantErr: false,
		},
		{
			name:    "100MB",
			input:   "100M",
			want:    100 * 1024 * 1024,
			wantErr: false,
		},
		{
			name:    "1GB",
			input:   "1G",
			want:    1 * 1024 * 1024 * 1024,
			wantErr: false,
		},
		{
			name:    "小写字母",
			input:   "100m",
			want:    100 * 1024 * 1024,
			wantErr: false,
		},
		{
			name:    "无效格式",
			input:   "invalid",
			wantErr: true,
		},
		{
			name:    "无效单位",
			input:   "100X",
			wantErr: true,
		},
		{
			name:    "空字符串",
			input:   "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseLargerThan(tt.input)

			if tt.wantErr {
				if err == nil {
					t.Errorf("parseLargerThan() 期望错误，但没有错误")
				}
			} else {
				if err != nil {
					t.Errorf("parseLargerThan() 不期望错误，但得到 = %v", err)
					return
				}
				if got != tt.want {
					t.Errorf("parseLargerThan() = %v, want %v", got, tt.want)
				}
			}
		})
	}
}

// TestFormatSize 测试文件大小格式化
func TestFormatSize(t *testing.T) {
	tests := []struct {
		name  string
		input int64
		want  string
	}{
		{
			name:  "0字节",
			input: 0,
			want:  "0 B",
		},
		{
			name:  "1KB",
			input: 1024,
			want:  "1.0 KB",
		},
		{
			name:  "1MB",
			input: 1024 * 1024,
			want:  "1.0 MB",
		},
		{
			name:  "1GB",
			input: 1024 * 1024 * 1024,
			want:  "1.0 GB",
		},
		{
			name:  "1.5GB",
			input: 1536 * 1024 * 1024,
			want:  "1.5 GB",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatSize(tt.input)
			if got != tt.want {
				t.Errorf("formatSize() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestFileRunner_CleanupLocal 测试本地清理操作
func TestFileRunner_CleanupLocal(t *testing.T) {
	// 创建临时测试目录
	tempDir, err := os.MkdirTemp("", "filerunner-cleanup-test-*")
	if err != nil {
		t.Fatalf("创建临时目录失败: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// 创建一些测试文件
	testFiles := []struct {
		name string
		age  time.Duration // 相对于现在的年龄
	}{
		{"old1.log", 10 * 24 * time.Hour},  // 10天前
		{"old2.log", 8 * 24 * time.Hour},   // 8天前
		{"recent.log", 2 * 24 * time.Hour}, // 2天前
		{"new.log", 0},                     // 刚创建
	}

	for _, tf := range testFiles {
		filePath := filepath.Join(tempDir, tf.name)
		if err := os.WriteFile(filePath, []byte("test content"), 0644); err != nil {
			t.Fatalf("创建测试文件失败: %v", err)
		}

		// 设置文件的修改时间
		if tf.age > 0 {
			modTime := time.Now().Add(-tf.age)
			if err := os.Chtimes(filePath, modTime, modTime); err != nil {
				t.Fatalf("设置文件时间失败: %v", err)
			}
		}
	}

	t.Run("DryRun模式：不应该删除文件", func(t *testing.T) {
		runner := NewFileRunner()
		runner.config.Action = "cleanup"
		runner.config.Path = tempDir
		runner.config.Pattern = "*.log"
		runner.config.OlderThan = "7d"
		runner.config.DryRun = true

		ctx := context.Background()
		logChan := make(chan string, 100)

		deletedFiles, _, err := runner.executeCleanupLocal(ctx, logChan)
		if err != nil {
			t.Fatalf("executeCleanupLocal() 失败: %v", err)
		}

		// DryRun 应该找到 2 个超过 7 天的文件
		if len(deletedFiles) != 2 {
			t.Errorf("DryRun 应该找到 2 个文件，实际 %d 个", len(deletedFiles))
		}

		// 验证文件仍然存在
		for _, tf := range testFiles {
			filePath := filepath.Join(tempDir, tf.name)
			if _, err := os.Stat(filePath); os.IsNotExist(err) {
				t.Errorf("DryRun 模式不应该删除文件: %s", tf.name)
			}
		}
	})

	t.Run("实际删除模式：应该删除旧文件", func(t *testing.T) {
		// 重新创建测试文件
		for _, tf := range testFiles {
			filePath := filepath.Join(tempDir, tf.name)
			if err := os.WriteFile(filePath, []byte("test content"), 0644); err != nil {
				t.Fatalf("创建测试文件失败: %v", err)
			}
			if tf.age > 0 {
				modTime := time.Now().Add(-tf.age)
				if err := os.Chtimes(filePath, modTime, modTime); err != nil {
					t.Fatalf("设置文件时间失败: %v", err)
				}
			}
		}

		runner := NewFileRunner()
		runner.config.Action = "cleanup"
		runner.config.Path = tempDir
		runner.config.Pattern = "*.log"
		runner.config.OlderThan = "7d"
		runner.config.DryRun = false

		ctx := context.Background()
		logChan := make(chan string, 100)

		deletedFiles, _, err := runner.executeCleanupLocal(ctx, logChan)
		if err != nil {
			t.Fatalf("executeCleanupLocal() 失败: %v", err)
		}

		// 应该删除 2 个超过 7 天的文件
		if len(deletedFiles) != 2 {
			t.Errorf("应该删除 2 个文件，实际 %d 个", len(deletedFiles))
		}

		// 验证旧文件已删除
		for _, tf := range testFiles {
			filePath := filepath.Join(tempDir, tf.name)
			_, err := os.Stat(filePath)
			if tf.age > 7*24*time.Hour {
				// 应该已删除
				if !os.IsNotExist(err) {
					t.Errorf("旧文件应该已删除: %s", tf.name)
				}
			} else {
				// 应该仍存在
				if os.IsNotExist(err) {
					t.Errorf("新文件不应该被删除: %s", tf.name)
				}
			}
		}
	})
}

// TestFileRunner_RunnerInterface 测试 Runner 接口实现
func TestFileRunner_RunnerInterface(t *testing.T) {
	// 验证 FileRunner 实现了 Runner 接口
	var _ core.Runner = (*FileRunner)(nil)

	runner := NewFileRunner()

	// 测试 GetStatus
	status := runner.GetStatus()
	if status != core.StatusPending {
		t.Errorf("初始状态应该是 Pending，实际 %v", status)
	}

	// 测试 GetResult（未执行时应该为 nil）
	result := runner.GetResult()
	if result != nil {
		t.Errorf("未执行时 GetResult() 应该返回 nil")
	}

	// 测试 Stop
	if err := runner.Stop(); err != nil {
		t.Errorf("Stop() 失败: %v", err)
	}

	// 测试 Kill
	if err := runner.Kill(); err != nil {
		t.Errorf("Kill() 失败: %v", err)
	}

	// 测试 Cleanup
	if err := runner.Cleanup(); err != nil {
		t.Errorf("Cleanup() 失败: %v", err)
	}
}

// TestShellQuote 测试 Shell 转义
func TestShellQuote(t *testing.T) {
	runner := NewFileRunner()

	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "普通路径",
			input: "/var/log/app",
			want:  "'/var/log/app'",
		},
		{
			name:  "包含空格的路径",
			input: "/var/log/my app",
			want:  "'/var/log/my app'",
		},
		{
			name:  "包含单引号的路径",
			input: "/var/log/user's app",
			want:  "'/var/log/user'\\''s app'",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := runner.shellQuote(tt.input)
			if got != tt.want {
				t.Errorf("shellQuote() = %v, want %v", got, tt.want)
			}
		})
	}
}
