// 计划任务的执行
package worker

import (
	"os/exec"
	"time"

	"github.com/codelieche/cronjob/backend/common"
)

// 任务执行器
type Executor struct {
}

// 执行一个任务
func (executor *Executor) ExecuteJob(info *common.JobExecuteInfo, c chan<- *common.JobExecuteResult) (err error) {
	// log.Println("执行计划任务：", info.Job.Name, info.Job.Time)
	// 启动一个协程来执行command
	go func() {
		// 执行shell命令
		var (
			cmd       *exec.Cmd                // shell执行命令
			output    []byte                   // job执行的输出结果
			result    *common.JobExecuteResult // Job执行的结果
			timeStart time.Time                // 开始执行时间
			jobLock   *common.JobLock          // 计划任务的锁
		)

		// 初始化分布式锁
		jobLock = app.EtcdManager.CreateJobLock(info.Job.Name)

		// 尝试上锁
		if err = jobLock.TryLock(); err != nil {
			// 上锁失败，无需执行
			// log.Println("上锁失败：", jobLock, err.Error())
			// 执行结果
			result = &common.JobExecuteResult{
				ExecuteInfo: info,
				IsExecute:   false,
				Output:      nil,
				Err:         err,
				StartTime:   time.Now(),
				EndTime:     time.Now(),
			}
			// 即使未执行也需要把结果输出到channel中
			c <- result
			return
		} else {
			// 上锁成功才执行shell命令
			defer jobLock.Unlock()
			// log.Println("获取到锁：", jobLock)
		}

		// 开始执行时间: 得到锁才会去执行的
		timeStart = time.Now()
		// 传入执行command的上下文
		cmd = exec.CommandContext(info.ExecuteCtx, "/bin/bash", "-c", info.Job.Command)

		// 执行并捕获输出
		output, err = cmd.CombinedOutput()

		// 任务执行完成后，把执行的结果返回给Scheduler
		// Scheduler会从executingTable中删除执行记录
		result = &common.JobExecuteResult{
			ExecuteInfo: info,
			IsExecute:   true, // 有执行到
			Output:      output,
			Err:         err,
			StartTime:   timeStart,
			EndTime:     time.Now(),
		}

		// 推送结果
		c <- result

	}()
	return
}

// 初始化执行器
func NewExecutor() (executor *Executor) {
	executor = &Executor{}
	return
}
