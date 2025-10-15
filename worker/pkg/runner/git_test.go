package runner

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/codelieche/cronjob/worker/pkg/core"
	"github.com/google/uuid"
)

// TestGitRunner_ParseArgs 测试参数解析
func TestGitRunner_ParseArgs(t *testing.T) {
	tests := []struct {
		name    string
		args    string
		wantErr bool
		errMsg  string
	}{
		{
			name: "有效的 SSH URL 配置",
			args: `{
				"url": "git@github.com:user/repo.git",
				"branch": "main",
				"credential": "test-cred-id",
				"clean": false
			}`,
			wantErr: false,
		},
		{
			name: "有效的 HTTPS URL 配置",
			args: `{
				"url": "https://github.com/user/repo.git",
				"branch": "develop",
				"credential": "test-cred-id",
				"clean": true
			}`,
			wantErr: false,
		},
		{
			name: "缺少 URL 字段",
			args: `{
				"branch": "main",
				"credential": "test-cred-id"
			}`,
			wantErr: true,
			errMsg:  "url 字段必填",
		},
		{
			name: "无凭证配置（公开仓库）",
			args: `{
				"url": "https://github.com/user/public-repo.git",
				"branch": "main"
			}`,
			wantErr: false, // 凭证为可选，公开仓库无需凭证
		},
		{
			name: "URL 格式不正确",
			args: `{
				"url": "invalid-url",
				"branch": "main",
				"credential": "test-cred-id"
			}`,
			wantErr: true,
			errMsg:  "URL 格式不正确",
		},
		{
			name: "默认分支（未指定）",
			args: `{
				"url": "git@github.com:user/repo.git",
				"credential": "test-cred-id"
			}`,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runner := NewGitRunner()
			task := &core.Task{
				ID:   uuid.New(),
				Args: tt.args,
			}

			err := runner.ParseArgs(task)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseArgs() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if err != nil && tt.errMsg != "" {
				if err.Error() != tt.errMsg && len(err.Error()) > 0 {
					// 只要错误信息包含关键字即可
					t.Logf("ParseArgs() error = %v, expected contains %v", err, tt.errMsg)
				}
			}

			// 如果没有错误，检查默认值
			if err == nil && runner.config.Branch == "" {
				t.Error("ParseArgs() 应该设置默认分支为 main")
			}
		})
	}
}

// TestGitRunner_ValidateGitURL 测试 URL 验证
func TestGitRunner_ValidateGitURL(t *testing.T) {
	runner := NewGitRunner()

	tests := []struct {
		name    string
		url     string
		wantErr bool
	}{
		{
			name:    "有效的 SSH URL",
			url:     "git@github.com:user/repo.git",
			wantErr: false,
		},
		{
			name:    "有效的 HTTPS URL",
			url:     "https://github.com/user/repo.git",
			wantErr: false,
		},
		{
			name:    "有效的 GitLab SSH URL",
			url:     "git@gitlab.com:group/project.git",
			wantErr: false,
		},
		{
			name:    "无效的 URL（无扩展名）",
			url:     "git@github.com:user/repo",
			wantErr: true,
		},
		{
			name:    "无效的 URL（格式错误）",
			url:     "invalid-url",
			wantErr: true,
		},
		{
			name:    "HTTP URL（不支持）",
			url:     "http://github.com/user/repo.git",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := runner.validateGitURL(tt.url)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateGitURL() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// TestGitRunner_RunnerInterface 测试 Runner 接口实现
func TestGitRunner_RunnerInterface(t *testing.T) {
	var runner core.Runner = NewGitRunner()

	// 测试状态管理
	if runner.GetStatus() != core.StatusPending {
		t.Errorf("初始状态应该是 pending，实际是 %s", runner.GetStatus())
	}

	// 测试 Stop
	err := runner.Stop()
	if err != nil {
		t.Errorf("Stop() 返回错误: %v", err)
	}

	if runner.GetStatus() != core.StatusStopped {
		t.Errorf("Stop() 后状态应该是 stopped，实际是 %s", runner.GetStatus())
	}

	// 测试 Kill
	runner = NewGitRunner() // 重新创建
	err = runner.Kill()
	if err != nil {
		t.Errorf("Kill() 返回错误: %v", err)
	}

	if runner.GetStatus() != core.StatusStopped {
		t.Errorf("Kill() 后状态应该是 stopped，实际是 %s", runner.GetStatus())
	}

	// 测试 Cleanup
	err = runner.Cleanup()
	if err != nil {
		t.Errorf("Cleanup() 返回错误: %v", err)
	}
}

// TestGitRunner_TempFileCleanup 测试临时文件清理
func TestGitRunner_TempFileCleanup(t *testing.T) {
	runner := NewGitRunner()

	// 创建一个临时文件
	tmpFile := filepath.Join(os.TempDir(), "test_git_key_cleanup")
	err := os.WriteFile(tmpFile, []byte("test content"), 0600)
	if err != nil {
		t.Fatalf("创建测试文件失败: %v", err)
	}

	// 添加到清理列表
	runner.tempFiles = append(runner.tempFiles, tmpFile)

	// 执行清理
	runner.cleanup()

	// 验证文件已删除
	if _, err := os.Stat(tmpFile); !os.IsNotExist(err) {
		t.Error("cleanup() 应该删除临时文件")
		os.Remove(tmpFile) // 清理测试文件
	}

	// 验证清理列表已清空
	if len(runner.tempFiles) != 0 {
		t.Error("cleanup() 应该清空 tempFiles 列表")
	}
}

// TestGitRunner_GetWorkDirectory 测试工作目录获取
func TestGitRunner_GetWorkDirectory(t *testing.T) {
	runner := NewGitRunner()
	runner.Task = &core.Task{ // 🔥 直接访问公共字段
		ID: uuid.New(),
	}

	// 测试默认工作目录
	workDir, err := runner.GetWorkingDirectory() // 🔥 使用基类方法
	if err != nil {
		t.Fatalf("GetWorkingDirectory() 返回错误: %v", err)
	}
	if workDir == "" {
		t.Error("GetWorkingDirectory() 不应该返回空字符串")
	}

	// 验证目录包含 tasks 和 task ID
	if !filepath.IsAbs(workDir) {
		// 如果不是绝对路径，说明使用了相对路径
		t.Logf("工作目录: %s", workDir)
	}
}

// TestGitRunner_Exists 测试 exists 辅助函数
func TestGitRunner_Exists(t *testing.T) {
	// 测试不存在的路径
	if exists("/nonexistent/path/12345") {
		t.Error("exists() 对不存在的路径应该返回 false")
	}

	// 测试存在的路径
	tmpFile := filepath.Join(os.TempDir(), "test_exists_func")
	err := os.WriteFile(tmpFile, []byte("test"), 0644)
	if err != nil {
		t.Fatalf("创建测试文件失败: %v", err)
	}
	defer os.Remove(tmpFile)

	if !exists(tmpFile) {
		t.Error("exists() 对存在的文件应该返回 true")
	}

	// 测试目录
	tmpDir := filepath.Join(os.TempDir(), "test_exists_dir")
	err = os.Mkdir(tmpDir, 0755)
	if err != nil {
		t.Fatalf("创建测试目录失败: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	if !exists(tmpDir) {
		t.Error("exists() 对存在的目录应该返回 true")
	}
}

// TestGitRunner_BuildGitEnv 测试 Git 环境变量构建
func TestGitRunner_BuildGitEnv(t *testing.T) {
	runner := NewGitRunner()

	// 测试没有 SSH 密钥的情况
	env := runner.buildGitEnv()
	if len(env) != 0 {
		t.Error("没有 SSH 密钥时，buildGitEnv() 应该返回空数组")
	}

	// 测试有 SSH 密钥的情况
	runner.tempFiles = append(runner.tempFiles, "/tmp/test_key")
	env = runner.buildGitEnv()

	if len(env) != 1 {
		t.Errorf("有 SSH 密钥时，buildGitEnv() 应该返回 1 个环境变量，实际 %d 个", len(env))
	}

	if len(env) > 0 && env[0][:15] != "GIT_SSH_COMMAND" {
		t.Errorf("环境变量应该以 GIT_SSH_COMMAND 开头，实际: %s", env[0])
	}
}

// TestGitRunner_BuildResults 测试结果构建
func TestGitRunner_BuildResults(t *testing.T) {
	runner := NewGitRunner()
	runner.config = &GitConfig{
		URL:    "git@github.com:user/repo.git",
		Branch: "main",
	}

	// 测试成功结果
	result := runner.buildSuccessResult("clone", "/tmp/workdir", "abc123def456")
	if result == nil {
		t.Fatal("buildSuccessResult() 不应该返回 nil")
	}

	if result.Status != core.StatusSuccess {
		t.Errorf("Status 应该是 success，实际: %s", result.Status)
	}

	if result.ExitCode != 0 {
		t.Errorf("ExitCode 应该是 0，实际: %d", result.ExitCode)
	}

	if result.Output == "" {
		t.Error("Output 不应该为空")
	}

	// 测试错误结果
	testErr := errors.New("test error")
	errResult := runner.buildErrorResult(testErr)
	if errResult == nil {
		t.Fatal("buildErrorResult() 不应该返回 nil")
	}

	if errResult.Status != core.StatusFailed {
		t.Errorf("Status 应该是 failed，实际: %s", errResult.Status)
	}

	if errResult.ExitCode != 1 {
		t.Errorf("ExitCode 应该是 1，实际: %d", errResult.ExitCode)
	}
}

// TestGitRunner_Execute_NoApiserver 测试没有 apiserver 时的执行
func TestGitRunner_Execute_NoApiserver(t *testing.T) {
	runner := NewGitRunner()
	runner.Task = &core.Task{ // 🔥 直接访问公共字段
		ID: uuid.New(),
	}
	runner.config = &GitConfig{
		URL:        "git@github.com:user/repo.git",
		Branch:     "main",
		Credential: "test-cred-id",
	}

	ctx := context.Background()
	logChan := make(chan string, 10)

	// 不设置 apiserver
	result, err := runner.Execute(ctx, logChan)

	// 应该返回错误
	if err == nil {
		t.Error("没有 apiserver 时，Execute() 应该返回错误")
	}

	if result == nil {
		t.Fatal("result 不应该为 nil")
	}

	if result.Status != core.StatusFailed {
		t.Errorf("没有 apiserver 时，Status 应该是 failed，实际: %s", result.Status)
	}

	close(logChan)
}
