package runner

import "github.com/codelieche/cronjob/worker/pkg/core"

// init 注册CommandRunner到默认注册表
func init() {
	// 注册CommandRunner到默认注册表
	core.RegisterRunner("command", func() core.Runner {
		return NewCommandRunner()
	})
	// 默认的Runner类型
	core.RegisterRunner("default", func() core.Runner {
		return NewCommandRunner()
	})
}
