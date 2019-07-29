package main

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"os/exec"
	"time"
)

// 协程中执行结果
type Result struct {
	Id     int    // 执行ID
	Output []byte // 执行输出的结果
	Err    error  // 执行错误结果
}

func main() {
	// 执行一个command，让它在一个协诚里去执行，让它执行5秒，在3秒的时候，杀掉这个进程

	// 先定义变量
	var (
		ctx        context.Context    // 上下文
		cancelFunc context.CancelFunc // 上下文取消函数
		cmd        *exec.Cmd
		err        error
		output     []byte

		resultsChan chan *Result // 执行结果Channel
	)

	resultsChan = make(chan *Result, 20)

	i := 0
	for {
		i += 1
		if i >= 10 {
			// 跳出循环
			break
		}

		go func(count int) {
			log.Printf("======== 第%d次执行 ========\n", count)
			// 先生成上下文
			ctx, cancelFunc = context.WithCancel(context.TODO())

			// 协诚中执行命令
			go func() {
				n := rand.Intn(10) // 取个随机数
				log.Println(n)
				command := fmt.Sprintf("echo `date` Start！; sleep %d; echo `date` End！;", n)
				log.Println(command)
				cmd = exec.CommandContext(ctx, "/bin/sh", "-c", command)

				output, err = cmd.CombinedOutput()

				//	 把结果传到结果channel中
				resultsChan <- &Result{
					Id:     count,
					Output: output,
					Err:    err,
				}
			}()

			// 获取个随机整数
			n := rand.Intn(10)
			time.Sleep(time.Duration(n) * time.Second)
			// 执行取消函数
			cancelFunc()
		}(i)

	}

	// 开始获取结果
	log.Println("开始获取结果：")
	for _ = range [9]int{} {

		result := <-resultsChan
		log.Printf("======== 打印第%d结果： 开始", result.Id)
		if result.Err != nil {
			log.Println(result.Err.Error())
		} else {
			// 打印输出: byte
			//log.Println(result.Output)
			//	打印输出：string
			log.Println(string(result.Output))

		}
		fmt.Println("\n")
	}

	close(resultsChan)

	fmt.Println("执行完毕！")
}
