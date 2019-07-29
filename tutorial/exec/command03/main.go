package main

import (
	"context"
	"fmt"
	"os/exec"
	"time"
)

func main() {
	// 执行一个command，让它在一个协诚里去执行，让它执行5秒，在3秒的时候，杀掉这个进程

	// 先定义变量
	var (
		ctx        context.Context    // 上下文
		cancelFunc context.CancelFunc // 上下文取消函数
		cmd        *exec.Cmd
		err        error
		results    []byte
	)

	fmt.Println("======== 第一次执行 ========")
	// 先生成上下文
	ctx, cancelFunc = context.WithCancel(context.TODO())

	// 协诚中执行命令
	go func() {
		command := "echo `date +'%F:%T'` Start！; sleep 3; echo `date +'%F:%T'` End！;"
		cmd = exec.CommandContext(ctx, "/bin/sh", "-c", command)

		if results, err = cmd.CombinedOutput(); err != nil {
			fmt.Println("执行出错", err.Error())
			panic(err)
		} else {
			// 打印输出: byte
			fmt.Println(results)
			//	打印输出：string
			fmt.Println(string(results))
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

		if results, err = cmd.CombinedOutput(); err != nil {
			fmt.Println("执行出错", err.Error())
			panic(err)
		} else {
			// 打印输出: byte
			fmt.Println(results)
			//	打印输出：string
			fmt.Println(string(results))
		}
	}()

	time.Sleep(2 * time.Second)
	// 执行取消函数
	cancelFunc()

	fmt.Println("执行完毕！")
}
