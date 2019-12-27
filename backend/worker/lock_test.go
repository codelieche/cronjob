package worker

import (
	"testing"
	"time"
)

func TestJobLock_TryLock(t *testing.T) {

	jobLock := NewJobLock("jobs/default/123456")

	jobLock.TryLock()

	i := 0
	for i < 10 {
		i++
		jobLock.leaseLock()

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
		jobLock.ctxCancelFunc()
	}()

	jobLock.LeaseLoop()
}

func TestJobLock_LeaseLoop2(t *testing.T) {

	jobLock := NewJobLock("jobs/default/123456")

	jobLock.TryLock()

	jobLock.LeaseLoop()
}
