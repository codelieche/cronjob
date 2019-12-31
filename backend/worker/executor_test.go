package worker

import (
	"log"
	"testing"
	"time"

	"github.com/codelieche/cronjob/backend/common/datamodels"
)

func TestExecutor_PostJobExecuteToMaster(t *testing.T) {
	// 1. 定义变量
	executor := Executor{}

	now := time.Now()
	jobExecuter := datamodels.JobExecute{
		Worker:       "test-worker",
		Category:     "default",
		Name:         "test job",
		JobID:        1,
		Command:      "echo `date`; sleep 10",
		Status:       "start",
		PlanTime:     now,
		ScheduleTime: now.Add(time.Second),
		StartTime:    now.Add(time.Second),
		EndTime:      now.Add(time.Minute),
		LogID:        "",
	}

	// 2. 创建向master发起请求
	if jobExecuter, err := executor.PostJobExecuteToMaster(&jobExecuter); err != nil {
		t.Error(err)
	} else {
		log.Println(jobExecuter)
	}

}

func TestExecutor_PostJobExecuteResultToMaster(t *testing.T) {
	// 1. 定义变量
	executor := Executor{}

	now := time.Now()
	jobExecuter := datamodels.JobExecute{
		Worker:       "test-worker",
		Category:     "default",
		Name:         "test job",
		JobID:        1,
		Command:      "echo `date`; sleep 10",
		Status:       "start",
		PlanTime:     now,
		ScheduleTime: now.Add(time.Second),
		StartTime:    now.Add(time.Second),
		EndTime:      now.Add(time.Minute),
		LogID:        "",
	}

	// 2. 创建向master发起请求
	if jobExecuter, err := executor.PostJobExecuteToMaster(&jobExecuter); err != nil {
		t.Error(err)
	} else {
		log.Println(jobExecuter)
		// 3. 构建执行结果
		result := datamodels.JobExecuteResult{
			ExecuteID:  jobExecuter.ID,
			IsExecuted: true,
			Output:     []byte("这个是测试内容的日志"),
			Err:        nil,
			StartTime:  time.Now(),
			EndTime:    time.Now().Add(time.Minute),
			Status:     "",
		}

		if jobExecuter, err := executor.PostJobExecuteResultToMaster(&result); err != nil {
			t.Error(err)
		} else {
			log.Println(jobExecuter)
		}
	}

}
