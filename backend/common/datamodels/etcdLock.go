package datamodels

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/coreos/etcd/clientv3"
)

var EtcdJobsLockDir = "/cronjob/lock/"

// etcd分布式锁
// 计划任务执行的时候需要锁：/crontab/lock/jobs/:category/:name
// client --> server:
// TryLocker: 申请锁，判断是否成功
type EtcdLock struct {
	Name         string             // 锁的名字，eg：jobs/:category/:name
	kv           clientv3.KV        // KV
	lease        clientv3.Lease     // 租约
	cancelFunc   context.CancelFunc // 取消函数：用于取消自动续租
	leaseID      clientv3.LeaseID   // 租约ID
	IsLocked     bool               // 释放上锁成功，也可根据leaseID释放不为0判断
	NeedKillChan chan bool          // 是否需要杀掉当前上锁的程序
}

// 尝试上锁
func (etcdLock *EtcdLock) TryLock() (err error) {
	// 1. 定义变量
	var (
		leaseGrantResponse *clientv3.LeaseGrantResponse
		cancelCtx          context.Context                         // 取消上下文
		cancelFunc         context.CancelFunc                      // 取消函数
		leaseID            clientv3.LeaseID                        // 租约ID
		keepResponseChan   <-chan *clientv3.LeaseKeepAliveResponse // 保持续租的响应channel
		txn                clientv3.Txn                            // etcd的事务
		etcdKey            string                                  // etcd锁的key
		txnResponse        *clientv3.TxnResponse                   // 事务响应
	)

	// 2. 创建租约
	// 2-2: 创建租约-得到租约ID
	if leaseGrantResponse, err = etcdLock.lease.Grant(context.Background(), 5); err != nil {
		// 创建租约出错
		return err
	} else {
		// 得到租约ID
		leaseID = leaseGrantResponse.ID
	}

	// 2-2: 自动续租
	// 取消自动续租相关的上下文和取消函数
	cancelCtx, cancelFunc = context.WithCancel(context.Background())
	if keepResponseChan, err = etcdLock.lease.KeepAlive(cancelCtx, leaseID); err != nil {
		goto FAIL
	}

	// 3. 处理自动续租的响应，启动个协程不断的消耗channel中的内容
	go func() {
		var (
			keepResponse *clientv3.LeaseKeepAliveResponse
		)

		for {
			select {
			case keepResponse = <-keepResponseChan:
				if keepResponse == nil {
					// 特别注意这里要退出for循环，用goto END，别用break
					goto END
				} else {
					//log.Println(keepResponse)
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
		Then(clientv3.OpPut(etcdKey, "1", clientv3.WithLease(leaseID))).
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
	etcdLock.leaseID = leaseID
	etcdLock.cancelFunc = cancelFunc
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
		etcdLock.lease.Revoke(context.Background(), etcdLock.leaseID)
	}
}

// 实例化一个etcdLock
func NewEtcdLock(name string, kv clientv3.KV, lease clientv3.Lease) (etcdLock *EtcdLock, err error) {
	name = strings.TrimSpace(name)
	if name == "" {
		err = errors.New("锁的名字不可为空")
		return nil, err
	}
	etcdLock = &EtcdLock{
		Name:         name,
		kv:           kv,
		lease:        lease,
		leaseID:      0,
		IsLocked:     false,
		NeedKillChan: make(chan bool),
	}
	return etcdLock, nil
}
