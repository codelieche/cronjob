package main

import (
	"context"
	"fmt"
	"log"
	"os/exec"
	"time"
)

// 协程中执行结果
type Result struct {
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

	fmt.Println("======== 第一次执行 ========")
	// 先生成上下文
	ctx, cancelFunc = context.WithCancel(context.TODO())

	// 协诚中执行命令
	go func() {
		command := "echo `date +'%F:%T'` Start！; sleep 3; echo `date +'%F:%T'` End！;"
		cmd = exec.CommandContext(ctx, "/bin/sh", "-c", command)

		output, err = cmd.CombinedOutput()

		//	 把结果传到结果channel中
		resultsChan <- &Result{
			Output: output,
			Err:    err,
		}
	}()

	time.Sleep(4 * time.Second)
	// 执行取消函数
	cancelFunc()

	fmt.Println("======== 第二次执行 ========")
	// 先生成上下文
	ctx, cancelFunc = context.WithCancel(context.TODO())
	// 协诚中执行命令
	go func() {
		command := "echo `date +'%F:%T'` Start！; sleep 5; echo `date +'%F:%T'` End！;"
		cmd = exec.CommandContext(ctx, "/bin/sh", "-c", command)

		output, err = cmd.CombinedOutput()

		//	 把结果传到结果channel中
		resultsChan <- &Result{
			Output: output,
			Err:    err,
		}
	}()

	time.Sleep(2 * time.Second)
	// 执行取消函数
	cancelFunc()

	// 开始获取结果

	for _ = range [2]int{} {

		result := <-resultsChan
		log.Println("打印结果：")
		if result.Err != nil {
			log.Println(result.Err.Error())
		} else {
			// 打印输出: byte
			log.Println(result.Output)
			//	打印输出：string
			log.Println(string(result.Output))

		}
	}

	fmt.Println(resultsChan)

	fmt.Println("执行完毕！")
}
