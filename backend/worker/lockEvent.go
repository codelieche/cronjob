package worker

import (
	"encoding/json"
	"log"

	"github.com/codelieche/cronjob/backend/common/datamodels"
	"github.com/codelieche/cronjob/backend/master/sockets"
)

// 捕获tryLock的相关事件
func tryLockEventHandler(event *sockets.MessageEvent) {

	lockResponse := datamodels.LockResponse{}
	if err := json.Unmarshal([]byte(event.Data), &lockResponse); err != nil {
		log.Println("解析tryLock的响应结果出错：", err)
		return
	} else {
		// 判断是否成功
		//log.Println(lockResponse)
		//if jobLock, isExist := socket.jobLockMap[lockResponse.ID]; isExist {
		//	jobLock.lockResultChan <- &lockResponse
		//} else {
		//	log.Printf("id为%d的jobLock不存在", lockResponse.ID)
		//}
	}

	// log.Println("jobLock Done")
}

func leaseLockEventHandler(event *sockets.MessageEvent) {
	lockResponse := datamodels.LockResponse{}
	if err := json.Unmarshal([]byte(event.Data), &lockResponse); err != nil {
		log.Println("解析releaseLock的响应结果出错：", err)
		return
	} else {
		// 判断是否成功
		log.Println(lockResponse)
		if jobLock, isExist := socket.jobLockMap[lockResponse.ID]; isExist {
			if !lockResponse.Success {
				log.Println("续租失败，需要kill了", jobLock.Name)
				//jobLock.NeedKillChan <- true
			}
		} else {
			log.Printf("id为%d的jobLock不存在", lockResponse.ID)
		}
	}

	// log.Println("jobLock Done")
}
