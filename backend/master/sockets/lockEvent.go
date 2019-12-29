package sockets

import (
	"encoding/json"
	"log"
	"math/rand"
	"strconv"
	"strings"
	"time"

	"github.com/codelieche/cronjob/backend/common/datamodels"
)

func randonIntString() string {
	rand.Seed(time.Now().UnixNano())
	randInt := rand.Intn(100000000)
	secret := strconv.Itoa(randInt)
	return secret
}

// 客户端尝试获取锁
// 尝试上锁，data，传递的是：锁的lock
func tryLockEventHandler(event *MessageEvent, client *Client) {
	// 捕获异常
	defer recoverSocketClientError(client)

	if app == nil {
		initApp()
	}
	// 1. 定义变量
	var (
		lockRequest *datamodels.LockRequest
		remoteAddr  string
		lockName    string // 锁的名字：比如：jobs/:category/:id
		etcdLock    *datamodels.EtcdLock
		isExist     bool // 锁释放存在
		err         error
		secret      string
		result      datamodels.LockResponse // 响应结果
	)

	// 2. 获取变量
	remoteAddr = client.RemoteAddr
	lockRequest = &datamodels.LockRequest{}
	if json.Unmarshal([]byte(event.Data), &lockRequest); err != nil {
		log.Println("解析LockRequest信息出错：", err)
		return
	} else {
		lockName = lockRequest.Name
		lockName = strings.TrimSpace(lockName)
	}

	// 如果锁，存在，那么就直接返回
	if etcdLock, isExist = app.etcdLocksMap[lockName]; isExist {
		// 锁存在，直接返回
		//log.Println(etcdLock)
		if etcdLock.IsLocked {
			goto ERR
		}
	}

	if etcdLock, err = app.etcd.NewEtcdLock(lockName, 10); err != nil {
		log.Println("获取锁失败：", err)
		goto ERR
	} else {
		etcdLock.Description = remoteAddr
	}

	// 3. 尝试上锁
	if err = etcdLock.TryLock(); err != nil {
		log.Println(err)
		goto ERR
	} else {
		// 上锁成功
		event = &MessageEvent{
			Category: "tryLock",
			Data:     etcdLock.GetLeaseID(),
		}

		// 记得设置秘钥
		if lockRequest.Secret != "" {
			secret = lockRequest.Secret
		} else {
			secret = randonIntString()
		}

		//log.Println("lock")
		app.opEtcdLockMux.Lock()
		etcdLock.Secret = secret
		app.etcdLocksMap[lockName] = etcdLock
		//log.Println("unlock")
		app.opEtcdLockMux.Unlock()

		// 回写结果给客户端
		result = datamodels.LockResponse{
			ID:      lockRequest.ID,
			Success: true,
			Name:    lockName,
			Secret:  secret,
			Message: "上锁成功",
		}

		if data, err := json.Marshal(result); err != nil {
			log.Println(err)
		} else {
			event.Data = string(data)
		}

		if err = client.conn.WriteJSON(event); err != nil {
			// 写入数据出错
			// 没发送成功就立刻释放
			log.Println("发送抢锁成功信息失败：", err)
			releaseLockEventHandler(lockName, client)
		} else {
			// 设置自动过期
			go etcdLock.SetAutoKillTicker(func() {
				// 需要发送kill信息
				//log.Println("到期后，后期处理函数")
				//log.Println(conn)
				event.Data = lockName
				releaseLockEventHandler(lockName, client)
			})
		}

		return
	}

ERR:
	// 出现错误，需要告诉客户端，本次上锁失败
	event = &MessageEvent{
		Category: "tryLock",
		Data:     "false",
	}
	// 回写错误结果
	result = datamodels.LockResponse{
		ID:      lockRequest.ID,
		Success: false,
		Name:    lockName,
		Secret:  secret,
		Message: "上锁失败",
	}

	if data, err := json.Marshal(result); err != nil {
		log.Println(err)

	} else {
		event.Data = string(data)
	}

	// conn可能关闭了，就会引发panic
	if client.IsActive {
		client.conn.WriteJSON(event)
	}
	return
}

// 续租key的锁
func leaseLockEventHandler(event *MessageEvent, client *Client) {
	// 捕获异常
	defer recoverSocketClientError(client)

	// 1. 定义变量
	var (
		lockRequest *datamodels.LockRequest
		etcdLock    *datamodels.EtcdLock
		lockName    string
		secret      string
		isExist     bool
		err         error
	)

	// 2. 获取变量
	lockRequest = &datamodels.LockRequest{}
	if err = json.Unmarshal([]byte(event.Data), lockRequest); err != nil {
		log.Println("续租请求信息解析出错：", err)
		return
	} else {
		lockName = lockRequest.Name
		lockName = strings.TrimSpace(lockName)
		secret = lockRequest.Secret
	}

	// 3. 对锁进行续租
	// 上锁
	app.opEtcdLockMux.RLock()

	// 释放读锁
	defer app.opEtcdLockMux.RUnlock()
	if etcdLock, isExist = app.etcdLocksMap[lockName]; isExist {
		if err = etcdLock.ResetTimer(time.Second*10, secret); err != nil {
			log.Printf("%s续租出错：%s", lockName, err)
			goto FAIL
		} else {
			log.Printf("%s续租成功", lockName)
			return
		}
	}

FAIL:
	// 失败了就需要发送信息
	messageEvent := MessageEvent{
		Category: "leaseLock",
		Data:     "",
	}
	result := datamodels.LockResponse{
		ID:      lockRequest.ID,
		Success: false,
		Name:    lockName,
	}
	if data, err := json.Marshal(result); err != nil {

	} else {
		messageEvent.Data = string(data)
	}

	// 发送失败消息
	client.conn.WriteJSON(messageEvent)
}

// 释放锁
// 客户端上锁，用完后，记得释放锁
func releaseLockEventHandler(lockName string, client *Client) {
	// 捕获异常
	//defer recoverSocketClientError(client)
	defer func() {
		if err := recover(); err != nil {
			log.Println(err)
		}
	}()

	// 1. 定义变量
	// log.Println("释放锁")
	var (
		//remoteAddr string
		//lockName string // 锁的名字
		etcdLock *datamodels.EtcdLock
		isExist  bool // 释放存在
	)

	// 2. 获取变量
	//remoteAddr = conn.RemoteAddr().String()
	//lockName = event.Data
	lockName = strings.TrimSpace(lockName)

	// 3. 获取锁
	app.opEtcdLockMux.Lock()
	if etcdLock, isExist = app.etcdLocksMap[lockName]; isExist {
		delete(app.etcdLocksMap, lockName)
		app.opEtcdLockMux.Unlock()
		etcdLock.UnLock()
	} else {
		app.opEtcdLockMux.Unlock()

		// 锁不存在
		log.Println("锁不存在")
	}

	// 4. 发送释放完毕的消息
}
