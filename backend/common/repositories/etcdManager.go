package repositories

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/codelieche/cronjob/backend/common/interfaces"

	"github.com/codelieche/cronjob/backend/common"
	"github.com/codelieche/cronjob/backend/common/datasources"

	"github.com/codelieche/cronjob/backend/common/datamodels"

	"github.com/coreos/etcd/clientv3"
)

// Job Manager
// 计划任务的管理器
type EtcdManager struct {
	etcd *datasources.Etcd
}

// 计划任务kill
// 杀掉计划任务运行的进程
func (etcdManager *EtcdManager) KillJob(category string, jobID int) (err error) {
	// 添加要杀掉的Job信息
	// 通过在：/crontab/kill/:name添加一条数据
	// Worker节点，会监听到这个条目的PUT操作，然后做相应的操作

	// 1. 定义变量
	var (
		jobKillKey         string
		killJob            *datamodels.JobKill
		killJobData        []byte
		leaseGrantResponse *clientv3.LeaseGrantResponse
		leaseID            clientv3.LeaseID
		putResponse        *clientv3.PutResponse
		ctx                context.Context
	)

	// 校验key
	category = strings.TrimSpace(category)
	if category == "" {
		category = "default"
	}
	//name = strings.TrimSpace(name)
	//if name == "" {
	//	err = fmt.Errorf("job的name不可为空")
	//	return
	//}

	// jobKillKey = ETCD_JOB_KILL_DIR + name
	jobKillKey = fmt.Sprintf("%s%s/%d", common.ETCD_JOB_KILL_DIR, category, jobID)
	killJob = &datamodels.JobKill{
		Category: category,
		JobID:    uint(jobID),
	}
	// 2. 通知worker杀死对应的任务
	// 2-1: 创建个租约
	ctx, _ = context.WithTimeout(context.TODO(), time.Duration(common.GetConfig().Etcd.Timeout)*time.Millisecond)
	if leaseGrantResponse, err = etcdManager.etcd.Lease.Grant(ctx, 5); err != nil {
		// 创建租约失败
		return
	}
	// 2-2： 得到租约ID
	leaseID = leaseGrantResponse.ID

	// 2-3: 添加kill记录
	if killJobData, err = json.Marshal(killJob); err != nil {
		return nil
	}

	if putResponse, err = etcdManager.etcd.KV.Put(
		context.TODO(),
		jobKillKey, string(killJobData),
		clientv3.WithLease(leaseID),
	); err != nil {
		return
	} else {
		// put成功
		putResponse = putResponse
		// log.Println(putResponse.Header.Revision)
	}

	return
}

// Watch keys
// 监听etcd key的变化: 比如监听jobs的变化，和监听kill的任务
// 传递的参数：要监听的key的前缀，和处理监听的接口
func (etcdManager *EtcdManager) WatchKeys(keyDir string, watchHandler interfaces.WatchHandler) (err error) {
	// 1. 定义变量
	var (
		getResponse *clientv3.GetResponse
		//kvPair             *mvccpb.KeyValue
		//job                *Job
		watchStartRevision int64
		watchChan          clientv3.WatchChan
		//watchResponse      clientv3.WatchResponse
		//watchEvent         *clientv3.Event
		ctx context.Context
	)

	// 2. get：/crontab/jobs/目录下的所有任务，并且获知当前集群的revision
	//keyDir = "/crontab/jobs/"
	ctx, _ = context.WithTimeout(context.TODO(), time.Duration(common.GetConfig().Etcd.Timeout)*time.Millisecond)
	if getResponse, err = etcdManager.etcd.KV.Get(
		ctx, keyDir,
		clientv3.WithPrefix(),
	); err != nil {
		log.Println("执行watchKeys出错：", err)
		os.Exit(1)
		//return
	}

	// 3. HandlerGetResponse(getResponse *clientv3.GetResponse)
	watchHandler.HandlerGetResponse(getResponse)

	// 4. watch新的变化
	func() { // 监听协程
		// 4-1: 从GET时刻后续版本开始监听
		watchStartRevision = getResponse.Header.Revision + 1
		log.Printf("开始watch事件:%s(Revision:%d)", keyDir, getResponse.Header.Revision)

		//	4-2：监听:/crontab/jobs/目录后续的变化
		watchChan = etcdManager.etcd.Watcher.Watch(
			context.TODO(),
			keyDir,
			clientv3.WithPrefix(),                // 监听以jobKeyDir为前缀的key
			clientv3.WithRev(watchStartRevision), // 设置开始的版本号
			clientv3.WithPrevKV(),                // 如果不需知道上一次的值，可不添加这个option
		)

		// 4-3: 处理监听事件的Channel
		watchHandler.HandlerWatchChan(watchChan)

	}()
	return
}

// 创建任务执行锁
func (etcdManager *EtcdManager) CreateJobLock(name string) (jobLock *common.JobLock) {
	// 返回一把锁
	jobLock = common.NewJobLock(name, etcdManager.etcd.KV, etcdManager.etcd.Lease)
	return
}

// 实例化Job Manager
func NewEtcdManager(etcdConfig *common.EtcdConfig) (*EtcdManager, error) {
	var (
		etcd        *datasources.Etcd
		etcdManager *EtcdManager
	)

	// log.Println(etcdConfig.TLS)
	etcd = datasources.GetEtcd()

	//	实例化Job Manager
	etcdManager = &EtcdManager{
		etcd: etcd,
	}

	// 返回
	return etcdManager, nil
}
