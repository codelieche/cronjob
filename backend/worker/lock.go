package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/codelieche/cronjob/backend/common"

	"github.com/codelieche/cronjob/backend/master/sockets"

	"github.com/codelieche/cronjob/backend/common/datamodels"
)

// 计划任务的锁
// 通过socket连接发起TryLock
// 通过socket发起续租
// 通过socket发起lease释放锁
type JobLock struct {
	ID             int                           `json:"id"`     // 锁请求的序号
	Name           string                        `json:"name"`   // 锁的名称
	Secret         string                        `json:"secret"` // 锁的秘钥
	ctx            context.Context               // 上下文
	ctxCancelFunc  context.CancelFunc            // 取消函数
	IsActive       bool                          `json:"is_active"` // 锁是否有效中
	lockResultChan chan *datamodels.LockResponse // 上锁后，要等待这个结果
	killChan       chan bool                     // 释放本程序的通道
	closeChan      chan int                      // 关闭锁的通道
}

func NewJobLock(name string) (jobLock *JobLock) {
	if socket == nil {
		connectMasterSocket()
	}

	// 尝试获取锁
	socket.lock.Lock()
	socket.RequestID++

	ctx, ctxCancelFun := context.WithCancel(context.Background())

	jobLock = &JobLock{
		ID:             socket.RequestID,
		Name:           name,
		Secret:         "",
		ctx:            ctx,
		ctxCancelFunc:  ctxCancelFun,
		lockResultChan: make(chan *datamodels.LockResponse, 5),
		killChan:       make(chan bool, 5),
		closeChan:      make(chan int, 5),
	}

	// 把joblock加入到socket的结构体中
	socket.jobLockMap[jobLock.ID] = jobLock
	socket.lock.Unlock()

	return jobLock
}

// 等待lockSuccess的协程
func (jobLock *JobLock) waitSocketTryLockResult() {
	timer := time.NewTimer(time.Second * 5)
	//ctx, _ := context.WithTimeout(context.Background(), time.Second*5)
	for {
		select {
		case result := <-jobLock.lockResultChan:
			// log.Println(result)
			if result != nil {
				jobLock.Secret = result.Secret
				// 成功的话，锁就有效
				jobLock.IsActive = result.Success
			}
			goto END
		case <-timer.C:
			//case <-ctx.Done():
			//log.Println("超时啦")
			jobLock.killChan <- true
			goto END
		}
	}
END:
	//log.Println("结束")
	return
}

// 尝试上锁
func (jobLock *JobLock) TryLock() (err error) {

	// 捕获异常
	defer func() {
		if err := recover(); err != nil {
			log.Println(err)
		}
	}()

	// 定义变量
	var (
		lockRequest  datamodels.LockRequest // 上锁请求信息
		messageEvent sockets.MessageEvent   // 给socket发送的event事件
		data         []byte
	)

	// 1. 发送上锁消息
	lockRequest = datamodels.LockRequest{
		ID:     jobLock.ID,
		Name:   jobLock.Name,
		Secret: "",
	}

	messageEvent = sockets.MessageEvent{
		Category: "tryLock",
		Data:     "",
	}

	if data, err = json.Marshal(lockRequest); err != nil {
		return err
	} else {
		messageEvent.Data = string(data)
	}

	if data, err = json.Marshal(messageEvent); err != nil {
		return err
	}

	log.Printf("%s\n", common.Packet(data))
	if err = socket.conn.WriteJSON(data); err != nil {
		// 发送消息失败
		log.Println(err)
		return err
	} else {
		// 判断是否成功
		//log.Println("等待结果的响应了")
		jobLock.waitSocketTryLockResult()
		//log.Println("等待结果的响应Done")
		if jobLock.IsActive {
			return nil
		} else {
			err = fmt.Errorf("上锁失败:%s", jobLock.Name)
			return err
		}

	}
}

// 续租
func (jobLock *JobLock) leaseLock() (err error) {

	// 1. 发送上锁消息
	lockRequest := datamodels.LockRequest{
		ID:     jobLock.ID,
		Name:   jobLock.Name,
		Secret: jobLock.Secret,
	}

	messageEvent := sockets.MessageEvent{
		Category: "leaseLock",
		Data:     "",
	}

	if data, err := json.Marshal(lockRequest); err != nil {

	} else {
		messageEvent.Data = string(data)
	}

	if err := socket.conn.WriteJSON(messageEvent); err != nil {
		// 发送消息失败
		log.Println("发送续租失败", err)
		return err
	} else {
		// 判断是否成功
		// log.Println("发送续租成功:", messageEvent.Data)
	}
	return nil
}

// 循环发起续租
func (jobLock *JobLock) LeaseLoop() {
	timer := time.NewTimer(time.Second * 9)

	for {
		select {
		case <-timer.C:
			// 发起续租
			if err := jobLock.leaseLock(); err != nil {
				log.Println("续租出现错误：", err)
				goto END
			} else {
				timer.Reset(time.Second * 9)
			}
		case <-jobLock.killChan:
			log.Println("收到kill信息")
			goto END
		case <-jobLock.ctx.Done():
			// 上下文取消了
			// log.Println("上下文取消了")
			goto END
		}
	}
END:
	//log.Println("LeaseLoop Done")
	jobLock.Unlock()
}

// 是否锁
// 释放租约应该也需要传递秘钥，后续优化，这里暂时只传递锁的名字
func (jobLock *JobLock) ReleaseLock() (err error) {

	// 1. 发送上锁消息
	lockRequest := datamodels.LockRequest{
		ID:     jobLock.ID,
		Name:   jobLock.Name,
		Secret: jobLock.Secret,
	}

	messageEvent := sockets.MessageEvent{
		Category: "releaseLock",
		Data:     lockRequest.Name,
	}

	//if data, err := json.Marshal(lockRequest); err != nil {
	//
	//} else {
	//	messageEvent.Data = string(data)
	//}

	if err := socket.conn.WriteJSON(messageEvent); err != nil {
		// 发送消息失败
		log.Println(err)
		return err
	} else {
		// 判断是否成功
		//log.Println("发送释放锁信息成功:", messageEvent.Data)
	}
	return nil
}

func (jobLock *JobLock) Unlock() {
	// 释放锁
	//log.Println("jobLock.UnLock")

	// 执行取消函数
	jobLock.ctxCancelFunc()

	// 删掉锁信息
	socket.lock.Lock()
	if _, isExist := socket.jobLockMap[jobLock.ID]; isExist {
		delete(socket.jobLockMap, jobLock.ID)
	}
	// 释放锁
	jobLock.ReleaseLock()
	socket.lock.Unlock()
	//log.Println("Un Lock Done")
}
