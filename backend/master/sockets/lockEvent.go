package sockets

import (
	"log"
	"strings"
	"time"

	"github.com/codelieche/cronjob/backend/common/datamodels"
)

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
		remoteAddr string
		lockName   string // 锁的名字：比如：jobs/:category/:id
		etcdLock   *datamodels.EtcdLock
		isExist    bool // 锁释放存在
		err        error
	)

	// 2. 获取变量
	remoteAddr = client.RemoteAddr
	lockName = event.Data
	lockName = strings.TrimSpace(lockName)

	// 如果锁，存在，那么就直接返回
	if etcdLock, isExist = app.etcdLocksMap[lockName]; isExist {
		// 锁存在，直接返回
		//log.Println(etcdLock)
		if etcdLock.IsLocked {
			goto ERR
		}
	}

	if etcdLock, err = app.etcd.NewEtcdLock(lockName); err != nil {
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
		//log.Println("lock")
		app.opEtcdLockMux.Lock()
		app.etcdLocksMap[lockName] = etcdLock
		//log.Println("unlock")
		app.opEtcdLockMux.Unlock()

		if err = client.conn.WriteJSON(event); err != nil {
			// 写入数据出错
			// 没发送成功就立刻释放
			log.Println("发送抢锁成功信息失败：", err)
			releaseLockEventHandler(lockName, client)
		} else {
			// 设置自动过期
			etcdLock.SetAutoKillTicker(func() {
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
		etcdLock *datamodels.EtcdLock
		lockName string
		isExist  bool
	)

	// 2. 获取变量
	lockName = event.Data
	lockName = strings.TrimSpace(lockName)

	// 3. 对锁进行续租
	// 上锁
	app.opEtcdLockMux.RLock()
	if etcdLock, isExist = app.etcdLocksMap[lockName]; isExist {
		etcdLock.ResetTimer(time.Second * 10)
	}
	// 释放读锁
	app.opEtcdLockMux.RUnlock()

}

// 释放锁
// 客户端上锁，用完后，记得释放锁
func releaseLockEventHandler(lockName string, client *Client) {
	// 捕获异常
	defer recoverSocketClientError(client)

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
	if etcdLock, isExist = app.etcdLocksMap[lockName]; isExist {
		app.opEtcdLockMux.Lock()
		delete(app.etcdLocksMap, lockName)
		app.opEtcdLockMux.Unlock()
		etcdLock.UnLock()
	} else {
		// 锁不存在
		log.Println("锁不存在")
	}

	// 4. 发送释放完毕的消息

}
