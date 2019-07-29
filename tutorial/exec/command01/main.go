package main

import (
	"fmt"
	"os/exec"
)

func main() {

	var (
		cmd *exec.Cmd
		err error
	)

	fmt.Println("第一次执行")
	cmd = exec.Command("/bin/bash", "-c", "echo `date`")
	//cmd = exec.Command("/bin/bash2", "-c", "echo `date`")

	err = cmd.Run()
	if err != nil {
		fmt.Println("执行出错", err.Error())
		panic(err)
	}

	fmt.Println("第二次执行")
	cmd = exec.Command("/bin/sh", "-c", "echo `date +'%F:%T'`")
	//cmd = exec.Command("/bin/sh2", "-c", "echo `date`")

	err = cmd.Run()
	if err != nil {
		fmt.Println("执行出错", err.Error())
		panic(err)
	}

	fmt.Println("执行完毕！")
}
