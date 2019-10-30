package common

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"go.etcd.io/etcd/pkg/transport"

	"go.etcd.io/etcd/mvcc/mvccpb"

	"go.etcd.io/etcd/clientv3"
)

// 保存Job到etcd中
// 返回上一次的Job和错误信息
func (etcdManager *EtcdManager) SaveJob(job *Job, isCreate bool) (prevJob *Job, err error) {
	// 把任务保存到/crontab/jobs/:name中
	// isCreate是否是创建：
	// 创建的时候：需要判断job是否存在，存在就报错
	// 不是创建，那么就是更新，需要判断是否存在，不存在就报错
	var (
		jobsDir  string
		jobKey   string
		jobValue []byte

		putResponse *clientv3.PutResponse
	)

	// 先处理下Job在etcd中的key
	if job.Name == "" {
		if isCreate {
			// name为空就自动生成一个
			if name, err := generateName(); err != nil {
				timeNowUnix := time.Now().UnixNano()
				job.Name = strconv.Itoa(int(timeNowUnix))
			} else {
				job.Name = name
			}

		} else {
			// 更新操作也是需要判断名称的
			err = fmt.Errorf("Job的Name不能为空")
			return nil, err
		}
	} else {
		// 如果是更新需要判断name是否可key匹配：
		// 这里交给http接口来处理
	}

	// 如果job的分类为空，就设置其为default
	if job.Category == "" {
		job.Category = "default"
	}

	// 检查category是否存在
	if category, err := etcdManager.GetCategory(job.Category); err != nil {
		//log.Println(err)
		if err == NOT_FOUND && job.Category == "default" {
			// 创建默认：
			c := Category{
				Name:        "default",
				IsActive:    true,
				CheckCmd:    "which bash",
				SetupCmd:    "echo `date`",
				TearDownCmd: "echo `date`",
			}

			if category, err = etcdManager.SaveCategory(&c, true); err != nil {
				return nil, err
			} else {
				log.Println("创建默认分类：", category)
			}
		} else {
			msg := fmt.Sprintf("分类%s不存在，请先创建相关分类", job.Category)
			err = errors.New(msg)
			return nil, err
		}
	} else {
		//log.Println(category)
	}

	// jobKey = ETCD_JOBS_DIR + job.Name
	jobsDir = ETCD_JOBS_DIR
	if strings.HasSuffix(jobsDir, "/") {
		jobsDir = string(jobsDir[:len(jobsDir)-1])
	}

	// 组合jobKey
	jobKey = fmt.Sprintf("%s/%s/%s", jobsDir, job.Category, job.Name)

	// 判断Job是否已经存在了
	// TODO：这里应该加个锁，抢到锁才创建，要不大量频繁创建，可能会造成name重复：
	if prevJob, err = etcdManager.GetJob(jobKey); err != nil {
		if isCreate {
			if err != NOT_FOUND {
				return nil, err
			} else {
				// 返回的是说不存在，那么可以创建
			}
		} else {
			// 更新操作：只要报错就返回
			return nil, err
		}

	} else {
		if isCreate {
			// 是创建：如果存在就返回
			err = fmt.Errorf("%s已经存在，不可创建", jobKey)
			return nil, err
		} else {
			// 更新操作：获取到，可以执行后续的更新操作
		}

	}

	// 任务信息json：对job序列化一下
	// 把自身的key也加入到job中
	job.Key = jobKey
	if jobValue, err = json.Marshal(job); err != nil {
		return nil, err
	}

	//	保存到etcd中
	if putResponse, err = etcdManager.kv.Put(
		context.TODO(),        // 上下文
		jobKey,                // Key
		string(jobValue),      // 值
		clientv3.WithPrevKV(), // 返回上一个版本的值
	); err != nil {
		return nil, err
	}

	// 如果是更新，那么返回上一个版本的job
	if putResponse.PrevKv != nil {
		//	对旧值反序列化下
		if err = json.Unmarshal(putResponse.PrevKv.Value, &prevJob); err != nil {
			log.Println(err.Error())
			// 这里虽然反序列化出错了，但是不影响保存的操作，这里我们可以把err设置为空
			return nil, nil
		} else {
			// 返回上一个的旧值
			prevJob.Key = jobKey
			return prevJob, err
		}
	} else {
		// 没有上一个的job值，直接返回
		return nil, nil
	}
}

// List Jobs
// prevLastKey是上次查询的最后一条key
// limit是想查询的数量
func (etcdManager *EtcdManager) ListJobs(page int, pageSize int) (jobList []*Job, err error) {
	// 定义变量
	var (
		prevLastKeyCreateRevision int64 // 分页的时候上一页的：kvParir.CreateRevision
		jobsDirKey                string
		getResponse               *clientv3.GetResponse
		kvPair                    *mvccpb.KeyValue
		job                       *Job
		ctx                       context.Context
		needDropPrevLastKey       bool
		count                     int
		limit                     int
	)
	// 想通过page + pageSize 计算prevLastKey的值
	// 这样的话用户只需要访问：jobs/list?page=5&pageSize=10 这种方式获取了
	jobsDirKey = ETCD_JOBS_DIR

	if pageSize > 100 {
		pageSize = 100
	}
	if pageSize < 0 {
		pageSize = 10
	}

	// 获取job对象
	//endKey := "/crontab/jobs/test2"
	//jobsDirKey = endKey

	ctx, _ = context.WithTimeout(context.TODO(), time.Duration(Config.Master.Etcd.Timeout)*time.Millisecond)

	if page > 1 {
		// 这种传page的方式也许不优，比如当数据量大了之后，查询prevLastKeyCreateRevision要点时间
		// 推荐继续兼容：传递prevLastKey的方式
		// 计算：prevLastKey，这样可快速的得到prevLastKeyCreateRevision
		limit = (page - 1) * pageSize
		if getResponse, err = etcdManager.kv.Get(
			ctx, jobsDirKey,
			clientv3.WithFromKey(),
			clientv3.WithKeysOnly(),
			clientv3.WithLimit(int64(limit)),
			clientv3.WithSort(clientv3.SortByCreateRevision, clientv3.SortAscend),
		); err != nil {
			return nil, err
		} else {
			if len(getResponse.Kvs) != limit {
				// 超过了范围了
				return nil, nil
			} else {
				kvP := getResponse.Kvs[len(getResponse.Kvs)-1]
				// 后面会根据这个来做分页
				prevLastKeyCreateRevision = kvP.CreateRevision
			}
		}
	}

	// jobsDirKey = "/crontab/jobs/default/test5"
	if prevLastKeyCreateRevision == 0 {
		//prevLastKey = jobsDirKey
		limit = pageSize
	} else {
		// 需要去除prevLastKey
		needDropPrevLastKey = true
		limit = pageSize + 1
	}

	// clientv3.WithFromKey() 会从传入的key开始获取，不可与WithPrefix同时使用
	if getResponse, err = etcdManager.kv.Get(
		ctx,
		jobsDirKey,
		clientv3.WithPrefix(),
		//clientv3.WithFromKey(),
		clientv3.WithMinCreateRev(prevLastKeyCreateRevision),
		clientv3.WithLimit(int64(limit)),
		clientv3.WithSort(clientv3.SortByCreateRevision, clientv3.SortAscend),
		//clientv3.WithSort(clientv3.SortByCreateRevision, clientv3.SortAscend),
	); err != nil {
		// 获取列表出错，直接返回
		return
	}

	// 变量kv
	for _, kvPair = range getResponse.Kvs {
		// log.Println(string(kvPair.Key), kvPair.ModRevision, kvPair.CreateRevision, kvPair.Version)
		count += 1
		if needDropPrevLastKey {
			if count > pageSize {
				// 不可大于pageSize条数据
				break
			}
			if kvPair.CreateRevision == prevLastKeyCreateRevision {
				// 需要过滤这条
				needDropPrevLastKey = false
				continue
			} else {
				// 不相等
				// 如果count=1 直接返回
				if count == 1 {
					return jobList, nil
				}
			}
		}

		//	对值序列化
		job = &Job{}
		if err = json.Unmarshal(kvPair.Value, job); err != nil {
			continue
		} else {
			// 如果job未保存key，那么就添加一下
			if job.Key == "" {
				job.Key = string(kvPair.Key)
			}
			jobList = append(jobList, job)
		}
	}

	//	返回结果
	return jobList, nil

}

// 获取Job的Detail
func (etcdManager *EtcdManager) GetJob(jobKey string) (job *Job, err error) {
	// 定义变量
	var (
		getResponse *clientv3.GetResponse
		keyValue    *mvccpb.KeyValue
		i           int
		ctx         context.Context
	)
	// 1. 对jobKey做校验
	jobKey = strings.TrimSpace(jobKey)
	if jobKey == "" {
		err = fmt.Errorf("传入的jobKey为空")
		return nil, err
	}
	if !strings.HasPrefix(jobKey, ETCD_JOBS_DIR) {
		err = errors.New("传入的key不正确")
		return nil, err
	}

	// 2. 从etcd中获取对象
	jobKey = strings.Replace(jobKey, "//", "/", -1)
	ctx, _ = context.WithTimeout(context.TODO(), time.Duration(Config.Master.Etcd.Timeout)*time.Millisecond)
	if getResponse, err = etcdManager.kv.Get(ctx, jobKey); err != nil {
		return nil, err
	}

	// 3. 获取kv对象
	//log.Println(getResponse.Header)
	//log.Println(getResponse.Kvs)
	if len(getResponse.Kvs) == 1 {
		for i = range getResponse.Kvs {
			keyValue = getResponse.Kvs[i]
			//log.Println(keyValue.Value)
			//	4. json反序列化
			if err = json.Unmarshal(keyValue.Value, &job); err != nil {
				return nil, err
			} else {
				job.Key = jobKey
				return job, nil
			}
		}
		goto NotFound
	} else {
		goto NotFound
	}

NotFound:
	err = NOT_FOUND
	return nil, err
}

// Delete Job
func (etcdManager *EtcdManager) DeleteJob(jobKey string) (success bool, err error) {
	// 定义变量
	var (
		deleteResponse *clientv3.DeleteResponse
		ctx            context.Context
	)
	// 1. 对key做判断
	jobKey = strings.TrimSpace(jobKey)
	if !strings.HasPrefix(jobKey, ETCD_JOBS_DIR) {
		err = errors.New("传入的key不正确")
		return false, err
	}

	if jobKey == "" {
		err = fmt.Errorf("jobKey不可为空")
		return false, err
	}

	// 2. 操作删除
	jobKey = strings.Replace(jobKey, "//", "/", -1)
	ctx, _ = context.WithTimeout(context.TODO(), time.Duration(Config.Master.Etcd.Timeout)*time.Millisecond)
	if deleteResponse, err = etcdManager.kv.Delete(
		ctx,
		jobKey,
		clientv3.WithPrevKV(),
	); err != nil {
		return false, err
	}

	// 3. 返回被删除的keyValue
	if len(deleteResponse.PrevKvs) < 1 {
		err = fmt.Errorf("%s不存在", jobKey)
		return false, err
	} else {
		// 删除成功
		return true, nil
	}
}

// 计划任务kill
// 杀掉计划任务运行的进程
func (etcdManager *EtcdManager) KillJob(category, name string) (err error) {
	// 添加要杀掉的Job信息
	// 通过在：/crontab/kill/:name添加一条数据
	// Worker节点，会监听到这个条目的PUT操作，然后做相应的操作

	// 1. 定义变量
	var (
		jobKillKey         string
		killJob            *KillJob
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
	name = strings.TrimSpace(name)
	if name == "" {
		err = fmt.Errorf("job的name不可为空")
		return
	}

	// jobKillKey = ETCD_JOB_KILL_DIR + name
	jobKillKey = fmt.Sprintf("%s%s/%s", ETCD_JOB_KILL_DIR, category, name)
	killJob = &KillJob{
		Category: category,
		Name:     name,
	}
	// 2. 通知worker杀死对应的任务
	// 2-1: 创建个租约
	ctx, _ = context.WithTimeout(context.TODO(), time.Duration(Config.Master.Etcd.Timeout)*time.Millisecond)
	if leaseGrantResponse, err = etcdManager.lease.Grant(ctx, 5); err != nil {
		// 创建租约失败
		return
	}
	// 2-2： 得到租约ID
	leaseID = leaseGrantResponse.ID

	// 2-3: 添加kill记录
	if killJobData, err = json.Marshal(killJob); err != nil {
		return nil
	}

	if putResponse, err = etcdManager.kv.Put(
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
func (etcdManager *EtcdManager) WatchKeys(keyDir string, watchHandler WatchHandler) (err error) {
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
	ctx, _ = context.WithTimeout(context.TODO(), time.Duration(Config.Master.Etcd.Timeout)*time.Millisecond)
	if getResponse, err = etcdManager.kv.Get(
		ctx, keyDir,
		clientv3.WithPrefix(),
	); err != nil {
		log.Println("执行watchKeys出错：", err)
		return
	}

	// 3. HandlerGetResponse(getResponse *clientv3.GetResponse)
	watchHandler.HandlerGetResponse(getResponse)

	// 4. watch新的变化
	func() { // 监听协程
		// 4-1: 从GET时刻后续版本开始监听
		watchStartRevision = getResponse.Header.Revision + 1
		log.Printf("开始watch事件:%s(Revision:%d)", keyDir, getResponse.Header.Revision)

		//	4-2：监听:/crontab/jobs/目录后续的变化
		watchChan = etcdManager.watcher.Watch(
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
func (etcdManager *EtcdManager) CreateJobLock(name string) (jobLock *JobLock) {
	// 返回一把锁
	jobLock = NewJobLock(name, etcdManager.kv, etcdManager.lease)
	return
}

// 实例化Job Manager
func NewEtcdManager(etcdConfig *EtcdConfig) (*EtcdManager, error) {
	var (
		config      clientv3.Config
		client      *clientv3.Client
		kv          clientv3.KV
		lease       clientv3.Lease
		watcher     clientv3.Watcher
		err         error
		etcdManager *EtcdManager
		tlsInfo     transport.TLSInfo
		tlsConfig   *tls.Config
	)

	// log.Println(etcdConfig.TLS)

	if etcdConfig.TLS != nil {
		// 检查其三个字段是否为空
		if etcdConfig.TLS.CertFile == "" || etcdConfig.TLS.KeyFile == "" || etcdConfig.TLS.CaFile == "" {
			log.Println(etcdConfig.TLS)
			err = errors.New("传入的TLS配置不可为空")
			return nil, err
		} else {
			tlsInfo = transport.TLSInfo{
				CertFile:      etcdConfig.TLS.CertFile,
				KeyFile:       etcdConfig.TLS.KeyFile,
				TrustedCAFile: etcdConfig.TLS.CaFile,
			}
			if tlsConfig, err = tlsInfo.ClientConfig(); err != nil {
				return nil, err
			}
		}
	}

	//	初始化etcd配置
	config = clientv3.Config{
		//Endpoints:   []string{"127.0.0.1:2379"}, // 集群地址
		Endpoints:   etcdConfig.Endpoints,    // 集群地址
		DialTimeout: 5000 * time.Microsecond, // 连接超时
		TLS:         tlsConfig,
	}

	// 建立连接
	if client, err = clientv3.New(config); err != nil {
		return nil, err
	} else {
		// 连接成功
	}

	// 得到KV的Lease的API子集
	kv = clientv3.NewKV(client)
	lease = clientv3.NewLease(client)
	watcher = clientv3.NewWatcher(client)

	//	实例化Job Manager
	etcdManager = &EtcdManager{
		client:  client,
		kv:      kv,
		lease:   lease,
		watcher: watcher,
	}

	// 返回
	return etcdManager, nil
}
