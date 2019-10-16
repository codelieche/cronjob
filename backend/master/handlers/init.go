package handlers

import (
	"log"
	"os"

	"github.com/codelieche/cronjob/backend/worker"

	"github.com/codelieche/cronjob/backend/common"
)

var etcdManager *common.EtcdManager
var workerManager *common.WorkerManager
var logHandler *worker.MongoLogHandler

func init() {
	if obj, err := common.NewEtcdManager(common.Config.Master.Etcd); err != nil {
		log.Println(err.Error())
		os.Exit(1)
	} else {
		etcdManager = obj
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
