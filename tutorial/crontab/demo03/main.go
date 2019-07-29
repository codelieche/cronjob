package main

import (
	"log"
	"time"

	"github.com/gorhill/cronexpr"
)

type CronJob struct {
	Expr     *cronexpr.Expression
	NextTime time.Time // expr.Next(now)
}

func main() {

	// 定义个调度表
	var scheduleTable map[string]*CronJob
	scheduleTable = make(map[string]*CronJob)

	// job 01
	if expr, err := cronexpr.Parse("*/5 * * * * * *"); err != nil {
		log.Panic(err)
	} else {
		log.Println(expr)
		//	去当前时间
		now := time.Now()

		cronJob := &CronJob{
			Expr:     expr,
			NextTime: expr.Next(now),
		}

		scheduleTable["job1"] = cronJob
	}

	// job 02
	if expr, err := cronexpr.Parse("*/20 * * * * * *"); err != nil {
		log.Panic(err)
	} else {
		log.Println(expr)
		//	去当前时间
		now := time.Now()

		cronJob := &CronJob{
			Expr:     expr,
			NextTime: expr.Next(now),
		}

		scheduleTable["job2"] = cronJob
	}

	// 启动一个调度协程
	go func() {
		//	定时检查任务调度表

		for {
			for jobName, cronJob := range scheduleTable {
				now := time.Now()
				//log.Println(jobName, cronJob, now)
				//	判断是否过期
				if cronJob.NextTime.Before(now) || cronJob.NextTime.Equal(now) {
					//	启动一个协程，执行这个任务
					go func(jobName string) {
						log.Println("执行：", jobName)
					}(jobName)

					//	计算下一次调度时间
					cronJob.NextTime = cronJob.Expr.Next(now)
					log.Println(jobName, "下次执行时间", cronJob.NextTime.Format("2006-01-02 15:04:05"))
				} else {
					//	如果没有过期
					select {
					case <-time.NewTimer(100 * time.Millisecond).C: // 将在100毫秒后可读，返回
					}
					// 或者直接睡眠100毫秒
					// time.Sleep(100 * time.Millisecond)
				}

			}
		}

	}()

	// 避免主协程退出
	time.Sleep(100 * time.Second)
	log.Println("=== Done ===")

}
