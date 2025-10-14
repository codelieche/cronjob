package runner

import "github.com/codelieche/cronjob/worker/pkg/core"

// init 注册所有Runner到默认注册表
func init() {
	// 注册CommandRunner
	core.RegisterRunner("command", func() core.Runner {
		return NewCommandRunner()
	})

	// 默认的Runner类型
	core.RegisterRunner("default", func() core.Runner {
		return NewCommandRunner()
	})

	// 注册HTTPRunner（v2.0 简化版）
	core.RegisterRunner("http", func() core.Runner {
		return NewHTTPRunner()
	})

	// 注册ScriptRunner（v1.0 标准版）
	core.RegisterRunner("script", func() core.Runner {
		return NewScriptRunner()
	})

	// 注册MessageRunner（v1.0 统一消息发送）
	core.RegisterRunner("message", func() core.Runner {
		return NewMessageRunner()
	})
}
