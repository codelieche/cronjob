package worker

import (
	_ "expvar"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/codelieche/cronjob/backend/common"
)

func runMonitorWeb() {

	// router
	router := newWebMonitorRouter()

	//address := fmt.Sprintf(":%d", webMonitorPort)
	address := fmt.Sprintf("%s:%d", common.Config.Worker.Http.Host, common.Config.Worker.Http.Port)
	log.Println("monitor web address:", address)
	if err := http.ListenAndServe(address, router); err != nil {
		log.Println("启动web失败：", err.Error())
		os.Exit(1)
	} else {
		log.Println("monitor web exit")
	}
}
