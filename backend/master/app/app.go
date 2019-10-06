package app

import (
	"time"

	"github.com/codelieche/cronjob/backend/master/apiserver"
)

type Master struct {
	TimeStart time.Time
	ApiServer *apiserver.ApiServer
}

func (master *Master) Run() {
	master.ApiServer.Run(":9000")
}

func NewMasterApp() *Master {
	apiServer := apiserver.NewApiServer()
	return &Master{
		TimeStart: time.Now(),
		ApiServer: apiServer,
	}
}
