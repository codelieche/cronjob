package handlers

import (
	"log"
	"os"

	"github.com/codelieche/cronjob/backend/common"
)

var jobManager *common.JobManager
var workerManager *common.WorkerManager

func init() {
	if jobManagerObj, err := common.NewJobManager(); err != nil {
		log.Println(err.Error())
		os.Exit(1)
	} else {
		jobManager = jobManagerObj
	}

	//	 初始化WorkerManager
	if workerManagerObj, err := common.NewWorkerManager(); err != nil {
		log.Println(err.Error())
		os.Exit(1)
	} else {
		workerManager = workerManagerObj
	}
}
