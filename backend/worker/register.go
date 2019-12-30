package worker

import (
	"errors"
	"fmt"
	"os"

	"github.com/levigross/grequests"

	"github.com/codelieche/cronjob/backend/common/datamodels"

	"github.com/codelieche/cronjob/backend/common"
)

// 注册节点信息到master
type Register struct {
	Info *datamodels.Worker // Worker节点的信息
	//Name     string `json:"name"`     // 节点的名称：Ip:Port（这样就算唯一的了）
	//HostName string `json:"hostname"` // 主机名
	//Ip       string `json:"ip"`       // IP地址
	//Port     int    `json:"port"`     // worker 监控服务的端口
	//Pid      int    `json:"pid"`      // Worker的端口号
}

// 获取worker信息，然后回写数据到master
// Master API：
// URL: /api/v1/worker/create
// Method: Post
// Data： json
func (register *Register) postWorkerInfoToMaster() (err error) {

	// 1. 定义变量
	var (
		url      string
		ro       *grequests.RequestOptions
		response *grequests.Response
		worker   datamodels.Worker
	)

	if register.Info.Pid < 1 {
		register.Info.GetInfo()
		register.Info.Port = common.GetConfig().Worker.Http.Port
	}

	// 2. 获取变量值
	url = fmt.Sprintf("%s/api/v1/worker/create", common.GetConfig().Worker.MasterUrl)
	ro = &grequests.RequestOptions{
		QueryStruct:    nil,
		JSON:           register.Info,
		Headers:        nil,
		UserAgent:      "",
		RequestTimeout: 0,
		RequestBody:    nil,
	}

	// 3. 发起请求
	if response, err = grequests.Post(url, ro); err != nil {
		return err
	} else {
		// 判断结果
		worker = datamodels.Worker{}
		if err = response.JSON(&worker); err != nil {
			return err
		} else {
			//log.Println(worker)
			if worker.Pid == register.Info.Pid {
				return nil
			} else {
				err = fmt.Errorf("返回的结果的Pid(%d)和当前的Pid不匹配(%d)", worker.Pid, register.Info.Pid)
				return err
			}
		}
	}
}

// 注册到：/crontab/workers/目录中
func (register *Register) keepOnlive() {

}

// worker退出的时候，需要删除掉worker信息
func (register *Register) deleteWorkerInfo() (err error) {

	// 1. 定义变量
	var (
		url      string
		response *grequests.Response
	)

	// 2. 获取变量
	url = fmt.Sprintf("%s/api/v1/worker/%s", common.GetConfig().Worker.MasterUrl, register.Info.Name)

	// 3. 发起删除请求
	if response, err = grequests.Delete(url, nil); err != nil {
		return
	} else {
		if response.StatusCode == 204 {
			// worker应该停止调度了
			if !app.Scheduler.isStoped {
				app.Scheduler.isStoped = true
			}
			return nil
		} else {
			err = errors.New(string(response.Bytes()))
			return err
		}
	}
}

func newRegister() (register *Register, err error) {
	// 定义变量
	var (
		workerInfo *datamodels.Worker
	)

	workerInfo = &datamodels.Worker{
		Port: common.GetConfig().Worker.Http.Port, // web监听的端口号
		Pid:  os.Getppid(),                        // 进程号
	}

	workerInfo.GetInfo()

	register = &Register{
		Info: workerInfo, // 工作节点的信息
	}

	return register, err
}
