package datamodels

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/coreos/etcd/clientv3"
)

var EtcdJobsLockDir = "/cronjob/lock/"

// etcd分布式锁
// 计划任务执行的时候需要锁：/crontab/lock/jobs/:category/:name
// client --> server:
// TryLocker: 申请锁，判断是否成功
type EtcdLock struct {
	CreatedAt    time.Time          `json:"created_at"`  // 创建时间
	Name         string             `json:"name"`        // 锁的名字，eg：jobs/:category/:name
	kv           clientv3.KV        `json:"-"`           // KV
	lease        clientv3.Lease     `json:"-"`           // 租约
	cancelFunc   context.CancelFunc `json:"-"`           // 取消函数：用于取消自动续租
	LeaseID      clientv3.LeaseID   `json:"lease_id"`    // 租约ID
	IsLocked     bool               `json:"is_locked"`   // 释放上锁成功，也可根据leaseID释放不为0判断
	NeedKillChan chan bool          `json:"-"`           // 是否需要杀掉当前上锁的程序
	Description  string             `json:"description"` // 备注信息
	ResetTimes   int                `json:"reset_times"` // 重置定时器的次数
	Secret       string             `json:"secret"`      // 设置个秘钥，续租的时候需要提供
	timer        *time.Timer        `json:"-"`           // 定时器
}

func (etcdLock *EtcdLock) GetLeaseID() string {
	return strconv.Itoa(int(etcdLock.LeaseID))
}

// 发起一次续租
// 注意：假如租约关联的key如果被删除了，租约还存在【还在有效期内】
// 那么我们发起续租租约会成功，但是key却已经删除了
func (etcdLock *EtcdLock) KeepAliveOnce(secret string) (success bool, err error) {
	// 1. 定义变量
	var (
		keepAliveResponse *clientv3.LeaseKeepAliveResponse
	)

	// 2. 对秘钥进行判断
	if etcdLock.Secret != "" && etcdLock.Secret != secret {
		err = fmt.Errorf("%s的秘钥传入不正确", etcdLock.Name)
		return false, err
	}

	// 3. 发起续租
	if keepAliveResponse, err = etcdLock.lease.KeepAliveOnce(context.Background(), etcdLock.LeaseID); err != nil {
		// 发起续租失败
		return false, err
	}

	// 4. 对续租结果进行判断
	log.Println(keepAliveResponse)
	return true, nil
}

// 尝试上锁
func (etcdLock *EtcdLock) TryLock() (err error) {
	// 捕获异常
	defer func() {
		if err := recover(); err != nil {
			log.Println(err)
			return
		}
	}()

	// 1. 定义变量
	var (
		leaseGrantResponse *clientv3.LeaseGrantResponse
		//cancelCtx          context.Context                         // 取消上下文
		cancelFunc context.CancelFunc // 取消函数
		leaseID    clientv3.LeaseID   // 租约ID
		//keepResponseChan   <-chan *clientv3.LeaseKeepAliveResponse // 保持续租的响应channel
		txn         clientv3.Txn          // etcd的事务
		etcdKey     string                // etcd锁的key
		txnResponse *clientv3.TxnResponse // 事务响应
	)

	// 2. 创建租约
	// 2-2: 创建租约-得到租约ID
	// 租约是10秒
	if leaseGrantResponse, err = etcdLock.lease.Grant(context.Background(), 10); err != nil {
		// 创建租约出错
		return err
	} else {
		// 得到租约ID
		leaseID = leaseGrantResponse.ID
	}

	// 2-2: 自动续租
	// 取消自动续租相关的上下文和取消函数
	// cancelCtx, cancelFunc = context.WithCancel(context.Background())
	// keepAlive会持续的发起续租请求，也可以使用keepAliveOnce只发送一次续租请求，后续手工自动续租
	//if keepResponseChan, err = etcdLock.lease.KeepAlive(cancelCtx, leaseID); err != nil {
	//	goto FAIL
	//}

	// 3. 处理自动续租的响应，启动个协程不断的消耗channel中的内容
	go func() {
		var (
			//keepResponse *clientv3.LeaseKeepAliveResponse
			needKill bool
		)

		for {
			select {
			//case keepResponse = <-keepResponseChan:
			//	if keepResponse == nil {
			//		// 特别注意这里要退出for循环，用goto END，别用break
			//		//log.Println("keepResponse的结果是空了")
			//		goto END
			//	} else {
			//		// log.Println(keepResponse)
			//	}
			case needKill = <-etcdLock.NeedKillChan:
				if needKill {
					//log.Println("需要释放锁")
					etcdLock.UnLock()
					goto END
				}

			}
		}

	END:
		// 自动续租完毕
		if etcdLock.IsLocked {
			// 这种情况是：当worker在执行任务，然后etcd挂掉了，或者worker与etcd集群网络不通了
			// 这个时候本程序不应该继续执行了，需要杀掉当前etcdLock对应的程序
			// log.Println("我开始是locked的，现在执行到了end，应该是有问题的")
			etcdLock.IsLocked = false
			// TODO: 这里有可能还需要优化：是否添加个重新续租？抢到就不kill，没抢到再kill
			etcdLock.NeedKillChan <- true
		}
	}()

	// 4. 创建事务txn
	txn = etcdLock.kv.Txn(context.Background())

	// 4-1: 获取锁的key
	etcdKey = EtcdJobsLockDir + etcdLock.Name
	// 抢到这个key就是抢到了锁

	// 4-2: 事务抢锁
	// 如果etcdKey的Revision是0：表示这个key不存在
	txn.If(clientv3.Compare(clientv3.CreateRevision(etcdKey), "=", 0)).
		// key不存在，就设置这个key的值为1，注意携带租约，如果不携带租约，节点宕机了，这个key就一直存在了
		// 把秘钥写入到etcd中，当分布式节点续租的时候需要根据秘钥判断
		Then(clientv3.OpPut(etcdKey, etcdLock.Secret, clientv3.WithLease(leaseID))).
		// 如果这个key存在的话，表示锁被别人抢走了，我们获取一下
		Else(clientv3.OpGet(etcdKey))

	// 4-3: 提交事务
	if txnResponse, err = txn.Commit(); err != nil {
		goto FAIL
	}

	// 4-4：事务结果进行判断
	// 成功返回，失败释放租约
	if !txnResponse.Succeeded {
		// 锁被占用了
		err = fmt.Errorf("锁已被占用:%s", etcdLock.Name)
		goto FAIL
	}

	// 4-5：抢锁成功
	etcdLock.LeaseID = leaseID
	// etcdLock.cancelFunc = cancelFunc
	etcdLock.IsLocked = true
	return nil
FAIL:
	// 当失败了：需要取消自动续租-->执行自动续租的取消函数
	cancelFunc()

	// 释放租约：etcdKey会马上删除
	etcdLock.lease.Revoke(context.Background(), leaseID)
	return
}

// 释放etcd的lock
func (etcdLock *EtcdLock) UnLock() {
	if etcdLock.IsLocked {
		// 设置jobLock.IsLocked为False
		etcdLock.IsLocked = false
		// 取消我们续租的协程
		etcdLock.cancelFunc()

		// 正常退出的程序也需要给它发送一条信息
		etcdLock.NeedKillChan <- false

		// 释放租约：关联的key会自动删除
		etcdLock.lease.Revoke(context.Background(), etcdLock.LeaseID)
	}
}

// 上锁成功后，是否想自动释放
// 比如：上锁成功了，然后需要客户端主动发送续租信息
// 超时后，自动kill续租的相关操作
func (etcdLock *EtcdLock) SetAutoKillTicker(callback func()) {
	for {
		select {
		case <-etcdLock.timer.C:
			// 发送一个需要杀掉的消息
			// log.Println("定时器到了")
			etcdLock.NeedKillChan <- true

			// 调用callback函数
			callback()
			goto END
		}
	}
END:
	return
}

// 重置timer
// 比如：timer：5,4,3,2,1,0
// 假如倒数到2了，你执行一下ResetTimer(10 * time.Second), 立刻变成：10，9，8，7...了
func (etcdLock *EtcdLock) ResetTimer(duration time.Duration, secret string) (err error) {
	if etcdLock.Secret != "" {
		if etcdLock.Secret != secret {
			err = fmt.Errorf("%s的秘钥不匹配", etcdLock.Name)
			return err
		}
	}

	// 判断是否结束了
	// log.Println(etcdLock)
	if !etcdLock.IsLocked {
		// 会有个监听lock的时间，当有lock删除，就会操作这个lock
		err = fmt.Errorf("%s:的状态是false了", etcdLock.Name)
		return
	}

	// log.Println("对lock设置续租:", etcdLock.Name, duration)
	// 时间最多一分钟，最少一秒钟
	if duration > time.Minute {
		duration = time.Minute
	}
	if duration < time.Second {
		duration = time.Second
	}
	etcdLock.ResetTimes++
	etcdLock.timer.Reset(duration)
	return nil
}

// 实例化一个etcdLock
func NewEtcdLock(name string, kv clientv3.KV, lease clientv3.Lease) (etcdLock *EtcdLock, err error) {
	name = strings.TrimSpace(name)
	if name == "" {
		err = errors.New("锁的名字不可为空")
		return nil, err
	}

	// 如果启动了自动kill的通道
	// 那么需要自动设置续租，etcdLock.ResetTimer()
	timer := time.NewTimer(time.Second * 10)

	etcdLock = &EtcdLock{
		CreatedAt:    time.Now(),
		Name:         name,
		kv:           kv,
		lease:        lease,
		LeaseID:      0,
		IsLocked:     false,
		NeedKillChan: make(chan bool),
		ResetTimes:   0,
		timer:        timer,
	}
	return etcdLock, nil
}
