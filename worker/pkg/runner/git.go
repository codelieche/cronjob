package runner

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/codelieche/cronjob/worker/pkg/core"
)

// GitConfig Git 操作配置（极简版）
type GitConfig struct {
	// URL 仓库地址（必填）
	// 支持 SSH: git@github.com:user/repo.git
	// 支持 HTTPS: https://github.com/user/repo.git
	URL string `json:"url"`

	// Branch 分支名（可选，默认 main）
	Branch string `json:"branch"`

	// Credential 凭证 ID（必填）
	// 支持类型：ssh_private_key, username_password, api_token
	Credential string `json:"credential"`

	// Clean 清空模式（可选，默认 false）
	// false: 智能 sync（不存在就 clone，存在就 pull）✅ 推荐
	// true:  强制重来（删除后 clone）⚠️ 慎用
	Clean bool `json:"clean"`
}

// GitResult Git 操作结果
type GitResult struct {
	// Action 操作类型（clone/pull）
	Action string `json:"action"`

	// Repository 仓库信息
	Repository string `json:"repository"` // 仓库 URL
	Branch     string `json:"branch"`     // 分支名
	Commit     string `json:"commit"`     // 当前提交哈希

	// ChangedFiles 变更统计（pull 时有效）
	ChangedFiles int `json:"changed_files,omitempty"` // 变更文件数
	Insertions   int `json:"insertions,omitempty"`    // 新增行数
	Deletions    int `json:"deletions,omitempty"`     // 删除行数

	// ExecuteInfo 执行信息
	WorkDir   string  `json:"work_dir"`  // 工作目录
	Duration  float64 `json:"duration"`  // 执行时长（秒）
	Timestamp string  `json:"timestamp"` // 执行时间
}

// HTTPAuth HTTP 认证信息
type HTTPAuth struct {
	Username string
	Password string
}

// GitRunner Git 操作执行器
type GitRunner struct {
	BaseRunner // 🔥 嵌入基类

	config *GitConfig

	// 临时文件清理
	tempFiles []string

	// HTTP 认证（用于 HTTPS URL）
	httpAuth *HTTPAuth
}

// NewGitRunner 创建新的 GitRunner 实例
func NewGitRunner() *GitRunner {
	r := &GitRunner{
		tempFiles: []string{},
	}
	r.InitBase() // 🔥 初始化基类
	return r
}

// ParseArgs 解析任务参数
func (r *GitRunner) ParseArgs(task *core.Task) error {
	r.Task = task // 🔥 直接访问公共字段

	// 1. 解析 JSON 配置
	r.config = &GitConfig{}
	if err := json.Unmarshal([]byte(task.Args), r.config); err != nil {
		return fmt.Errorf("解析配置失败: %w", err)
	}

	// 2. 验证必需字段
	if r.config.URL == "" {
		return fmt.Errorf("url 字段必填")
	}
	// 注意：credential 为可选字段，公开仓库不需要凭证

	// 3. 设置默认值
	if r.config.Branch == "" {
		r.config.Branch = "main"
	}

	// 4. 验证 URL 格式
	if err := r.validateGitURL(r.config.URL); err != nil {
		return err
	}

	return nil
}

// validateGitURL 验证 Git URL 格式
func (r *GitRunner) validateGitURL(url string) error {
	// SSH 格式：git@github.com:user/repo.git
	sshPattern := `^git@[\w\.\-]+:[\w\-\/]+\.git$`

	// HTTPS 格式：https://github.com/user/repo.git
	httpsPattern := `^https://[\w\.\-]+/[\w\-\/]+\.git$`

	sshMatch, _ := regexp.MatchString(sshPattern, url)
	httpsMatch, _ := regexp.MatchString(httpsPattern, url)

	if !sshMatch && !httpsMatch {
		return fmt.Errorf("URL 格式不正确，支持 SSH 或 HTTPS 格式")
	}

	return nil
}

// Execute 执行任务
func (r *GitRunner) Execute(ctx context.Context, logChan chan<- string) (*core.Result, error) {
	defer r.cleanup() // 清理临时文件

	// 🔥 直接访问公共字段
	r.Ctx, r.Cancel = context.WithCancel(ctx)
	r.Status = core.StatusRunning
	r.StartTime = time.Now()

	r.sendLog(logChan, "🚀 GitRunner 启动\n")
	r.SendLog(logChan, fmt.Sprintf("📦 仓库: %s\n", r.config.URL))
	r.SendLog(logChan, fmt.Sprintf("🌿 分支: %s\n", r.config.Branch))

	// 1. 获取工作目录
	workDir, err := r.GetWorkingDirectory()
	if err != nil {
		r.Result = r.buildErrorResult(err) // 🔥 直接访问
		return r.Result, err
	}
	r.SendLog(logChan, fmt.Sprintf("📁 工作目录: %s\n", workDir))

	// 2. 准备凭证（如果配置了凭证）
	if r.config.Credential != "" {
		if err := r.prepareCredentials(logChan); err != nil {
			r.Result = r.buildErrorResult(err) // 🔥 直接访问
			return r.Result, err
		}
	} else {
		r.sendLog(logChan, "ℹ️  未配置凭证，尝试访问公开仓库\n")
	}

	// 3. 执行 sync 操作
	action, err := r.syncRepository(workDir, logChan)
	if err != nil {
		r.Result = r.buildErrorResult(err) // 🔥 直接访问
		return r.Result, err
	}

	// 4. 获取提交信息
	commit, err := r.getCurrentCommit(workDir)
	if err != nil {
		r.sendLog(logChan, fmt.Sprintf("⚠️  获取提交信息失败: %v\n", err))
		commit = "unknown"
	}

	// 5. 构建成功结果
	r.sendLog(logChan, fmt.Sprintf("✅ %s 成功\n", action))
	r.sendLog(logChan, fmt.Sprintf("📌 当前提交: %s\n", commit[:8]))

	r.Result = r.buildSuccessResult(action, workDir, commit) // 🔥 直接访问
	r.Status = core.StatusSuccess                            // 🔥 直接访问
	return r.Result, nil
}

// prepareCredentials 准备 Git 凭证
func (r *GitRunner) prepareCredentials(logChan chan<- string) error {
	r.sendLog(logChan, "🔐 获取 Git 凭证...\n")

	// 1. 检查 apiserver 是否已注入
	if r.Apiserver == nil { // 🔥 直接访问
		err := fmt.Errorf("apiserver 未初始化，无法获取凭证")
		r.sendLog(logChan, fmt.Sprintf("❌ %v\n", err))
		return err
	}

	// 2. 从 apiserver 获取凭证
	cred, err := r.Apiserver.GetCredential(r.config.Credential) // 🔥 直接访问
	if err != nil {
		r.sendLog(logChan, fmt.Sprintf("❌ 获取凭证失败: %v\n", err))
		return fmt.Errorf("获取凭证失败: %w", err)
	}

	r.sendLog(logChan, fmt.Sprintf("✅ 成功获取凭证: %s\n", cred.Name))

	// 3. 根据凭证类型处理
	switch cred.Category {
	case "ssh_private_key":
		return r.prepareSSHKey(cred)
	case "username_password", "api_token":
		return r.prepareHTTPAuth(cred)
	default:
		return fmt.Errorf("不支持的凭证类型: %s", cred.Category)
	}
}

// prepareSSHKey 准备 SSH 私钥
func (r *GitRunner) prepareSSHKey(cred *core.Credential) error {
	// 1. 从凭证中获取私钥
	privateKey, ok := cred.GetString("private_key")
	if !ok || privateKey == "" {
		return fmt.Errorf("凭证缺少 private_key 字段")
	}

	// 2. 创建临时文件
	tmpKeyFile := filepath.Join(os.TempDir(), fmt.Sprintf("git_key_%s", r.Task.ID.String())) // 🔥 直接访问

	// 3. 写入私钥（权限 0600）
	if err := os.WriteFile(tmpKeyFile, []byte(privateKey), 0600); err != nil {
		return fmt.Errorf("写入 SSH 密钥失败: %w", err)
	}

	// 4. 记录待清理文件
	r.tempFiles = append(r.tempFiles, tmpKeyFile)

	return nil
}

// prepareHTTPAuth 准备 HTTP 认证（username_password）
func (r *GitRunner) prepareHTTPAuth(cred *core.Credential) error {
	// 1. 获取用户名和密码
	var username, password string
	var ok bool

	if cred.Category == "username_password" {
		username, ok = cred.GetString("username")
		if !ok || username == "" {
			return fmt.Errorf("凭证缺少 username 字段")
		}
		password, ok = cred.GetString("password")
		if !ok || password == "" {
			return fmt.Errorf("凭证缺少 password 字段")
		}
	} else if cred.Category == "api_token" {
		// api_token 类型：token 作为密码，用户名可以是任意值（如 "git" 或 "oauth2"）
		username = "git"
		password, ok = cred.GetString("token")
		if !ok || password == "" {
			return fmt.Errorf("凭证缺少 token 字段")
		}
	}

	// 2. 保存 HTTP 认证信息（用于后续修改 URL）
	r.httpAuth = &HTTPAuth{
		Username: username,
		Password: password,
	}

	return nil
}

// buildAuthURL 构建带认证信息的 URL（用于 HTTPS）
func (r *GitRunner) buildAuthURL() string {
	// 如果没有 HTTP 认证，直接返回原 URL
	if r.httpAuth == nil {
		return r.config.URL
	}

	// 解析 URL
	u, err := url.Parse(r.config.URL)
	if err != nil {
		return r.config.URL
	}

	// 只处理 HTTPS URL
	if u.Scheme != "https" && u.Scheme != "http" {
		return r.config.URL
	}

	// 设置用户名和密码
	u.User = url.UserPassword(r.httpAuth.Username, r.httpAuth.Password)

	return u.String()
}

// buildGitEnv 构建 Git 环境变量
func (r *GitRunner) buildGitEnv() []string {
	env := []string{}

	// SSH 凭证
	if len(r.tempFiles) > 0 {
		sshKeyFile := r.tempFiles[0] // 第一个是 SSH 密钥
		sshCommand := fmt.Sprintf("ssh -i %s -o StrictHostKeyChecking=no", sshKeyFile)
		env = append(env, fmt.Sprintf("GIT_SSH_COMMAND=%s", sshCommand))
	}

	return env
}

// syncRepository 同步仓库（智能判断 clone/pull）
func (r *GitRunner) syncRepository(workDir string, logChan chan<- string) (string, error) {
	gitDir := filepath.Join(workDir, ".git")

	if r.config.Clean {
		// 清空模式：删除后重新 clone
		r.sendLog(logChan, "🗑️  清空模式：删除现有内容\n")

		// 删除 .git 目录
		if exists(gitDir) {
			if err := os.RemoveAll(gitDir); err != nil {
				return "", fmt.Errorf("删除 .git 失败: %w", err)
			}
		}

		// 删除所有文件（保留目录本身）
		entries, _ := os.ReadDir(workDir)
		for _, entry := range entries {
			path := filepath.Join(workDir, entry.Name())
			os.RemoveAll(path)
		}

		r.sendLog(logChan, "📥 开始克隆仓库...\n")
		return "clone", r.gitClone(workDir, logChan)
	} else {
		// 智能模式：自动判断
		if !exists(gitDir) {
			// 不是 Git 仓库：clone
			r.sendLog(logChan, "📥 首次克隆仓库...\n")
			return "clone", r.gitClone(workDir, logChan)
		} else {
			// 是 Git 仓库：pull
			r.sendLog(logChan, "🔄 拉取最新代码...\n")
			return "pull", r.gitPull(workDir, logChan)
		}
	}
}

// gitClone 克隆仓库到工作目录
func (r *GitRunner) gitClone(workDir string, logChan chan<- string) error {
	// 构建带认证的 URL（如果使用 HTTP 认证）
	authURL := r.buildAuthURL()

	// 构建命令
	args := []string{
		"clone",
		"-b", r.config.Branch, // 指定分支
		"--single-branch", // 只克隆单个分支
		"--depth", "1",    // 浅克隆（节省时间和空间）
		authURL,
		".", // 克隆到当前目录（workDir）
	}

	cmd := exec.CommandContext(r.Ctx, "git", args...) // 🔥 直接访问
	cmd.Dir = workDir

	// 设置环境变量（SSH 凭证）
	cmd.Env = append(os.Environ(), r.buildGitEnv()...)

	// 执行命令
	output, err := cmd.CombinedOutput()
	r.sendLog(logChan, string(output))

	if err != nil {
		return fmt.Errorf("克隆失败: %w\n%s", err, string(output))
	}

	return nil
}

// gitPull 拉取仓库更新
func (r *GitRunner) gitPull(workDir string, logChan chan<- string) error {
	// 0. 如果使用 HTTP 认证，更新远程 origin 的 URL
	if r.httpAuth != nil {
		authURL := r.buildAuthURL()
		setURLCmd := exec.CommandContext(r.Ctx, "git", "remote", "set-url", "origin", authURL) // 🔥 使用基类方法
		setURLCmd.Dir = workDir
		setURLCmd.Env = append(os.Environ(), r.buildGitEnv()...)
		setURLCmd.CombinedOutput() // 忽略错误
	}

	// 1. 先 checkout 到指定分支
	checkoutCmd := exec.CommandContext(r.Ctx, "git", "checkout", r.config.Branch) // 🔥 使用基类方法
	checkoutCmd.Dir = workDir
	checkoutCmd.Env = append(os.Environ(), r.buildGitEnv()...)

	output, err := checkoutCmd.CombinedOutput()
	if err != nil {
		r.sendLog(logChan, string(output))
		// checkout 失败不致命，继续尝试 pull
	}

	// 2. pull 最新代码
	pullCmd := exec.CommandContext(r.Ctx, "git", "pull", "origin", r.config.Branch) // 🔥 使用基类方法
	pullCmd.Dir = workDir
	pullCmd.Env = append(os.Environ(), r.buildGitEnv()...)

	output, err = pullCmd.CombinedOutput()
	r.sendLog(logChan, string(output))

	if err != nil {
		return fmt.Errorf("拉取失败: %w\n%s", err, string(output))
	}

	return nil
}

// getCurrentCommit 获取当前提交哈希
func (r *GitRunner) getCurrentCommit(workDir string) (string, error) {
	cmd := exec.Command("git", "rev-parse", "HEAD")
	cmd.Dir = workDir

	output, err := cmd.Output()
	if err != nil {
		return "", err
	}

	return strings.TrimSpace(string(output)), nil
}

// Stop 停止任务执行（优雅停止）
func (r *GitRunner) Stop() error {
	r.Lock()
	defer r.Unlock()

	// 🔥 直接访问取消上下文
	if r.Cancel != nil {
		r.Cancel()
	}
	r.Status = core.StatusStopped
	return nil
}

// Kill 强制终止任务执行
func (r *GitRunner) Kill() error {
	return r.Stop() // GitRunner 不需要区分 Stop/Kill
}

// GetStatus, GetResult 方法继承自 BaseRunner

// Cleanup 清理资源
func (r *GitRunner) Cleanup() error {
	// 🔥 直接访问取消上下文
	if r.Cancel != nil {
		r.Cancel()
	}

	// 清理临时文件
	r.cleanup()

	return nil
}

// cleanup 清理临时文件
func (r *GitRunner) cleanup() {
	for _, file := range r.tempFiles {
		os.Remove(file)
	}
	r.tempFiles = nil
}

// buildSuccessResult 构建成功结果
func (r *GitRunner) buildSuccessResult(action, workDir, commit string) *core.Result {
	duration := time.Since(r.StartTime).Seconds() // 🔥 直接访问

	gitResult := &GitResult{
		Action:     action,
		Repository: r.config.URL,
		Branch:     r.config.Branch,
		Commit:     commit,
		WorkDir:    workDir,
		Duration:   duration,
		Timestamp:  time.Now().Format(time.RFC3339),
	}

	outputJSON, _ := json.Marshal(gitResult)

	return &core.Result{
		Status:    core.StatusSuccess,
		ExitCode:  0,
		Output:    string(outputJSON),
		Error:     "",
		StartTime: r.StartTime, // 🔥 使用基类方法
		EndTime:   time.Now(),
	}
}

// buildErrorResult 构建错误结果
func (r *GitRunner) buildErrorResult(err error) *core.Result {
	return &core.Result{
		Status:    core.StatusFailed,
		ExitCode:  1,
		Output:    "",
		Error:     err.Error(),
		StartTime: r.StartTime, // 🔥 使用基类方法
		EndTime:   time.Now(),
	}
}

// sendLog 发送日志
func (r *GitRunner) sendLog(logChan chan<- string, message string) {
	if logChan != nil {
		select {
		case logChan <- message:
		default:
		}
	}
}

// exists 检查路径是否存在
func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
