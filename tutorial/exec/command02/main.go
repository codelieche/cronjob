package main

import (
	"fmt"
	"os/exec"
)

func main() {

	// 先定义变量
	var (
		cmd     *exec.Cmd
		err     error
		results []byte
	)

	fmt.Println("======== 第一次执行 ========")
	cmd = exec.Command("/bin/bash", "-c", "echo `date`")
	if results, err = cmd.CombinedOutput(); err != nil {
		fmt.Println("执行出错", err.Error())
		panic(err)
	} else {
		// 打印输出: byte
		fmt.Println(results)
		//	打印输出：string
		fmt.Println(string(results))
	}

	fmt.Println("======== 第二次执行 ========")
	cmd = exec.Command("/bin/sh", "-c", "echo `date +'%F:%T'`")
	//cmd = exec.Command("/bin/sh2", "-c", "echo `date`")
	if results, err = cmd.CombinedOutput(); err != nil {
		fmt.Println("执行出错", err.Error())
		panic(err)
	} else {
		// 打印输出: byte
		fmt.Println(results)
		//	打印输出：string
		fmt.Println(string(results))
	}

	fmt.Println("执行完毕！")
}
