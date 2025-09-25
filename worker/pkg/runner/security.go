package runner

import (
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/codelieche/cronjob/worker/pkg/utils/logger"
	"go.uber.org/zap"
)

// SecurityConfig 安全配置
type SecurityConfig struct {
	// 是否启用安全检查
	Enabled bool `json:"enabled"`

	// 允许的命令白名单（如果为空则允许所有）
	AllowedCommands []string `json:"allowed_commands"`

	// 禁止的命令黑名单
	BlockedCommands []string `json:"blocked_commands"`

	// 禁止的命令模式（正则表达式）
	BlockedPatterns []string `json:"blocked_patterns"`

	// 禁止的路径模式
	BlockedPaths []string `json:"blocked_paths"`

	// 最大参数长度
	MaxArgsLength int `json:"max_args_length"`
}

// DefaultSecurityConfig 默认安全配置
func DefaultSecurityConfig() *SecurityConfig {
	return &SecurityConfig{
		Enabled: true,
		BlockedCommands: []string{
			"rm", "del", "rd", "rmdir", "format", "fdisk", "mkfs",
			"dd", "shutdown", "reboot", "halt", "poweroff",
			"init", "systemctl", "service", "chmod", "chown",
			"sudo", "su", "passwd", "useradd", "userdel",
			"mount", "umount", "fdisk", "parted",
		},
		BlockedPatterns: []string{
			`rm\s+.*-rf.*/`,         // rm -rf /
			`rm\s+.*-rf.*\*`,        // rm -rf *
			`rm\s+.*-rf.*\.\.`,      // rm -rf ..
			`del\s+.*/s.*/q.*\*`,    // del /s /q *
			`format\s+.*c:`,         // format c:
			`shutdown\s+.*-h\s+now`, // shutdown -h now
			`reboot\s+.*-f`,         // reboot -f
			`dd\s+.*if=.*of=.*`,     // dd if=/dev/zero of=...
			`mkfs\s+.*`,             // mkfs.*
			`fdisk\s+.*`,            // fdisk.*
			`mount\s+.*`,            // mount.*
			`umount\s+.*`,           // umount.*
			`chmod\s+.*777`,         // chmod 777
			`chown\s+.*root`,        // chown root
			`sudo\s+.*`,             // sudo.*
			`su\s+.*`,               // su.*
			`passwd\s+.*`,           // passwd.*
			`useradd\s+.*`,          // useradd.*
			`userdel\s+.*`,          // userdel.*
		},
		BlockedPaths: []string{
			" / ", "/bin", "/sbin", "/usr", "/etc", "/var", "/root",
			"C:\\", "C:\\Windows", "C:\\System32",
		},
		MaxArgsLength: 1000,
	}
}

// LoadSecurityConfig 从环境变量加载安全配置
func LoadSecurityConfig() *SecurityConfig {
	config := DefaultSecurityConfig()

	// 检查是否禁用安全检查
	if os.Getenv("COMMAND_SECURITY_DISABLED") == "true" {
		config.Enabled = false
		return config
	}

	// 从环境变量加载配置
	if allowed := os.Getenv("COMMAND_ALLOWED"); allowed != "" {
		config.AllowedCommands = strings.Split(allowed, ",")
	}

	if blocked := os.Getenv("COMMAND_BLOCKED"); blocked != "" {
		config.BlockedCommands = strings.Split(blocked, ",")
	}

	if patterns := os.Getenv("COMMAND_BLOCKED_PATTERNS"); patterns != "" {
		config.BlockedPatterns = strings.Split(patterns, ",")
	}

	if paths := os.Getenv("COMMAND_BLOCKED_PATHS"); paths != "" {
		config.BlockedPaths = strings.Split(paths, ",")
	}

	return config
}

// CommandSecurity 命令安全检查器
type CommandSecurity struct {
	config *SecurityConfig
}

// NewCommandSecurity 创建命令安全检查器
func NewCommandSecurity() *CommandSecurity {
	return &CommandSecurity{
		config: LoadSecurityConfig(),
	}
}

// ValidateCommand 验证命令是否安全
func (cs *CommandSecurity) ValidateCommand(command string, args []string) error {
	if !cs.config.Enabled {
		return nil
	}

	// 检查命令是否在白名单中
	if len(cs.config.AllowedCommands) > 0 {
		if !cs.isCommandAllowed(command) {
			return fmt.Errorf("命令 '%s' 不在允许列表中", command)
		}
	}

	// 检查命令是否在黑名单中
	if cs.isCommandBlocked(command) {
		return fmt.Errorf("命令 '%s' 被禁止执行", command)
	}

	// 构建完整命令字符串用于模式匹配
	fullCommand := command
	if len(args) > 0 {
		fullCommand += " " + strings.Join(args, " ")
	}

	// 检查是否匹配危险模式
	if cs.matchesBlockedPattern(fullCommand) {
		return fmt.Errorf("命令包含危险模式: %s", fullCommand)
	}

	// 检查参数长度
	if cs.config.MaxArgsLength > 0 && len(fullCommand) > cs.config.MaxArgsLength {
		return fmt.Errorf("命令参数过长: %d > %d", len(fullCommand), cs.config.MaxArgsLength)
	}

	// 检查是否包含危险路径
	if cs.containsBlockedPath(fullCommand) {
		return fmt.Errorf("命令包含危险路径: %s", fullCommand)
	}

	return nil
}

// isCommandAllowed 检查命令是否在允许列表中
func (cs *CommandSecurity) isCommandAllowed(command string) bool {
	for _, allowed := range cs.config.AllowedCommands {
		if command == allowed {
			return true
		}
	}
	return false
}

// isCommandBlocked 检查命令是否在黑名单中
func (cs *CommandSecurity) isCommandBlocked(command string) bool {
	for _, blocked := range cs.config.BlockedCommands {
		if command == blocked {
			return true
		}
	}
	return false
}

// matchesBlockedPattern 检查是否匹配危险模式
func (cs *CommandSecurity) matchesBlockedPattern(command string) bool {
	for _, pattern := range cs.config.BlockedPatterns {
		matched, err := regexp.MatchString(pattern, command)
		if err != nil {
			continue
		}
		if matched {
			return true
		}
	}
	return false
}

// containsBlockedPath 检查是否包含危险路径
func (cs *CommandSecurity) containsBlockedPath(command string) bool {
	for _, path := range cs.config.BlockedPaths {
		if strings.Contains(command, path) {
			logger.Error("命令包含危险路径", zap.String("command", command), zap.String("path", path))
			return true
		}
	}
	return false
}

// 全局安全检查器实例
var globalSecurity *CommandSecurity

// GetGlobalSecurity 获取全局安全检查器实例
func GetGlobalSecurity() *CommandSecurity {
	// 每次都重新加载配置，确保环境变量生效
	return NewCommandSecurity()
}
