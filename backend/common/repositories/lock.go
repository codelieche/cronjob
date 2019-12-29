package repositories

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/codelieche/cronjob/backend/common"

	"github.com/codelieche/cronjob/backend/common/datamodels"
	"github.com/codelieche/cronjob/backend/common/datasources"
	"github.com/coreos/etcd/clientv3"
)

// Lock Repository
type LockRepository interface {
	// 创建、抢锁
	Create(name string, ttl int64) (lock *datamodels.Lock, err error)
	// 对锁续租
	Lease(leaseID int64) error
	// 删除锁: 释放租约
	Release(leaseID int64) error
}

// 实例化LockRepository
func NewLockRepository(etcd *datasources.Etcd) *lockRepository {
	return &lockRepository{etcd: etcd}
}

// lock repository
type lockRepository struct {
	// 通过etcd创建锁
	etcd *datasources.Etcd
}

// 创建或者抢锁
func (r *lockRepository) Create(name string, ttl int64) (lock *datamodels.Lock, err error) {
	// 1. 变量处理
	// 1-1： 定义变量
	var (
		leaseGrantResponse *clientv3.LeaseGrantResponse
		leaseID            clientv3.LeaseID      // 租约ID
		txn                clientv3.Txn          // etcd的事务
		etcdKey            string                // etcd锁的key
		password           string                // 锁的密码
		txnResponse        *clientv3.TxnResponse // 事务响应
	)

	// 1-2：对name、ttl进行校验
	name = strings.TrimSpace(name)
	if name == "" {
		err = errors.New("名字不可设置为空")
		return nil, err
	}

	if ttl < 1 {
		ttl = 10
	}
	if ttl > 300 {
		ttl = 300
	}
	// 1-3：随机生成个密码
	password = strconv.Itoa(int(time.Now().UnixNano()))

	// 2. 创建租约
	// 2-2: 创建租约-得到租约ID
	// 租约是10秒
	if leaseGrantResponse, err = r.etcd.Lease.Grant(context.Background(), ttl); err != nil {
		// 创建租约出错
		return nil, err
	} else {
		// 得到租约ID
		leaseID = leaseGrantResponse.ID
	}

	// 2-2: 自动续租
	// 可用lease.KeepAlive(cancelCtx, leaseID)不断的续租，但是我们这里需要自己手动发起续租请求
	// 通过这里创建的锁，需要自己发起：KeepAliveOnce(ctx, leaseID)续租

	// 3. 创建事务txn
	txn = r.etcd.KV.Txn(context.Background())

	// 3-1: 获取锁的key
	etcdKey = common.ETCD_JOBS_LOCK_DIR + name
	// 抢到这个key就是抢到了锁
	lock = &datamodels.Lock{
		Name:     etcdKey,
		TTL:      ttl,
		Password: password,
		LeaseID:  int64(leaseID),
		IsActive: false,
	}

	// 3-2: 事务抢锁
	// 如果etcdKey的Revision是0：表示这个key不存在
	txn.If(clientv3.Compare(clientv3.CreateRevision(etcdKey), "=", 0)).
		// key不存在，就设置这个key的值为1，注意携带租约，如果不携带租约，节点宕机了，这个key就一直存在了
		// 把秘钥写入到etcd中，当分布式节点续租的时候需要根据秘钥判断
		Then(clientv3.OpPut(etcdKey, password, clientv3.WithLease(leaseID))).
		// 如果这个key存在的话，表示锁被别人抢走了，我们获取一下
		Else(clientv3.OpGet(etcdKey))

	// 3-3: 提交事务
	if txnResponse, err = txn.Commit(); err != nil {
		goto FAIL
	}

	// 3-4：事务结果进行判断
	// 成功返回，失败释放租约
	if !txnResponse.Succeeded {
		// 锁被占用了
		err = fmt.Errorf("锁已被占用:%s", lock.Name)
		goto FAIL
	}

	// 3-5：抢锁成功
	// etcdLock.cancelFunc = cancelFunc
	lock.IsActive = true
	return lock, nil
FAIL:

	// 4: 抢锁失败
	// 即使没抢到锁，租约也可释放一下
	// 释放租约：etcdKey会马上删除
	r.etcd.Lease.Revoke(context.Background(), leaseID)
	return nil, err
}

// 对锁进行续租
func (r *lockRepository) Lease(leaseID int64) (err error) {
	// 1. 定义变量
	var (
		keepAliveResponse *clientv3.LeaseKeepAliveResponse
		ctx               context.Context
	)

	if leaseID <= 0 {
		err = errors.New("租约ID小于0")
		return err
	}

	// 2. 发起续租请求
	// 2-1: 准备上下文，设置个超时2秒
	ctx, _ = context.WithTimeout(context.Background(), time.Second*2)

	if keepAliveResponse, err = r.etcd.Lease.KeepAliveOnce(ctx, clientv3.LeaseID(leaseID)); err != nil {
		return err
	} else {
		keepAliveResponse = keepAliveResponse
		// log.Println(keepAliveResponse)
		// 续租成功
		return nil
	}
}

// 释放锁
// err是nil就表示释放租约成功
func (r *lockRepository) Release(leaseID int64) (err error) {
	// 1. 定义变量
	var (
		leaseRevokeResponse *clientv3.LeaseRevokeResponse
		ctx                 context.Context
	)

	if leaseID <= 0 {
		err = errors.New("租约ID小于0")
		return err
	}

	// 2. 释放租约
	// 2-1: 准备上下文，设置个超时2秒
	ctx, _ = context.WithTimeout(context.Background(), time.Second*2)

	// 2-2：发起释放租约请求
	if leaseRevokeResponse, err = r.etcd.Lease.Revoke(ctx, clientv3.LeaseID(leaseID)); err != nil {
		return err
	} else {
		leaseRevokeResponse = leaseRevokeResponse
		// log.Println(leaseRevokeResponse)
		return nil
	}
}
