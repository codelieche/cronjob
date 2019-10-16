package handlers

import (
	"log"
	"os"

	"github.com/codelieche/cronjob/backend/worker"

	"github.com/codelieche/cronjob/backend/common"
)

var jobManager *common.JobManager
var workerManager *common.WorkerManager
var logHandler *worker.MongoLogHandler

func init() {
	if jobManagerObj, err := common.NewJobManager(common.Config.Master.Etcd); err != nil {
		log.Println(err.Error())
		os.Exit(1)
	} else {
		jobManager = jobManagerObj
	}

	//	 初始化WorkerManager
	if workerManagerObj, err := common.NewWorkerManager(common.Config.Master.Etcd); err != nil {
		log.Println(err.Error())
		os.Exit(1)
	} else {
		workerManager = workerManagerObj
	}

	//	初始化logHandler
	if logHandlerobj, err := worker.NewMongoLogHandler(common.Config.Master.Mongo); err != nil {
		log.Println(err.Error())
		os.Exit(1)
	} else {
		logHandler = logHandlerobj
	}
}
