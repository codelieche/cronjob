package worker

import (
	"log"
	"testing"
	"time"
)

func TestJobLock_TryLock(t *testing.T) {

	jobLock := NewJobLock("jobs/default/123456")

	if err := jobLock.TryLock(); err != nil {
		t.Errorf(err.Error())
	} else {
		go func() {
			time.Sleep(time.Minute)
			//jobLock.ctxCancelFunc() // 取消续租
			jobLock.Unlock() // 释放锁
		}()
		jobLock.LeaseLoop()
	}

}

func TestJobLock_TryLock02(t *testing.T) {

	jobLock := NewJobLock("jobs/default/123456")

	jobLock.TryLock()

	i := 0
	for i < 10 {
		i++
		if err := jobLock.leaseRequest(); err != nil {
			if i > 1 {
				return
			} else {
				log.Println("续租失败")
				t.Error(err)
			}
		}

		if i == 3 {
			jobLock.ReleaseLock()
		}
		time.Sleep(time.Second * 4)
	}
}

func TestJobLock_LeaseLoop(t *testing.T) {

	jobLock := NewJobLock("jobs/default/123456")

	jobLock.TryLock()

	go func() {
		time.Sleep(time.Second * 30)
		jobLock.closeChan <- true
	}()

	jobLock.LeaseLoop()
}

func TestJobLock_LeaseLoop2(t *testing.T) {

	jobLock := NewJobLock("jobs/default/123456")

	jobLock.TryLock()

	jobLock.LeaseLoop()
}
