package master

import (
	"fmt"
	"log"
	"time"

	"github.com/codelieche/cronjob/backend/common"

	"github.com/codelieche/cronjob/backend/master/apiserver"
)

type App struct {
	TimeStart time.Time
	ApiServer *apiserver.ApiServer
}

func (app *App) Run() {
	addr := fmt.Sprintf("%s:%d", common.Config.Master.Http.Host, common.Config.Master.Http.Port)
	log.Printf("master api server: http://%s\n", addr)
	app.ApiServer.Run(addr)
}

func NewMasterApp() *App {
	// 解析配置
	apiServer := apiserver.NewApiServer()
	return &App{
		TimeStart: time.Now(),
		ApiServer: apiServer,
	}
}
