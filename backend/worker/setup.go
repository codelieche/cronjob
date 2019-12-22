package worker

import (
	"errors"
	"fmt"
	"log"
	"os/exec"
	"strings"

	"github.com/codelieche/cronjob/backend/common/datamodels"
)

// worker工作节点setup执行环境
// 执行分类相关的命令：
// 1. 先执行checkCmd的命令：成功就跳过，
// 2. 未成功就执行SetupCmd，再执行CheckCmd
// 3. 当worker关闭的时候，执行TearDownCmd的命令
func (w *Worker) setupExecuteEnvrionment() {
	var (
		categoryName string
		success      bool
		err          error
	)
	w.Categories = config.Categories

	// 遍历worker的计划任务类型，逐个设置
	if len(w.Categories) < 1 {
		log.Println("worker的Categories为空，程序自动加入default")
		//w.Categories = append(w.Categories, "default")
		w.Categories["default"] = true
	}

	for categoryName = range w.Categories {
		if success, err = w.checkOrSetUpJobExecuteEnvironment(categoryName); err != nil {
			log.Printf("检查分类%s执行环境出错：%s", categoryName, err.Error())
			// 不退出，让程序继续执行
			//os.Exit(1)
		} else {
			if success {
				log.Printf("已经准备好执行%s类型的任务", categoryName)
			}
		}
	}
}

// worker add category
func (w *Worker) addExecuteCategory(name string) (success bool, err error) {
	var (
		isExist       bool
		categoryValue bool
	)

	name = strings.TrimSpace(name)

	if categoryValue, isExist = w.Categories[name]; isExist {
		msg := fmt.Sprintf("%s分类已经存在,值是：%v", name, categoryValue)
		err = errors.New(msg)
		return false, err
	} else {
	}

	// 开始添加
	if success, err = w.checkOrSetUpJobExecuteEnvironment(name); err != nil {
		return
	} else {
		// 如果成功了，需要修改w.Categories
		if success {
			w.Categories[name] = true
		} else {
			// 竟然没成功
		}
	}
	return
}

// worker delete category
func (w *Worker) removeExecuteCategory(name string) (success bool, err error) {
	// 定义变量
	var (
		categoryValue bool
		isExist       bool
	)

	// 判断name是否为空
	name = strings.TrimSpace(name)
	if name == "" {
		err = errors.New("传入的name为空")
		return
	}

	if categoryValue, isExist = w.Categories[name]; !isExist {
		msg := fmt.Sprintf("%s不存在于worker的categories中", name)
		err = errors.New(msg)
		return
	} else {
		//log.Println(name, categoryValue)
		if categoryValue {
			delete(w.Categories, name)
			return true, nil
		} else {
			delete(w.Categories, name)
			return true, nil
		}
	}
}

// 检查或者准备执行某类计划任务的环境
func (w *Worker) checkOrSetUpJobExecuteEnvironment(name string) (success bool, err error) {
	// 定义变量
	var (
		category *datamodels.Category
	)

	// 第1步：先获取分类信息
	if category, err = w.CategoryRepo.GetByName(name); err != nil {
		// 1-1: 如果获取当前分类没有，就返回
		if name == "default" {
			category = &datamodels.Category{
				IsActive:    true,
				Name:        "default",
				Description: "默认的任务类型",
				CheckCmd:    "which bash",
				SetupCmd:    "echo `date`; sleep 1; echo `date`",
				TearDownCmd: "echo `date`; sleep 1; echo `date`",
			}
			// 保存到etcd中
			if _, err = w.CategoryRepo.Save(category); err != nil {
				// 插入出错，返回
				return
			} else {
				// 继续后续的操作
			}
		} else {
			// 获取分类信息出错, 返回吧
			return
		}
	} else {
		if !category.IsActive {
			msg := fmt.Sprintf("%s类型任务is_active是false，不可执行, 请设置其为true后方可执行", name)
			err = errors.New(msg)
			return
		}
	}

	// 第2步：开始执行检查任务
	if category.CheckCmd != "" {
		// 2-1: 执行CheckCmd
		if success, err = executeCommand(category.CheckCmd); err != nil {
			// 执行检查命令出错，这个时候进入第三步，执行SetUp操作
			goto NEEDSETUP
		} else {
			// 判断是否成功
			if success {
				return success, err
			} else {
				goto NEEDSETUP
			}
		}
	NEEDSETUP:
		if category.SetupCmd == "" {
			//	如果Setup的命令为空，那就抛出错误
			return false, err
		} else {
			// 执行Setup命令
			//	进入后续的步骤
		}
	}

	// 第3步：执行Setup
	if category.SetupCmd != "" {
		// 3-1: 执行setup命令
		if success, err = executeCommand(category.SetupCmd); err != nil {
			// 执行setup出错
			return false, err
		} else {
			//	3-2: 执行检查命令
			if category.CheckCmd != "" {
				return executeCommand(category.CheckCmd)
			} else {
				return true, err
			}
		}
	} else {
		// setup为空的话：也直接返回吧
	}
	return true, nil
}

// 执行命令
func executeCommand(cmdStr string) (success bool, err error) {
	var (
		cmd        *exec.Cmd
		outputData []byte
	)
	cmd = exec.Command("/bin/bash", "-c", cmdStr)
	if outputData, err = cmd.CombinedOutput(); err != nil {
		// 执行出错
		log.Printf("执行出错的命令是：%s\n", cmdStr)
		return false, err
	} else {
		log.Println(string(outputData))
		return true, nil
	}
}
