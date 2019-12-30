package worker

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strconv"
	"time"

	"github.com/codelieche/cronjob/backend/common"

	"github.com/levigross/grequests"

	"github.com/codelieche/cronjob/backend/common/datamodels"
)

// 计划任务的锁
// 通过httl连接发起TryLock.
// 通过http发起续租
// 通过http发起release释放锁
type JobLock struct {
	ID           int         `json:"id"`        // 锁请求的序号
	Name         string      `json:"name"`      // 锁的名称
	LeaseID      int64       `json:"lease_id"`  // 锁对应的租约ID
	Password     string      `json:"password"`  // 锁的密码
	IsActive     bool        `json:"is_active"` // 锁是否有效中
	NeedKillChan chan bool   // 释放本程序的通道：当timer到期了，还未发起续租，那么就需要kill，jobLock对应的任务
	closeChan    chan bool   // 关闭锁的通道
	timer        *time.Timer // 取消当前锁的定时器，如果8秒钟内未成功发其续租，timer就到期了
}

// 实例化一个JobLock
func NewJobLock(name string) (jobLock *JobLock) {
	// 定义变量

	// 尝试获取锁
	timer := time.NewTimer(time.Second * 9)
	jobLock = &JobLock{
		Name:         name,
		Password:     "",
		NeedKillChan: make(chan bool, 5), // 是否需要杀掉锁绑定的任务
		closeChan:    make(chan bool, 5),
		timer:        timer, // 计时器，当时间到了，立刻发送kill信息
	}
	return jobLock
}

// 尝试上锁
// 通过http向master发起抢锁请求
// 如果抢锁成功会返回200的结果
// 抢锁失败就是400的错误请求
// Master API：
// URL: /api/v1/lock/create
// Method: POST
// Data: {"name": 锁的名字, "ttl": "锁绑定的租约的time to live"}
func (jobLock *JobLock) TryLock() (err error) {

	// 捕获异常
	defer func() {
		if err := recover(); err != nil {
			log.Println(err)
		}
	}()

	// 1. 定义变量
	var (
		url      string
		ro       *grequests.RequestOptions
		lock     datamodels.Lock   // 锁
		data     map[string]string // 请求数据
		response *grequests.Response
	)

	// 2. 获取变量
	url = fmt.Sprintf("%s/api/v1/lock/create", common.GetConfig().Worker.MasterUrl)
	data = map[string]string{"name": jobLock.Name, "ttl": "10"}
	ro = &grequests.RequestOptions{
		Data:    data,
		Headers: nil,
	}

	// 3. 发起上锁请求
	if response, err = grequests.Post(url, ro); err != nil {
		// 通过http发起锁请求失败
		log.Println(err)
		return err
	} else {
		// 4. 对结果进行判断
		defer response.Close()

		// 4-1：判断抢锁的响应码
		// 当锁被占用的时候响应码会是400的
		if response.StatusCode != 200 {
			err = errors.New(string(response.Bytes()))
			return
		}

		// 4-2：反序列化
		lock = datamodels.Lock{}

		if err = json.Unmarshal(response.Bytes(), &lock); err != nil {
			log.Println(string(response.Bytes()))
			return err
		} else {
			// 反序列化成功

			// 4-3：对结果进行判断
			//log.Println(lock)
			if lock.IsActive && lock.LeaseID > 0 {
				jobLock.IsActive = true
				jobLock.LeaseID = lock.LeaseID
				jobLock.Password = lock.Password
				// 5：重点，启动自动续租的协程
				return nil
			} else {
				// 获取锁失败
				err = errors.New("抢锁失败")
				return err
			}
		}
	}

}

// 循环发起续租
func (jobLock *JobLock) LeaseLoop() {
	ticker := time.NewTicker(time.Second * 5)
	for {
		select {
		case <-ticker.C:
			// 发起续租: 每隔5秒钟，发起一次请求
			if err := jobLock.leaseRequest(); err != nil {
				log.Println("续租出现错误：", err)
				goto END
			} else {
				//log.Println("续租成功")
				jobLock.timer.Reset(time.Second * 9)
			}
		case <-jobLock.timer.C:
			// 9秒内，未发起过续租请求，这个时候，jobLock应该关闭了
			// 需要杀掉jobLock所绑定的任务
			if jobLock.IsActive {
				log.Println("jobLock timer 到期: 需要kill lock绑定的任务")
				jobLock.NeedKillChan <- true
			}
			// 1. 让jobLock绑定的上下文取消
			// 2. 设置状态是false
			// 执行下Unlock函数即可
			jobLock.Unlock()
			goto END
		case <-jobLock.closeChan:
			goto END
		}
	}
END:
	//log.Println("LeaseLoop Done")
	jobLock.Unlock()
}

// 释放锁
// 释放租约应该也需要传递秘钥，后续优化，这里暂时只传递锁的名字
func (jobLock *JobLock) ReleaseLock() (err error) {

	if err = jobLock.releaseRequest(); err != nil {
		// 发起释放锁失败
		return err
	} else {
		return nil
	}
}

// 发起续租的请求
// Master API：
// URL: /api/v1/lock/lease
// Method: POST
// Data: {"lease_id": 租约的iD, "password": "锁的密码"}
func (jobLock *JobLock) leaseRequest() (err error) {
	// 1. 定义变量
	var (
		url          string
		leaseID      string
		data         map[string]string
		ro           *grequests.RequestOptions
		response     *grequests.Response
		baseResponse datamodels.BaseResponse
	)

	// 2. 获取变量
	if !jobLock.IsActive {
		err = fmt.Errorf("锁%s的状态已经是false，不可再续租", jobLock.Name)
		return err
	}
	url = fmt.Sprintf("%s/api/v1/lock/lease", common.GetConfig().Worker.MasterUrl)

	leaseID = strconv.Itoa(int(jobLock.LeaseID))
	data = map[string]string{"lease_id": leaseID, "password": jobLock.Password}
	ro = &grequests.RequestOptions{
		Data:    data,
		Headers: nil,
	}

	// 3. 发起请求
	if response, err = grequests.Post(url, ro); err != nil {
		// 出现错误，可延时1秒后，继续发起请求
		log.Println(err)
		if jobLock.IsActive {
			// 这里后续，可加个重试次数，大于几次就不重试了
			// 但是有个全局的timer，重试2-3次也就，不可试了
			time.Sleep(time.Second)
			return jobLock.leaseRequest()
		} else {
			return err
		}
	} else {
		// log.Println(string(response.Bytes()))
		defer response.Close()
		// 4. 对结果进行判断
		//4-1: 对结果反序列化
		baseResponse = datamodels.BaseResponse{}
		if err = json.Unmarshal(response.Bytes(), &baseResponse); err != nil {
			return err
		} else {
			// 4-2: 对status进行判断
			if baseResponse.Status == "success" {
				// 重置jobLock.timer： 放到loop中执行
				// log.Println("设置timer", jobLock.timer)
				// jobLock.timer.Reset(time.Second * 9)
				return nil
			} else {
				err = errors.New(baseResponse.Message)
				return err
			}
		}
	}
}

// 发起释放租约请求
// Master API：
// URL: /api/v1/lock/release
// Method: Delete
func (jobLock *JobLock) releaseRequest() (err error) {
	// 1. 定义变量
	var (
		url string
		//ro  *grequests.RequestOptions
		response     *grequests.Response
		baseResponse datamodels.BaseResponse
	)

	// 2. 获取变量
	url = fmt.Sprintf("%s/api/v1/lock/release/%d", common.GetConfig().Worker.MasterUrl, jobLock.LeaseID)

	// 3. 发起请求
	if response, err = grequests.Delete(url, nil); err != nil {
		return err
	} else {
		defer response.Close()
		// 4. 对结果进行判断
		//4-1: 对结果反序列化
		baseResponse = datamodels.BaseResponse{}
		if err = json.Unmarshal(response.Bytes(), &baseResponse); err != nil {
			return err
		} else {
			// 4-2: 对status进行判断
			if baseResponse.Status == "success" {
				return nil
			} else {
				err = errors.New(baseResponse.Message)
				return err
			}
		}
	}
}

func (jobLock *JobLock) Unlock() {
	// 释放锁
	//log.Println("jobLock.UnLock")

	// 1. 发起释放锁的请求
	jobLock.releaseRequest()
	//if err := jobLock.releaseRequest(); err != nil {
	//	log.Println(err)
	//}

	// 1. 执行取消函数

	// 2. 设置状态为false
	if jobLock.IsActive {
		jobLock.IsActive = false
	}

	// 3. 发送个close到closeChan
	jobLock.closeChan <- true

	// 删掉锁信息

	//log.Println("Un Lock Done")
}
