package runner

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/codelieche/cronjob/worker/pkg/core"
	"github.com/codelieche/cronjob/worker/pkg/utils/logger"
	"go.uber.org/zap"
	"golang.org/x/crypto/ssh"
)

// FileConfig 文件操作配置
type FileConfig struct {
	Action string `json:"action"` // cleanup/backup/compress/stat

	// 远程连接（可选，为空则本地操作）
	Host       string `json:"host"`       // 远程主机地址
	Port       int    `json:"port"`       // SSH 端口（默认 22）
	Credential string `json:"credential"` // 凭证 ID
	Username   string `json:"username"`   // SSH 用户名（默认 root）

	// 通用字段
	Path      string `json:"path"`      // 目标路径
	Pattern   string `json:"pattern"`   // 文件匹配模式
	Recursive bool   `json:"recursive"` // 递归子目录

	// cleanup 专用
	OlderThan  string   `json:"older_than"`  // 7d, 30d, 90d
	LargerThan string   `json:"larger_than"` // 100M, 1G
	DryRun     bool     `json:"dry_run"`     // 试运行模式
	Exclude    []string `json:"exclude"`     // 排除路径

	// backup 专用
	Source      string `json:"source"`      // 源路径
	Target      string `json:"target"`      // 目标路径
	Compress    bool   `json:"compress"`    // 是否压缩
	Incremental bool   `json:"incremental"` // 增量备份
	KeepDays    int    `json:"keep_days"`   // 保留天数

	// compress 专用
	Format       string `json:"format"`        // tar.gz, zip
	RemoveSource bool   `json:"remove_source"` // 压缩后删除源
	Level        int    `json:"level"`         // 压缩级别

	// stat 专用
	SortBy string `json:"sort_by"` // size/time/name
	Limit  int    `json:"limit"`   // 返回数量
}

// FileRunner 文件操作执行器
//
// 支持本地和远程文件操作（通过纯 SSH 命令）
// 核心功能：
// - cleanup：文件清理（支持时间、大小筛选、DryRun）
// - backup：文件备份（支持压缩、增量）
// - compress：文件压缩（tar.gz/zip）
// - stat：文件统计（磁盘占用分析）
//
// 远程操作：纯 SSH 命令，无需 SFTP
type FileRunner struct {
	task      *core.Task         // 任务对象
	config    FileConfig         // 文件操作配置
	apiserver core.Apiserver     // API Server 客户端（用于获取凭证）
	status    core.Status        // 当前状态
	result    *core.Result       // 执行结果
	cancel    context.CancelFunc // 取消函数
	mutex     sync.RWMutex       // 并发保护

	// SSH 连接（仅远程模式）
	sshClient *ssh.Client // SSH 客户端（纯命令方式，无需 SFTP）
}

// NewFileRunner 创建新的 FileRunner
func NewFileRunner() *FileRunner {
	return &FileRunner{
		status: core.StatusPending,
	}
}

// ParseArgs 解析任务参数
func (r *FileRunner) ParseArgs(task *core.Task) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	r.task = task

	// 解析 args（JSON 字符串）
	if err := json.Unmarshal([]byte(task.Args), &r.config); err != nil {
		return fmt.Errorf("解析文件操作配置失败: %w", err)
	}

	// 验证操作类型
	supportedActions := map[string]bool{
		"cleanup":  true,
		"backup":   true,
		"compress": true,
		"stat":     true,
	}
	if !supportedActions[r.config.Action] {
		return fmt.Errorf("不支持的操作类型: %s（支持: cleanup, backup, compress, stat）", r.config.Action)
	}

	// 验证必填字段（根据操作类型）
	switch r.config.Action {
	case "cleanup":
		if r.config.Path == "" {
			return fmt.Errorf("cleanup 操作：path 不能为空")
		}
		if r.config.Pattern == "" {
			return fmt.Errorf("cleanup 操作：pattern 不能为空")
		}
	case "backup":
		if r.config.Source == "" {
			return fmt.Errorf("backup 操作：source 不能为空")
		}
		if r.config.Target == "" {
			return fmt.Errorf("backup 操作：target 不能为空")
		}
	case "compress":
		if r.config.Source == "" {
			return fmt.Errorf("compress 操作：source 不能为空")
		}
	case "stat":
		if r.config.Path == "" {
			return fmt.Errorf("stat 操作：path 不能为空")
		}
	}

	// 远程模式验证
	if r.config.Host != "" {
		if r.config.Port == 0 {
			r.config.Port = 22 // 默认端口
		}
		if r.config.Username == "" {
			r.config.Username = "root" // 默认用户
		}
		if r.config.Credential == "" {
			return fmt.Errorf("远程模式：credential 不能为空")
		}
	}

	return nil
}

// SetTask 设置任务（实现 Runner 接口）
func (r *FileRunner) SetTask(task *core.Task) error {
	return r.ParseArgs(task)
}

// SetApiserver 设置 API Server 客户端（用于获取凭证）
func (r *FileRunner) SetApiserver(apiserver core.Apiserver) {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	r.apiserver = apiserver
}

// GetStatus 获取当前状态
func (r *FileRunner) GetStatus() core.Status {
	r.mutex.RLock()
	defer r.mutex.RUnlock()
	return r.status
}

// GetResult 获取执行结果
func (r *FileRunner) GetResult() *core.Result {
	r.mutex.RLock()
	defer r.mutex.RUnlock()
	return r.result
}

// Execute 执行文件操作
func (r *FileRunner) Execute(ctx context.Context, logChan chan<- string) (*core.Result, error) {
	startTime := time.Now()

	// 更新状态为运行中
	r.mutex.Lock()
	r.status = core.StatusRunning
	r.mutex.Unlock()

	// 创建可取消的上下文
	ctx, cancel := context.WithCancel(ctx)
	r.mutex.Lock()
	r.cancel = cancel
	r.mutex.Unlock()
	defer cancel()

	r.sendLog(logChan, fmt.Sprintf("🚀 FileRunner 启动 - 操作类型: %s\n", r.config.Action))

	// 1. 判断本地/远程模式
	isRemote := r.config.Host != ""
	if isRemote {
		r.sendLog(logChan, fmt.Sprintf("🌐 远程模式: %s@%s:%d\n",
			r.config.Username, r.config.Host, r.config.Port))

		// 建立 SSH 连接
		if err := r.connectSSH(ctx, logChan); err != nil {
			r.sendLog(logChan, fmt.Sprintf("❌ SSH 连接失败: %v\n", err))
			return r.buildErrorResult("SSH 连接失败", err, startTime), err
		}
		defer r.closeSSH()
		r.sendLog(logChan, "✅ SSH 连接成功\n")
	} else {
		r.sendLog(logChan, "💻 本地模式\n")
	}

	// 2. 验证路径安全性
	r.sendLog(logChan, "🔒 验证路径安全性...\n")
	if err := r.validatePath(r.getTargetPath()); err != nil {
		r.sendLog(logChan, fmt.Sprintf("❌ 路径验证失败: %v\n", err))
		return r.buildErrorResult("路径验证失败", err, startTime), err
	}
	r.sendLog(logChan, "✅ 路径验证通过\n")

	// 3. 根据操作类型执行
	var result *core.Result
	var err error

	switch r.config.Action {
	case "cleanup":
		result, err = r.executeCleanup(ctx, logChan, startTime)
	case "backup":
		result, err = r.executeBackup(ctx, logChan, startTime)
	case "compress":
		result, err = r.executeCompress(ctx, logChan, startTime)
	case "stat":
		result, err = r.executeStat(ctx, logChan, startTime)
	default:
		err = fmt.Errorf("不支持的操作类型: %s", r.config.Action)
		return r.buildErrorResult("操作类型错误", err, startTime), err
	}

	if err != nil {
		return r.buildErrorResult("执行失败", err, startTime), err
	}

	// 更新状态
	r.mutex.Lock()
	r.status = core.StatusSuccess
	r.result = result
	r.mutex.Unlock()

	return result, nil
}

// Stop 停止执行
func (r *FileRunner) Stop() error {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	if r.cancel != nil {
		r.cancel()
	}

	r.status = core.StatusStopped
	return nil
}

// Kill 强制终止执行
func (r *FileRunner) Kill() error {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	if r.cancel != nil {
		r.cancel()
	}

	// 关闭 SSH 连接（如果有）
	if r.sshClient != nil {
		r.sshClient.Close()
		r.sshClient = nil
	}

	r.status = core.StatusFailed
	return nil
}

// Cleanup 清理资源
func (r *FileRunner) Cleanup() error {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	// 取消上下文
	if r.cancel != nil {
		r.cancel()
		r.cancel = nil
	}

	// 关闭 SSH 连接
	if r.sshClient != nil {
		r.sshClient.Close()
		r.sshClient = nil
	}

	return nil
}

// getTargetPath 获取目标路径（根据操作类型）
func (r *FileRunner) getTargetPath() string {
	switch r.config.Action {
	case "cleanup", "stat":
		return r.config.Path
	case "backup":
		return r.config.Source
	case "compress":
		return r.config.Source
	default:
		return ""
	}
}

// sendLog 发送日志到通道
func (r *FileRunner) sendLog(logChan chan<- string, message string) {
	if logChan != nil {
		select {
		case logChan <- message:
		default:
			// 通道已满或已关闭，记录到日志
			logger.Logger().Warn("日志通道发送失败",
				zap.String("message", message))
		}
	}
}

// buildErrorResult 构建错误结果
func (r *FileRunner) buildErrorResult(message string, err error, startTime time.Time) *core.Result {
	endTime := time.Now()
	errorMsg := fmt.Sprintf("%s: %v", message, err)
	return &core.Result{
		Status:     core.StatusFailed,
		Error:      errorMsg,
		ExecuteLog: errorMsg,
		Output:     "",
		StartTime:  startTime,
		EndTime:    endTime,
		Duration:   endTime.Sub(startTime).Milliseconds(),
	}
}

// addTimestampToPath 为路径添加时间戳
func (r *FileRunner) addTimestampToPath(originalPath string, compress bool) string {
	// 生成时间戳：202510150002 格式（精确到分钟）
	timestamp := time.Now().Format("200601021504")

	// 获取目录和文件名
	dir := filepath.Dir(originalPath)
	base := filepath.Base(originalPath)

	// 移除可能已有的扩展名
	ext := filepath.Ext(base)
	nameWithoutExt := strings.TrimSuffix(base, ext)

	// 构建新文件名：原名_时间戳
	newName := fmt.Sprintf("%s_%s", nameWithoutExt, timestamp)

	// 根据是否压缩添加扩展名
	if compress {
		if !strings.HasSuffix(originalPath, ".tar.gz") {
			newName += ".tar.gz"
		} else {
			newName += ext // 保留原扩展名
		}
	} else {
		if ext != "" {
			newName += ext // 保留原扩展名
		}
	}

	return filepath.Join(dir, newName)
}

// ============================================================================
// 路径安全验证
// ============================================================================

// 路径白名单（可通过环境变量配置：FILE_RUNNER_ALLOWED_PATHS）
var defaultAllowedPaths = []string{
	// 日志类
	"/var/log",
	"/opt/logs",

	// 数据类
	"/data",
	"/opt/data",

	// 备份类
	"/backup",
	"/data/backup",

	// 临时类
	"/tmp",

	// Web类
	"/var/www/uploads",

	// 应用类（可选）
	"/opt/app",
	"/home/*/app",
}

// 禁止路径（硬编码，不可配置）
var forbiddenPaths = []string{
	"/",
	"/etc",
	"/usr",
	"/bin",
	"/sbin",
	"/boot",
	"/lib",
	"/lib64",
	"/sys",
	"/proc",
	"/dev",
	"/root",
}

// getAllowedPaths 获取允许的路径列表
func getAllowedPaths() []string {
	// 从环境变量读取（逗号分隔）
	if envPaths := os.Getenv("FILE_RUNNER_ALLOWED_PATHS"); envPaths != "" {
		paths := strings.Split(envPaths, ",")
		result := make([]string, 0, len(paths))
		for _, p := range paths {
			if trimmed := strings.TrimSpace(p); trimmed != "" {
				result = append(result, trimmed)
			}
		}
		return result
	}
	return defaultAllowedPaths
}

// validatePath 验证路径安全性
func (r *FileRunner) validatePath(path string) error {
	// 1. 空路径检查
	if path == "" {
		return fmt.Errorf("路径不能为空")
	}

	// 2. 检查禁止路径
	for _, forbidden := range forbiddenPaths {
		// 精确匹配或前缀匹配
		if path == forbidden || strings.HasPrefix(path, forbidden+"/") {
			return fmt.Errorf("禁止操作系统目录: %s", path)
		}
	}

	// 3. 检查白名单（仅本地模式）
	if r.config.Host == "" {
		allowedPaths := getAllowedPaths()
		allowed := false
		for _, allowedPath := range allowedPaths {
			if strings.HasPrefix(path, allowedPath) {
				allowed = true
				break
			}
		}
		if !allowed {
			return fmt.Errorf("路径不在白名单中: %s（允许的路径: %v）",
				path, allowedPaths)
		}
	}

	// 4. 检查路径是否存在（仅本地模式）
	if r.config.Host == "" {
		if _, err := os.Stat(path); os.IsNotExist(err) {
			// 某些操作（如 backup 的 target）允许不存在
			if r.config.Action == "backup" && path == r.config.Target {
				return nil // 备份目标路径允许不存在
			}
			return fmt.Errorf("路径不存在: %s", path)
		}
	}

	return nil
}

// ============================================================================
// cleanup 操作实现
// ============================================================================

// executeCleanup 执行清理操作
func (r *FileRunner) executeCleanup(ctx context.Context, logChan chan<- string, startTime time.Time) (*core.Result, error) {
	r.sendLog(logChan, "🗑️ 开始文件清理...\n")
	r.sendLog(logChan, fmt.Sprintf("📁 目标路径: %s\n", r.config.Path))
	r.sendLog(logChan, fmt.Sprintf("🔍 匹配模式: %s\n", r.config.Pattern))

	if r.config.DryRun {
		r.sendLog(logChan, "⚠️ 试运行模式：只分析，不实际删除\n")
	}

	var deletedFiles []string
	var totalSize int64
	var err error

	// 判断本地/远程
	if r.config.Host != "" {
		// 远程操作
		deletedFiles, totalSize, err = r.executeCleanupRemote(ctx, logChan)
	} else {
		// 本地操作
		deletedFiles, totalSize, err = r.executeCleanupLocal(ctx, logChan)
	}

	if err != nil {
		return nil, err
	}

	endTime := time.Now()

	// 构建 Output（JSON 格式）
	outputData := map[string]interface{}{
		"action":        "cleanup",
		"path":          r.config.Path,
		"pattern":       r.config.Pattern,
		"dry_run":       r.config.DryRun,
		"deleted_count": len(deletedFiles),
		"deleted_size":  formatSize(totalSize),
		"deleted_files": deletedFiles,
		"duration_ms":   endTime.Sub(startTime).Milliseconds(),
	}

	if r.config.Host != "" {
		outputData["host"] = r.config.Host
	}

	if r.config.DryRun {
		outputData["message"] = "试运行模式：未实际删除文件"
	}

	outputJSON, _ := json.Marshal(outputData)

	r.sendLog(logChan, fmt.Sprintf("✅ 清理完成：%d 个文件，%s\n",
		len(deletedFiles), formatSize(totalSize)))

	successMsg := fmt.Sprintf("成功清理 %d 个文件", len(deletedFiles))
	return &core.Result{
		Status:     core.StatusSuccess,
		ExecuteLog: successMsg,
		Output:     string(outputJSON),
		StartTime:  startTime,
		EndTime:    endTime,
		Duration:   endTime.Sub(startTime).Milliseconds(),
	}, nil
}

// executeCleanupLocal 本地清理
func (r *FileRunner) executeCleanupLocal(ctx context.Context, logChan chan<- string) ([]string, int64, error) {
	// 1. 扫描匹配的文件
	var allFiles []string
	var totalSize int64

	// 使用 filepath.Walk 遍历目录
	err := filepath.Walk(r.config.Path, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// 检查上下文取消
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		// 跳过目录（除非 Recursive 为 false 且是子目录）
		if info.IsDir() {
			if !r.config.Recursive && path != r.config.Path {
				return filepath.SkipDir
			}
			return nil
		}

		// 匹配文件名
		matched, err := filepath.Match(r.config.Pattern, filepath.Base(path))
		if err != nil {
			return err
		}
		if !matched {
			return nil
		}

		// 检查排除路径
		for _, exclude := range r.config.Exclude {
			if strings.Contains(path, exclude) {
				return nil
			}
		}

		// 检查时间条件
		if r.config.OlderThan != "" {
			olderThan, err := parseOlderThan(r.config.OlderThan)
			if err != nil {
				return err
			}
			if time.Since(info.ModTime()) < olderThan {
				return nil // 不够老，跳过
			}
		}

		// 检查大小条件
		if r.config.LargerThan != "" {
			largerThan, err := parseLargerThan(r.config.LargerThan)
			if err != nil {
				return err
			}
			if info.Size() < largerThan {
				return nil // 不够大，跳过
			}
		}

		// 符合条件的文件
		allFiles = append(allFiles, path)
		totalSize += info.Size()

		return nil
	})

	if err != nil {
		return nil, 0, fmt.Errorf("扫描文件失败: %w", err)
	}

	r.sendLog(logChan, fmt.Sprintf("📊 找到 %d 个符合条件的文件，共 %s\n",
		len(allFiles), formatSize(totalSize)))

	// 2. 执行删除（或试运行）
	var deletedFiles []string
	if r.config.DryRun {
		// 试运行：只记录，不删除
		for _, file := range allFiles {
			info, _ := os.Stat(file)
			size := int64(0)
			if info != nil {
				size = info.Size()
			}
			deletedFiles = append(deletedFiles,
				fmt.Sprintf("%s (%s)", file, formatSize(size)))
		}
		r.sendLog(logChan, "⚠️ 试运行模式：以上文件将被删除（实际未删除）\n")
	} else {
		// 实际删除
		for _, file := range allFiles {
			info, _ := os.Stat(file)
			size := int64(0)
			if info != nil {
				size = info.Size()
			}

			if err := os.Remove(file); err != nil {
				r.sendLog(logChan, fmt.Sprintf("⚠️ 删除失败: %s (%v)\n", file, err))
			} else {
				r.sendLog(logChan, fmt.Sprintf("❌ 已删除: %s (%s)\n",
					file, formatSize(size)))
				deletedFiles = append(deletedFiles,
					fmt.Sprintf("%s (%s)", file, formatSize(size)))
			}
		}
	}

	return deletedFiles, totalSize, nil
}

// ============================================================================
// backup 操作实现
// ============================================================================

// executeBackup 执行备份操作
func (r *FileRunner) executeBackup(ctx context.Context, logChan chan<- string, startTime time.Time) (*core.Result, error) {
	r.sendLog(logChan, "💾 开始文件备份...\n")
	r.sendLog(logChan, fmt.Sprintf("📁 源路径: %s\n", r.config.Source))
	r.sendLog(logChan, fmt.Sprintf("📁 目标路径: %s\n", r.config.Target))

	var copiedCount int
	var copiedSize int64
	var backupFile string
	var err error

	// 判断本地/远程
	if r.config.Host != "" {
		// 远程操作
		copiedCount, copiedSize, backupFile, err = r.executeBackupRemote(ctx, logChan)
	} else {
		// 本地操作
		copiedCount, copiedSize, backupFile, err = r.executeBackupLocal(ctx, logChan)
	}

	if err != nil {
		return nil, err
	}

	endTime := time.Now()

	// 构建 Output（JSON 格式）
	outputData := map[string]interface{}{
		"action":       "backup",
		"source":       r.config.Source,
		"target":       r.config.Target,
		"copied_count": copiedCount,
		"copied_size":  formatSize(copiedSize),
		"compressed":   r.config.Compress,
		"backup_file":  backupFile,
		"duration_ms":  endTime.Sub(startTime).Milliseconds(),
	}

	if r.config.Host != "" {
		outputData["host"] = r.config.Host
	}

	outputJSON, _ := json.Marshal(outputData)

	r.sendLog(logChan, fmt.Sprintf("✅ 备份完成：%d 个文件，%s\n",
		copiedCount, formatSize(copiedSize)))
	r.sendLog(logChan, fmt.Sprintf("📄 备份文件: %s\n", backupFile))

	successMsg := fmt.Sprintf("成功备份 %d 个文件到 %s", copiedCount, backupFile)
	return &core.Result{
		Status:     core.StatusSuccess,
		ExecuteLog: successMsg,
		Output:     string(outputJSON),
		StartTime:  startTime,
		EndTime:    endTime,
		Duration:   endTime.Sub(startTime).Milliseconds(),
	}, nil
}

// executeBackupLocal 本地备份
func (r *FileRunner) executeBackupLocal(ctx context.Context, logChan chan<- string) (int, int64, string, error) {
	// 确定目标文件名（添加时间戳）
	targetPath := r.addTimestampToPath(r.config.Target, r.config.Compress)

	r.sendLog(logChan, fmt.Sprintf("📝 生成备份文件名: %s\n", filepath.Base(targetPath)))

	// 创建目标目录
	targetDir := filepath.Dir(targetPath)
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return 0, 0, "", fmt.Errorf("创建目标目录失败: %w", err)
	}

	var copiedCount int
	var copiedSize int64

	if r.config.Compress {
		// 压缩备份
		r.sendLog(logChan, "📦 压缩备份模式\n")
		r.sendLog(logChan, fmt.Sprintf("📄 目标文件: %s\n", targetPath))
		count, size, err := r.compressDirectory(r.config.Source, targetPath, logChan)
		if err != nil {
			return 0, 0, "", err
		}
		copiedCount = count
		copiedSize = size
	} else {
		// 直接复制
		r.sendLog(logChan, "📋 直接复制模式\n")
		r.sendLog(logChan, fmt.Sprintf("📂 目标目录: %s\n", targetPath))
		count, size, err := r.copyDirectory(r.config.Source, targetPath, logChan)
		if err != nil {
			return 0, 0, "", err
		}
		copiedCount = count
		copiedSize = size
	}

	return copiedCount, copiedSize, targetPath, nil
}

// executeBackupRemote 远程备份
func (r *FileRunner) executeBackupRemote(ctx context.Context, logChan chan<- string) (int, int64, string, error) {
	// 确定目标文件名（添加时间戳）
	targetPath := r.addTimestampToPath(r.config.Target, r.config.Compress)

	r.sendLog(logChan, fmt.Sprintf("📝 生成备份文件名: %s\n", filepath.Base(targetPath)))

	// 创建目标目录
	targetDir := filepath.Dir(targetPath)
	mkdirCmd := fmt.Sprintf("mkdir -p %s", r.shellQuote(targetDir))
	if _, err := r.execCommand(mkdirCmd); err != nil {
		return 0, 0, "", fmt.Errorf("创建目标目录失败: %w", err)
	}

	var cmd string
	if r.config.Compress {
		// 使用 tar 压缩备份
		r.sendLog(logChan, "📦 远程压缩备份\n")
		cmd = fmt.Sprintf("tar -czf %s -C %s .",
			r.shellQuote(targetPath),
			r.shellQuote(r.config.Source))
	} else {
		// 使用 cp 直接复制
		r.sendLog(logChan, "📋 远程直接复制\n")
		cmd = fmt.Sprintf("cp -r %s %s",
			r.shellQuote(r.config.Source),
			r.shellQuote(targetPath))
	}

	r.sendLog(logChan, fmt.Sprintf("🔧 执行命令: %s\n", cmd))
	if _, err := r.execCommand(cmd); err != nil {
		return 0, 0, "", fmt.Errorf("备份失败: %w", err)
	}

	// 统计文件数量和大小
	countCmd := fmt.Sprintf("find %s -type f | wc -l", r.shellQuote(r.config.Source))
	countOutput, _ := r.execCommand(countCmd)
	fileCount := 0
	fmt.Sscanf(strings.TrimSpace(countOutput), "%d", &fileCount)

	sizeCmd := fmt.Sprintf("du -sb %s | awk '{print $1}'", r.shellQuote(targetPath))
	sizeOutput, _ := r.execCommand(sizeCmd)
	var totalSize int64
	fmt.Sscanf(strings.TrimSpace(sizeOutput), "%d", &totalSize)

	r.sendLog(logChan, "✅ 远程备份完成\n")
	return fileCount, totalSize, targetPath, nil
}

// ============================================================================
// compress 操作实现
// ============================================================================

// executeCompress 执行压缩操作
func (r *FileRunner) executeCompress(ctx context.Context, logChan chan<- string, startTime time.Time) (*core.Result, error) {
	r.sendLog(logChan, "📦 开始文件压缩...\n")
	r.sendLog(logChan, fmt.Sprintf("📁 源路径: %s\n", r.config.Source))

	// 确定目标文件名
	targetPath := r.config.Target
	if targetPath == "" {
		targetPath = r.config.Source + ".tar.gz"
	}
	if !strings.HasSuffix(targetPath, ".tar.gz") && !strings.HasSuffix(targetPath, ".zip") {
		targetPath = targetPath + ".tar.gz"
	}

	r.sendLog(logChan, fmt.Sprintf("📁 目标文件: %s\n", targetPath))

	var originalSize int64
	var compressedSize int64
	var fileCount int
	var err error

	// 判断本地/远程
	if r.config.Host != "" {
		// 远程操作
		originalSize, compressedSize, fileCount, err = r.executeCompressRemote(ctx, logChan, targetPath)
	} else {
		// 本地操作
		originalSize, compressedSize, fileCount, err = r.executeCompressLocal(ctx, logChan, targetPath)
	}

	if err != nil {
		return nil, err
	}

	endTime := time.Now()

	// 计算压缩率
	compressionRatio := 0.0
	if originalSize > 0 {
		compressionRatio = float64(originalSize-compressedSize) / float64(originalSize) * 100
	}

	// 构建 Output（JSON 格式）
	outputData := map[string]interface{}{
		"action":            "compress",
		"source":            r.config.Source,
		"target":            targetPath,
		"original_size":     formatSize(originalSize),
		"compressed_size":   formatSize(compressedSize),
		"compression_ratio": fmt.Sprintf("%.1f%%", compressionRatio),
		"file_count":        fileCount,
		"duration_ms":       endTime.Sub(startTime).Milliseconds(),
	}

	if r.config.Host != "" {
		outputData["host"] = r.config.Host
	}

	outputJSON, _ := json.Marshal(outputData)

	r.sendLog(logChan, fmt.Sprintf("✅ 压缩完成：%d 个文件，%s → %s（压缩率: %.1f%%）\n",
		fileCount, formatSize(originalSize), formatSize(compressedSize), compressionRatio))

	successMsg := fmt.Sprintf("成功压缩 %d 个文件", fileCount)
	return &core.Result{
		Status:     core.StatusSuccess,
		ExecuteLog: successMsg,
		Output:     string(outputJSON),
		StartTime:  startTime,
		EndTime:    endTime,
		Duration:   endTime.Sub(startTime).Milliseconds(),
	}, nil
}

// executeCompressLocal 本地压缩
func (r *FileRunner) executeCompressLocal(ctx context.Context, logChan chan<- string, targetPath string) (int64, int64, int, error) {
	// 计算原始大小
	var originalSize int64
	var fileCount int

	err := filepath.Walk(r.config.Source, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			originalSize += info.Size()
			fileCount++
		}
		return nil
	})
	if err != nil {
		return 0, 0, 0, fmt.Errorf("扫描源文件失败: %w", err)
	}

	r.sendLog(logChan, fmt.Sprintf("📊 原始大小: %s（%d 个文件）\n",
		formatSize(originalSize), fileCount))

	// 压缩
	count, size, err := r.compressDirectory(r.config.Source, targetPath, logChan)
	if err != nil {
		return 0, 0, 0, err
	}

	// 是否删除源文件
	if r.config.RemoveSource {
		r.sendLog(logChan, "🗑️ 删除源文件...\n")
		if err := os.RemoveAll(r.config.Source); err != nil {
			r.sendLog(logChan, fmt.Sprintf("⚠️ 删除源文件失败: %v\n", err))
		} else {
			r.sendLog(logChan, "✅ 源文件已删除\n")
		}
	}

	return originalSize, size, count, nil
}

// executeCompressRemote 远程压缩
func (r *FileRunner) executeCompressRemote(ctx context.Context, logChan chan<- string, targetPath string) (int64, int64, int, error) {
	// 计算原始大小
	sizeCmd := fmt.Sprintf("du -sb %s | awk '{print $1}'", r.shellQuote(r.config.Source))
	sizeOutput, err := r.execCommand(sizeCmd)
	if err != nil {
		return 0, 0, 0, fmt.Errorf("计算原始大小失败: %w", err)
	}
	var originalSize int64
	fmt.Sscanf(strings.TrimSpace(sizeOutput), "%d", &originalSize)

	// 统计文件数
	countCmd := fmt.Sprintf("find %s -type f | wc -l", r.shellQuote(r.config.Source))
	countOutput, _ := r.execCommand(countCmd)
	fileCount := 0
	fmt.Sscanf(strings.TrimSpace(countOutput), "%d", &fileCount)

	r.sendLog(logChan, fmt.Sprintf("📊 原始大小: %s（%d 个文件）\n",
		formatSize(originalSize), fileCount))

	// 压缩
	r.sendLog(logChan, "📦 执行压缩...\n")
	compressCmd := fmt.Sprintf("tar -czf %s -C %s .",
		r.shellQuote(targetPath),
		r.shellQuote(r.config.Source))

	if _, err := r.execCommand(compressCmd); err != nil {
		return 0, 0, 0, fmt.Errorf("压缩失败: %w", err)
	}

	// 获取压缩后大小
	compressedSizeCmd := fmt.Sprintf("du -sb %s | awk '{print $1}'", r.shellQuote(targetPath))
	compressedOutput, _ := r.execCommand(compressedSizeCmd)
	var compressedSize int64
	fmt.Sscanf(strings.TrimSpace(compressedOutput), "%d", &compressedSize)

	// 是否删除源文件
	if r.config.RemoveSource {
		r.sendLog(logChan, "🗑️ 删除源文件...\n")
		removeCmd := fmt.Sprintf("rm -rf %s", r.shellQuote(r.config.Source))
		if _, err := r.execCommand(removeCmd); err != nil {
			r.sendLog(logChan, fmt.Sprintf("⚠️ 删除源文件失败: %v\n", err))
		} else {
			r.sendLog(logChan, "✅ 源文件已删除\n")
		}
	}

	return originalSize, compressedSize, fileCount, nil
}

// ============================================================================
// stat 操作实现
// ============================================================================

// executeStat 执行统计操作
func (r *FileRunner) executeStat(ctx context.Context, logChan chan<- string, startTime time.Time) (*core.Result, error) {
	r.sendLog(logChan, "📊 开始文件统计...\n")
	r.sendLog(logChan, fmt.Sprintf("📁 目标路径: %s\n", r.config.Path))

	var totalSize int64
	var totalFiles int
	var totalDirs int
	var topFiles []map[string]interface{}
	var err error

	// 判断本地/远程
	if r.config.Host != "" {
		// 远程操作
		totalSize, totalFiles, totalDirs, topFiles, err = r.executeStatRemote(ctx, logChan)
	} else {
		// 本地操作
		totalSize, totalFiles, totalDirs, topFiles, err = r.executeStatLocal(ctx, logChan)
	}

	if err != nil {
		return nil, err
	}

	endTime := time.Now()

	// 构建 Output（JSON 格式）
	outputData := map[string]interface{}{
		"action":      "stat",
		"path":        r.config.Path,
		"total_size":  formatSize(totalSize),
		"total_files": totalFiles,
		"total_dirs":  totalDirs,
		"top_files":   topFiles,
		"duration_ms": endTime.Sub(startTime).Milliseconds(),
	}

	if r.config.Host != "" {
		outputData["host"] = r.config.Host
	}

	outputJSON, _ := json.Marshal(outputData)

	r.sendLog(logChan, fmt.Sprintf("✅ 统计完成：%d 个文件，%d 个目录，总大小 %s\n",
		totalFiles, totalDirs, formatSize(totalSize)))

	successMsg := fmt.Sprintf("统计完成：%d 个文件，总大小 %s", totalFiles, formatSize(totalSize))
	return &core.Result{
		Status:     core.StatusSuccess,
		ExecuteLog: successMsg,
		Output:     string(outputJSON),
		StartTime:  startTime,
		EndTime:    endTime,
		Duration:   endTime.Sub(startTime).Milliseconds(),
	}, nil
}

// executeStatLocal 本地统计
func (r *FileRunner) executeStatLocal(ctx context.Context, logChan chan<- string) (int64, int, int, []map[string]interface{}, error) {
	type FileInfoStat struct {
		Path    string
		Size    int64
		ModTime time.Time
	}

	var totalSize int64
	var totalFiles int
	var totalDirs int
	var allFiles []FileInfoStat

	// 遍历目录
	err := filepath.Walk(r.config.Path, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			totalDirs++
		} else {
			totalFiles++
			totalSize += info.Size()
			allFiles = append(allFiles, FileInfoStat{
				Path:    path,
				Size:    info.Size(),
				ModTime: info.ModTime(),
			})
		}
		return nil
	})

	if err != nil {
		return 0, 0, 0, nil, fmt.Errorf("统计失败: %w", err)
	}

	r.sendLog(logChan, fmt.Sprintf("📊 找到 %d 个文件，%d 个目录\n", totalFiles, totalDirs))

	// 排序并获取 Top N
	limit := r.config.Limit
	if limit == 0 {
		limit = 10
	}

	// 按大小排序（目前只支持按大小）
	sortBy := r.config.SortBy
	if sortBy == "" {
		sortBy = "size"
	}

	// 简单的冒泡排序（按大小降序）
	for i := 0; i < len(allFiles)-1; i++ {
		for j := 0; j < len(allFiles)-i-1; j++ {
			if allFiles[j].Size < allFiles[j+1].Size {
				allFiles[j], allFiles[j+1] = allFiles[j+1], allFiles[j]
			}
		}
	}

	// 取 Top N
	topFiles := make([]map[string]interface{}, 0, limit)
	for i := 0; i < len(allFiles) && i < limit; i++ {
		topFiles = append(topFiles, map[string]interface{}{
			"path":     allFiles[i].Path,
			"size":     formatSize(allFiles[i].Size),
			"modified": allFiles[i].ModTime.Format("2006-01-02 15:04:05"),
		})
	}

	return totalSize, totalFiles, totalDirs, topFiles, nil
}

// executeStatRemote 远程统计
func (r *FileRunner) executeStatRemote(ctx context.Context, logChan chan<- string) (int64, int, int, []map[string]interface{}, error) {
	// 统计总大小
	sizeCmd := fmt.Sprintf("du -sb %s | awk '{print $1}'", r.shellQuote(r.config.Path))
	sizeOutput, err := r.execCommand(sizeCmd)
	if err != nil {
		return 0, 0, 0, nil, fmt.Errorf("统计大小失败: %w", err)
	}
	var totalSize int64
	fmt.Sscanf(strings.TrimSpace(sizeOutput), "%d", &totalSize)

	// 统计文件数
	fileCountCmd := fmt.Sprintf("find %s -type f | wc -l", r.shellQuote(r.config.Path))
	fileCountOutput, _ := r.execCommand(fileCountCmd)
	totalFiles := 0
	fmt.Sscanf(strings.TrimSpace(fileCountOutput), "%d", &totalFiles)

	// 统计目录数
	dirCountCmd := fmt.Sprintf("find %s -type d | wc -l", r.shellQuote(r.config.Path))
	dirCountOutput, _ := r.execCommand(dirCountCmd)
	totalDirs := 0
	fmt.Sscanf(strings.TrimSpace(dirCountOutput), "%d", &totalDirs)

	r.sendLog(logChan, fmt.Sprintf("📊 找到 %d 个文件，%d 个目录\n", totalFiles, totalDirs))

	// 获取最大的 N 个文件
	limit := r.config.Limit
	if limit == 0 {
		limit = 10
	}

	topFilesCmd := fmt.Sprintf("find %s -type f -exec du -b {} + | sort -rn | head -%d",
		r.shellQuote(r.config.Path), limit)
	topFilesOutput, _ := r.execCommand(topFilesCmd)

	// 解析 top files
	topFiles := make([]map[string]interface{}, 0, limit)
	if topFilesOutput != "" {
		lines := strings.Split(strings.TrimSpace(topFilesOutput), "\n")
		for _, line := range lines {
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				var size int64
				fmt.Sscanf(parts[0], "%d", &size)
				topFiles = append(topFiles, map[string]interface{}{
					"path": strings.Join(parts[1:], " "),
					"size": formatSize(size),
				})
			}
		}
	}

	return totalSize, totalFiles, totalDirs, topFiles, nil
}

// ============================================================================
// SSH 连接管理（远程操作）
// ============================================================================

// getAndValidateCredential 获取并验证凭证
func (r *FileRunner) getAndValidateCredential(logChan chan<- string) (*core.Credential, error) {
	// 1. 检查 apiserver 是否已注入
	if r.apiserver == nil {
		err := fmt.Errorf("apiserver 未初始化，无法获取凭证")
		r.sendLog(logChan, fmt.Sprintf("❌ %v\n", err))
		return nil, err
	}

	// 2. 获取凭证
	r.sendLog(logChan, fmt.Sprintf("🔐 获取 SSH 凭证...\n"))
	cred, err := r.apiserver.GetCredential(r.config.Credential)
	if err != nil {
		r.sendLog(logChan, fmt.Sprintf("❌ 获取凭证失败: %v\n", err))
		return nil, err
	}
	r.sendLog(logChan, fmt.Sprintf("✅ 成功获取凭证: %s\n", cred.Name))

	// 3. 验证凭证类型（支持两种）
	supportedTypes := map[string]bool{
		"ssh_private_key":   true,
		"username_password": true,
	}
	if !supportedTypes[cred.Category] {
		err := fmt.Errorf("凭证类型不支持：期望 ssh_private_key 或 username_password，实际 %s", cred.Category)
		r.sendLog(logChan, fmt.Sprintf("❌ %v\n", err))
		return nil, err
	}

	return cred, nil
}

// connectSSH 建立 SSH 连接
func (r *FileRunner) connectSSH(ctx context.Context, logChan chan<- string) error {
	// 1. 获取凭证
	cred, err := r.getAndValidateCredential(logChan)
	if err != nil {
		return fmt.Errorf("获取凭证失败: %w", err)
	}

	// 2. 准备 SSH 配置
	var authMethod ssh.AuthMethod

	switch cred.Category {
	case "ssh_private_key":
		// SSH 密钥认证（推荐）
		privateKey, ok := cred.GetString("private_key")
		if !ok || privateKey == "" {
			err := fmt.Errorf("凭证缺少 private_key 字段")
			r.sendLog(logChan, fmt.Sprintf("❌ %v\n", err))
			return err
		}

		signer, err := ssh.ParsePrivateKey([]byte(privateKey))
		if err != nil {
			r.sendLog(logChan, fmt.Sprintf("❌ 解析私钥失败: %v\n", err))
			return fmt.Errorf("解析私钥失败: %w", err)
		}
		authMethod = ssh.PublicKeys(signer)
		r.sendLog(logChan, "🔑 使用 SSH 密钥认证\n")

	case "username_password":
		// 用户名密码认证
		password, ok := cred.GetString("password")
		if !ok {
			err := fmt.Errorf("凭证缺少 password 字段")
			r.sendLog(logChan, fmt.Sprintf("❌ %v\n", err))
			return err
		}
		authMethod = ssh.Password(password)
		r.sendLog(logChan, "🔐 使用密码认证\n")

	default:
		return fmt.Errorf("不支持的凭证类型: %s", cred.Category)
	}

	// 3. SSH 客户端配置
	config := &ssh.ClientConfig{
		User:            r.config.Username,
		Auth:            []ssh.AuthMethod{authMethod},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(), // 生产环境建议验证 HostKey
		Timeout:         30 * time.Second,
	}

	// 4. 建立 SSH 连接
	addr := fmt.Sprintf("%s:%d", r.config.Host, r.config.Port)
	r.sendLog(logChan, fmt.Sprintf("🔗 连接 SSH: %s\n", addr))

	sshClient, err := ssh.Dial("tcp", addr, config)
	if err != nil {
		r.sendLog(logChan, fmt.Sprintf("❌ SSH 连接失败: %v\n", err))
		return fmt.Errorf("SSH 连接失败: %w", err)
	}

	r.sshClient = sshClient
	r.sendLog(logChan, "✅ SSH 连接建立成功（纯命令模式，无需 SFTP）\n")

	return nil
}

// closeSSH 关闭 SSH 连接
func (r *FileRunner) closeSSH() {
	if r.sshClient != nil {
		r.sshClient.Close()
		r.sshClient = nil
	}
}

// execCommand 执行远程命令（核心方法）
func (r *FileRunner) execCommand(cmd string) (string, error) {
	if r.sshClient == nil {
		return "", fmt.Errorf("SSH 未连接")
	}

	session, err := r.sshClient.NewSession()
	if err != nil {
		return "", fmt.Errorf("创建 SSH 会话失败: %w", err)
	}
	defer session.Close()

	output, err := session.CombinedOutput(cmd)
	return string(output), err
}

// executeCleanupRemote 远程清理
func (r *FileRunner) executeCleanupRemote(ctx context.Context, logChan chan<- string) ([]string, int64, error) {
	// 1. 构建 find 命令查找文件
	findCmd := r.buildFindCommand()
	r.sendLog(logChan, fmt.Sprintf("🔍 查找命令: %s\n", findCmd))

	// 2. 执行查找
	output, err := r.execCommand(findCmd)
	if err != nil {
		// find 命令可能因为没有匹配文件而返回非零退出码，这是正常的
		if output == "" {
			r.sendLog(logChan, "📊 未找到符合条件的文件\n")
			return []string{}, 0, nil
		}
		// 如果有输出但返回错误，继续处理（部分文件可能有权限问题）
		r.sendLog(logChan, fmt.Sprintf("⚠️ 查找过程有警告: %v\n", err))
	}

	// 3. 解析文件列表
	output = strings.TrimSpace(output)
	if output == "" {
		r.sendLog(logChan, "📊 未找到符合条件的文件\n")
		return []string{}, 0, nil
	}

	files := strings.Split(output, "\n")
	r.sendLog(logChan, fmt.Sprintf("📊 找到 %d 个符合条件的文件\n", len(files)))

	// 4. 计算总大小（使用 du 命令）
	var totalSize int64
	if len(files) > 0 {
		// 批量计算大小
		duCmd := fmt.Sprintf("du -cb %s 2>/dev/null | tail -1 | awk '{print $1}'",
			strings.Join(files, " "))
		sizeOutput, err := r.execCommand(duCmd)
		if err == nil {
			sizeOutput = strings.TrimSpace(sizeOutput)
			if sizeOutput != "" {
				fmt.Sscanf(sizeOutput, "%d", &totalSize)
			}
		}
	}

	r.sendLog(logChan, fmt.Sprintf("💾 总大小: %s\n", formatSize(totalSize)))

	// 5. 试运行模式
	if r.config.DryRun {
		r.sendLog(logChan, "⚠️ 试运行模式：以下文件将被删除（实际未删除）\n")
		deletedFiles := make([]string, 0, len(files))
		for i, file := range files {
			if i < 10 { // 只显示前 10 个
				r.sendLog(logChan, fmt.Sprintf("  - %s\n", file))
			}
			deletedFiles = append(deletedFiles, file)
		}
		if len(files) > 10 {
			r.sendLog(logChan, fmt.Sprintf("  ... 还有 %d 个文件\n", len(files)-10))
		}
		return deletedFiles, totalSize, nil
	}

	// 6. 实际删除（批量删除，效率高）
	if len(files) > 0 {
		r.sendLog(logChan, fmt.Sprintf("🗑️ 删除 %d 个文件...\n", len(files)))

		// 使用 xargs 批量删除，更安全高效
		// 将文件列表传递给 xargs，避免命令行参数过长
		deleteCmd := fmt.Sprintf("printf '%%s\\n' %s | xargs -r rm -f",
			r.shellEscape(files...))

		_, err := r.execCommand(deleteCmd)
		if err != nil {
			r.sendLog(logChan, fmt.Sprintf("⚠️ 删除过程有错误: %v\n", err))
			// 继续处理，返回文件列表
		} else {
			r.sendLog(logChan, "✅ 删除完成\n")
		}
	}

	return files, totalSize, nil
}

// buildFindCommand 构建 find 命令
func (r *FileRunner) buildFindCommand() string {
	// 基础命令
	cmd := fmt.Sprintf("find %s -name '%s' -type f",
		r.shellQuote(r.config.Path),
		r.config.Pattern)

	// 递归控制
	if !r.config.Recursive {
		cmd += " -maxdepth 1"
	}

	// 添加时间条件
	if r.config.OlderThan != "" {
		if days := r.parseOlderThanDays(r.config.OlderThan); days > 0 {
			cmd += fmt.Sprintf(" -mtime +%d", days)
		}
	}

	// 添加大小条件
	if r.config.LargerThan != "" {
		if sizeStr := r.parseLargerThanStr(r.config.LargerThan); sizeStr != "" {
			cmd += fmt.Sprintf(" -size +%s", sizeStr)
		}
	}

	// 添加排除条件
	for _, exclude := range r.config.Exclude {
		cmd += fmt.Sprintf(" ! -path '*%s*'", exclude)
	}

	return cmd
}

// parseOlderThanDays 解析时间条件为天数
func (r *FileRunner) parseOlderThanDays(s string) int {
	duration, err := parseOlderThan(s)
	if err != nil {
		return 0
	}
	return int(duration.Hours() / 24)
}

// parseLargerThanStr 解析大小条件为 find 命令格式
func (r *FileRunner) parseLargerThanStr(s string) string {
	s = strings.TrimSpace(strings.ToUpper(s))
	if len(s) < 2 {
		return ""
	}

	unit := s[len(s)-1:]
	valueStr := s[:len(s)-1]

	// find 命令的大小单位：c=bytes, k=KB, M=MB, G=GB
	unitMap := map[string]string{
		"K": "k",
		"M": "M",
		"G": "G",
	}

	if findUnit, ok := unitMap[unit]; ok {
		return valueStr + findUnit
	}

	return ""
}

// shellQuote 为 shell 命令转义路径
func (r *FileRunner) shellQuote(s string) string {
	// 简单的单引号转义
	return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'"
}

// shellEscape 转义多个参数
func (r *FileRunner) shellEscape(args ...string) string {
	escaped := make([]string, len(args))
	for i, arg := range args {
		escaped[i] = r.shellQuote(arg)
	}
	return strings.Join(escaped, " ")
}

// ============================================================================
// 工具函数
// ============================================================================

// parseOlderThan 解析时间条件（7d -> 7天）
func parseOlderThan(s string) (time.Duration, error) {
	s = strings.TrimSpace(s)
	if len(s) < 2 {
		return 0, fmt.Errorf("无效的时间格式: %s（示例: 7d, 30d）", s)
	}

	unit := s[len(s)-1:]
	valueStr := s[:len(s)-1]

	var value int
	if _, err := fmt.Sscanf(valueStr, "%d", &value); err != nil {
		return 0, fmt.Errorf("无效的时间数值: %s", valueStr)
	}

	switch unit {
	case "d":
		return time.Duration(value) * 24 * time.Hour, nil
	case "h":
		return time.Duration(value) * time.Hour, nil
	case "m":
		return time.Duration(value) * time.Minute, nil
	default:
		return 0, fmt.Errorf("无效的时间单位: %s（支持: d=天, h=小时, m=分钟）", unit)
	}
}

// parseLargerThan 解析大小条件（100M -> 100MB）
func parseLargerThan(s string) (int64, error) {
	s = strings.TrimSpace(strings.ToUpper(s))
	if len(s) < 2 {
		return 0, fmt.Errorf("无效的大小格式: %s（示例: 100M, 1G）", s)
	}

	unit := s[len(s)-1:]
	valueStr := s[:len(s)-1]

	var value int64
	if _, err := fmt.Sscanf(valueStr, "%d", &value); err != nil {
		return 0, fmt.Errorf("无效的大小数值: %s", valueStr)
	}

	switch unit {
	case "K":
		return value * 1024, nil
	case "M":
		return value * 1024 * 1024, nil
	case "G":
		return value * 1024 * 1024 * 1024, nil
	default:
		return 0, fmt.Errorf("无效的大小单位: %s（支持: K, M, G）", unit)
	}
}

// formatSize 格式化文件大小
func formatSize(size int64) string {
	const unit = 1024
	if size < unit {
		return fmt.Sprintf("%d B", size)
	}
	div, exp := int64(unit), 0
	for n := size / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(size)/float64(div), "KMGTPE"[exp])
}

// copyFile 复制文件
func copyFile(src, dst string) error {
	sourceFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	destFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer destFile.Close()

	if _, err := io.Copy(destFile, sourceFile); err != nil {
		return err
	}

	// 复制文件权限
	sourceInfo, err := os.Stat(src)
	if err != nil {
		return err
	}
	return os.Chmod(dst, sourceInfo.Mode())
}

// copyDirectory 复制目录
func (r *FileRunner) copyDirectory(src, dst string, logChan chan<- string) (int, int64, error) {
	var count int
	var totalSize int64

	// 获取源目录信息
	srcInfo, err := os.Stat(src)
	if err != nil {
		return 0, 0, fmt.Errorf("获取源目录信息失败: %w", err)
	}

	// 创建目标目录
	if err := os.MkdirAll(dst, srcInfo.Mode()); err != nil {
		return 0, 0, fmt.Errorf("创建目标目录失败: %w", err)
	}

	// 遍历源目录
	err = filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// 计算相对路径
		relPath, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}

		// 目标路径
		targetPath := filepath.Join(dst, relPath)

		if info.IsDir() {
			// 创建目录
			return os.MkdirAll(targetPath, info.Mode())
		}

		// 复制文件
		if err := copyFile(path, targetPath); err != nil {
			r.sendLog(logChan, fmt.Sprintf("⚠️ 复制失败: %s -> %s (%v)\n", path, targetPath, err))
			return nil // 继续处理其他文件
		}

		count++
		totalSize += info.Size()

		return nil
	})

	if err != nil {
		return 0, 0, fmt.Errorf("复制目录失败: %w", err)
	}

	return count, totalSize, nil
}

// compressDirectory 压缩目录为 tar.gz
func (r *FileRunner) compressDirectory(src, dst string, logChan chan<- string) (int, int64, error) {
	// 创建目标文件
	outFile, err := os.Create(dst)
	if err != nil {
		return 0, 0, fmt.Errorf("创建压缩文件失败: %w", err)
	}
	defer outFile.Close()

	// 创建 gzip writer
	gzipWriter := gzip.NewWriter(outFile)
	defer gzipWriter.Close()

	// 创建 tar writer
	tarWriter := tar.NewWriter(gzipWriter)
	defer tarWriter.Close()

	var count int
	var totalSize int64

	// 遍历源目录
	err = filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// 创建 tar header
		header, err := tar.FileInfoHeader(info, info.Name())
		if err != nil {
			return err
		}

		// 计算相对路径
		relPath, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		header.Name = relPath

		// 写入 header
		if err := tarWriter.WriteHeader(header); err != nil {
			return err
		}

		// 如果是文件，写入内容
		if !info.IsDir() {
			file, err := os.Open(path)
			if err != nil {
				return err
			}
			defer file.Close()

			if _, err := io.Copy(tarWriter, file); err != nil {
				return err
			}

			count++
			totalSize += info.Size()
		}

		return nil
	})

	if err != nil {
		return 0, 0, fmt.Errorf("压缩目录失败: %w", err)
	}

	// 获取压缩后文件大小
	compressedInfo, err := os.Stat(dst)
	if err != nil {
		return count, 0, err
	}

	return count, compressedInfo.Size(), nil
}
