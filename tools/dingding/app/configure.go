package app

import "github.com/kataras/iris"

// 配置应用
func appConfigure(app *iris.Application) {
	// 配置应用
	app.Configure(iris.WithConfiguration(iris.Configuration{
		Tunneling:                         iris.TunnelingConfiguration{},
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
		TranslateFunctionContextKey:       "",
		TranslateLanguageContextKey:       "",
		ViewLayoutContextKey:              "",
		ViewDataContextKey:                "",
		RemoteAddrHeaders:                 nil,
	}))
}
