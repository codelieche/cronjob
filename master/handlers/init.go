package handlers

import "log"

var jobManager *JobManager

func init() {
	if jobManagerObj, err := NewJobManager(); err != nil {
		log.Println(err.Error())
		panic(err)
		return
	} else {
		jobManager = jobManagerObj
	}
}
