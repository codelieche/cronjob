package common

import (
	"context"

	"go.etcd.io/etcd/clientv3"
)

// 分布式锁
type JobLock struct {
	name       string             // 任务名称
	kv         clientv3.KV        // KV
	lease      clientv3.Lease     // 租约
	cancelFunc context.CancelFunc // 取消函数：用于取消自动续租
	leaseId    clientv3.LeaseID   // 租约ID
	isLocked   bool               // 释放上锁成功，也可根据leaseId是否不为0判断
}

// 初始化一把锁
func NewJobLock(name string, kv clientv3.KV, lease clientv3.Lease) (jobLock *JobLock) {
	jobLock = &JobLock{
		name:  name,
		kv:    kv,
		lease: lease,
	}
	return
}

// 尝试上锁
func (jobLock *JobLock) TryLock() (err error) {
	var (
		leaseGrantResponse *clientv3.LeaseGrantResponse
		cancelCtx          context.Context                         // 取消上下文
		cancelFunc         context.CancelFunc                      // 取消函数
		leaseId            clientv3.LeaseID                        // 租约ID
		keepResponseChan   <-chan *clientv3.LeaseKeepAliveResponse // 保持续租的响应channel
		txn                clientv3.Txn                            // 事务
		lockKey            string                                  // 锁的路径
		txnResponse        *clientv3.TxnResponse                   // 事务响应
	)
	// 1. 创建租约(5秒)
	if leaseGrantResponse, err = jobLock.lease.Grant(context.TODO(), 5); err != nil {
		return
	}

	// 2. 自动续租
	// 取消自动续租相关的上下文和取消函数
	cancelCtx, cancelFunc = context.WithCancel(context.TODO())
	// 租约ID
	leaseId = leaseGrantResponse.ID

	// 自动续租
	if keepResponseChan, err = jobLock.lease.KeepAlive(cancelCtx, leaseId); err != nil {
		goto FAIL
	}

	// 3. 处理租约响应的协程
	go func() {
		var (
			keepResponse *clientv3.LeaseKeepAliveResponse
		)
		for {
			select {
			case keepResponse = <-keepResponseChan:
				if keepResponse == nil {
					// 特别注意这里要退出for循环，用goto END, 别用break
					goto END
				} else {
					//log.Println(keepResponse)
				}
			}
		}
	END:
		// 自动续租完毕
	}()

	// 4. 创建事务txn
	txn = jobLock.kv.Txn(context.TODO())

	// 锁路径
	lockKey = ETCD_JOBS_LOCK_DIR + jobLock.name
	// 抢到这个key就是抢到了锁

	// 4. 事务抢锁
	// 如果lockKey的Revision是0：表示这个key不存在
	txn.If(clientv3.Compare(clientv3.CreateRevision(lockKey), "=", 0)).
		// key不存在，就设置这个key的值为1，注意携带租约，如果不携带租约，节点宕机了，这个key就一直存在了
		Then(clientv3.OpPut(lockKey, "1", clientv3.WithLease(leaseId))).
		// 如果这个key存在的话，表示锁被别人抢走了，我们获取一下
		Else(clientv3.OpGet(lockKey))

	// 提交事务
	if txnResponse, err = txn.Commit(); err != nil {
		goto FAIL
	}

	// 6. 成功返回，失败释放租约
	if !txnResponse.Succeeded {
		// 锁被占用了
		err = LOCK_IS_USING
		goto FAIL
	}

	// 抢锁成功
	jobLock.leaseId = leaseId
	jobLock.cancelFunc = cancelFunc
	jobLock.isLocked = true
	return
FAIL:
	// 取消自动续租
	cancelFunc()

	// 释放租约,key马上会删掉
	jobLock.lease.Revoke(context.TODO(), leaseId)
	return
}

func (jobLock *JobLock) Unlock() {

	if jobLock.isLocked {
		// 取消我们续租的协程
		jobLock.cancelFunc()

		// 释放租约：关联的key会自动删除
		jobLock.lease.Revoke(context.TODO(), jobLock.leaseId)
	}

}
