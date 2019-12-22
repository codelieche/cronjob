// 计划任务的执行
package worker

import (
	"fmt"
	"log"
	"os/exec"
	"time"

	"github.com/codelieche/cronjob/backend/common/datamodels"

	"github.com/codelieche/cronjob/backend/common"
)

// 任务执行器
type Executor struct {
}

// 执行一个任务
func (executor *Executor) ExecuteJob(info *datamodels.JobExecuteInfo, c chan<- *datamodels.JobExecuteResult) (err error) {
	// log.Println("执行计划任务：", info.Job.Name, info.Job.Time)
	// 启动一个协程来执行command
	go func() {
		// 执行shell命令
		var (
			jobExecute  *datamodels.JobExecute       // 任务执行
			jobLockName string                       // job锁的名字
			cmd         *exec.Cmd                    // shell执行命令
			output      []byte                       // job执行的输出结果
			result      *datamodels.JobExecuteResult // Job执行的结果
			timeStart   time.Time                    // 开始执行时间
			jobLock     *common.JobLock              // 计划任务的锁
		)

		// 初始化分布式锁: 分类/job_id
		jobLockName = fmt.Sprintf("jobs/%s/%d", info.Job.Category, info.Job.ID)
		jobLock = app.EtcdManager.CreateJobLock(jobLockName)
		// log.Println(info.Job)
		if !info.Job.IsActive {
			log.Println("当前Job状态是false，无需执行：", info.Job)
			return
		}

		// 尝试上锁
		if err = jobLock.TryLock(); err != nil {
			// 上锁失败，无需执行
			// log.Println("上锁失败：", jobLock, err.Error())
			// 执行结果
			result = &datamodels.JobExecuteResult{
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
			// 上锁成功才执行shell命令: 进入后续的命令
			defer jobLock.Unlock()
			// log.Println("获取到锁：", jobLock)
		}

		jobExecute = &datamodels.JobExecute{
			Worker:       register.Info.Name,
			Category:     info.Job.Category,
			Name:         info.Job.Name,
			JobID:        int(info.Job.ID),
			Command:      info.Job.Command,
			Status:       "start",
			PlanTime:     info.PlanTime,
			ScheduleTime: info.ExecuteTime,
			StartTime:    time.Now(),
			LogID:        "",
		}
		// 保存任务执行信息
		if jobExecute, err = app.JobExecuteRepo.Create(jobExecute); err != nil {
			log.Println("保存执行信息出错：", err)
		} else {
			info.JobExecuteID = jobExecute.ID
		}

		// log.Println("我是否上锁成功：", jobLock.IsLocked)
		// 判断是否要执行取消函数
		go func() {
			// 检查执行程序
			needKillJob := <-jobLock.NeedKillChan
			if needKillJob {
				//log.Println("需要执行取消函数")
				info.ExceteCancelFun()
			} else {
				// 正常退出的程序无需执行
				// log.Println("正常退出的程序，无需执行啥")
			}
		}()

		// 开始执行时间: 得到锁才会去执行的
		timeStart = time.Now()
		// 传入执行command的上下文
		cmd = exec.CommandContext(info.ExecuteCtx, "/bin/bash", "-c", info.Job.Command)

		// 如果需要日志就绑定output
		if info.Job.SaveOutput {
			// 执行并捕获输出
			output, err = cmd.CombinedOutput()
			//	如果想不保存执行信息，可把推送结果的放到这里来处理：c <- result

		} else {
			//  log.Println("无需捕获输出结果：依然也需要执行")

			err = cmd.Run()
			if err != nil {
				log.Println(info.Job.Name, "执行出错：", err)
			}
			output = []byte("Don't save output")
		}

		// 无论是否需要saveOutput，都记录执行信息
		// 任务执行完成后，把执行的结果返回给Scheduler
		// Scheduler会从executingTable中删除执行记录
		result = &datamodels.JobExecuteResult{
			ExecuteID:   info.JobExecuteID, // 把JobExecuteID传递给Result
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
