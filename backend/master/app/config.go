package app

import "github.com/kataras/iris/v12"

// 配置应用
func appConfigure(app *iris.Application) {
	// 配置应用
	app.Configure(iris.WithConfiguration(iris.Configuration{
		IgnoreServerErrors:                nil,
		DisableStartupLog:                 false,
		DisableInterruptHandler:           false,
		DisablePathCorrection:             false,
		DisablePathCorrectionRedirection:  false,
		EnablePathEscape:                  false,
		EnableOptimizations:               false,
		FireMethodNotAllowed:              false,
		DisableBodyConsumptionOnUnmarshal: false,
		DisableAutoFireStatusCode:         false,
		TimeFormat:                        "2006-01-02 15:04:05",
		Charset:                           "UTF-8",
		PostMaxMemory:                     0,
		ViewLayoutContextKey:              "",
		ViewDataContextKey:                "",
		RemoteAddrHeaders:                 nil,
	}))
}
