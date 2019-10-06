package handlers

import (
	"log"

	"github.com/codelieche/cronjob/backend/common"
)

var jobManager *common.JobManager

func init() {
	if jobManagerObj, err := common.NewJobManager(); err != nil {
		log.Println(err.Error())
		panic(err)
		return
	} else {
		jobManager = jobManagerObj
	}
}
