package repositories

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/coreos/etcd/mvcc/mvccpb"

	"github.com/codelieche/cronjob/backend/common"

	"github.com/codelieche/cronjob/backend/common/datamodels"
	"github.com/coreos/etcd/clientv3"
)

// 保存Job到Etcd中
func (r *jobRepository) saveJobToEtcd(job *datamodels.Job, isCreate bool) (prevJob *datamodels.Job, err error) {
	// 把任务保存到：/crontab/jobs/:id中
	// isCreate是否是创建
	// 创建的时候：需要判断job是否存在，存在就报错
	// 不是创建，那么就是更新，需要判断是否存在，不存在就报错
	var (
		jobsEtcdDir  string // job在etcd中的上级目录
		jobEtcdKey   string // job在etcd中的key
		jobEtcdValue []byte // job在etcd中的value
		putResponse  *clientv3.PutResponse
	)

	// 先处理下job的etcd中的key
	if job.Name == "" {
		err = fmt.Errorf("Job的Name不能为空")
		return nil, err
	}

	// 对job的分类进行校验
	if job.Category == nil {

		if job, err = r.GetWithCategory(int64(job.ID)); err != nil {
			return nil, err
		} else {
			// 获取到job
			log.Println(job.Category)
		}
	}
	if job.Category == nil || job.Category.Name == "" || job.Category.ID == 0 {
		err = errors.New("job的分类为空")
		return nil, err
	}

	// 如果job的分类为空，就设置其未default
	if job.CategoryID <= 0 {
		// 设置Job的Category
		if category, err := r.getOrCreateDefaultCategory(); err != nil {
			return nil, err
		} else {
			job.Category = category
			r.Save(job)
		}
	}

	// 检查category是否存在, 放在Save中做

	// 获取job存储的key
	jobsEtcdDir = common.ETCD_JOBS_DIR
	if strings.HasSuffix(jobsEtcdDir, "/") {
		jobsEtcdDir = string(jobsEtcdDir[:len(jobsEtcdDir)-1])
	}

	jobEtcdKey = fmt.Sprintf("%s/%s/%d", jobsEtcdDir, job.Category.Name, job.ID)

	// 判断job是否已经存在了
	// TOOD: 这里应该加个锁，抢到锁才创建，要不大量频繁创建，可能会造成name重复
	if prevJob, err = r.getJobFromEtcd(job.Category.Name, job.ID); err != nil {
		if err == common.NotFountError {
			// 为找到Job
			if !isCreate {
				// 更新操作，必须存在
				return nil, err
			}
		}
	} else {
		// 找到了
		if isCreate {
			// 由于是创建：存在的话，报错
			err = fmt.Errorf("job:%s已经存在于etcd中，不可创建", jobEtcdKey)
			return prevJob, err
		} else {
			// 更新操作：在调用的地方记得校验name
		}
	}

	// 对job反序列化
	if jobEtcdValue, err = json.Marshal(job); err != nil {
		return nil, err
	}

	// 保存数据到etcd中
	log.Println(jobEtcdKey, jobEtcdValue)
	if putResponse, err = r.etcd.KV.Put(
		context.TODO(),        // 上下文
		jobEtcdKey,            // key
		string(jobEtcdValue),  // 值
		clientv3.WithPrevKV(), // 返回上一个版本的值
	); err != nil {
		return nil, err
	} else {
		// 插入成功
	}

	// 返回
	// 如果是更新，那么返回上一个版本的job
	if putResponse.PrevKv != nil {
		// 对旧值反序列化
		if err = json.Unmarshal(putResponse.PrevKv.Value, &prevJob); err != nil {
			log.Println(err)
			// 这里虽然反序列化出错了，但是不影响保存的操作，这里我们可以把err设置为空
			return nil, nil
		} else {
			// 返回上一个就的值
			return prevJob, nil
		}
	} else {
		// 没有上一个job的值
		return nil, nil
	}
}

func (r *jobRepository) getJobFromEtcd(categoryName string, id uint) (job *datamodels.Job, err error) {
	// 1. 定义变量
	var (
		jobEtcdKey  string
		getResponse *clientv3.GetResponse
		keyValue    *mvccpb.KeyValue
		i           int
	)

	// 2. 对key做校验
	categoryName = strings.TrimSpace(categoryName)

	jobEtcdKey = fmt.Sprintf("%s%s/%d", common.ETCD_JOBS_DIR, categoryName, id)
	if categoryName == "" {
		err = fmt.Errorf("传入的分类不可为空")
		return nil, err
	}
	if id <= 0 {
		err = fmt.Errorf("传入的ID不可小于1")
		return nil, err
	}

	// 3. 从etcd中获取对象
	if getResponse, err = r.etcd.KV.Get(context.TODO(), jobEtcdKey); err != nil {
		return nil, err
	}

	// 4. 获取kv对象
	//log.Println(getResponse.Header)
	//log.Println(getResponse.Kvs)

	if len(getResponse.Kvs) == 1 {
		// 4-1: 获取etcd中的value
		for i = range getResponse.Kvs {
			keyValue = getResponse.Kvs[i]
			//log.Println(keyValue.Value)
			// 4-2: json反序列化
			job = &datamodels.Job{}
			if err = json.Unmarshal(keyValue.Value, job); err != nil {
				log.Println("获取Job反序列化出错：", err)
				return nil, err
			} else {
				job.EtcdKey = jobEtcdKey
				return job, nil
			}
		}
		return nil, nil
	} else {
		return nil, common.NotFountError
	}
}

// 从etcd中获取job的列表
func (r *jobRepository) listJobsFromEtcd(page int, pageSize int) (jobs []*datamodels.Job, err error) {
	// 定义变量
	//log.Println(page, pageSize)
	var (
		prevLastKeyCreateRevision int64 // 分页的时候上一页的：kvParir.CreateRevision
		jobsDirKey                string
		getResponse               *clientv3.GetResponse
		kvPair                    *mvccpb.KeyValue
		job                       *datamodels.Job
		ctx                       context.Context
		needDropPrevLastKey       bool
		count                     int
		limit                     int
	)
	// 想通过page + pageSize 计算prevLastKey的值
	// 这样的话用户只需要访问：jobs/list?page=5&pageSize=10 这种方式获取了
	jobsDirKey = common.ETCD_JOBS_DIR

	if pageSize > 100 {
		pageSize = 100
	}
	if pageSize < 0 {
		pageSize = 10
	}

	// 获取job对象
	//endKey := "/crontab/jobs/test2"
	//jobsDirKey = endKey

	config := common.Config
	ctx, _ = context.WithTimeout(context.TODO(), time.Duration(config.Master.Etcd.Timeout)*time.Millisecond)

	if page > 1 {
		// 这种传page的方式也许不优，比如当数据量大了之后，查询prevLastKeyCreateRevision要点时间
		// 推荐继续兼容：传递prevLastKey的方式
		// 计算：prevLastKey，这样可快速的得到prevLastKeyCreateRevision
		limit = (page - 1) * pageSize
		if getResponse, err = r.etcd.KV.Get(
			ctx, jobsDirKey,
			clientv3.WithFromKey(),
			clientv3.WithPrefix(),
			clientv3.WithKeysOnly(),
			clientv3.WithLimit(int64(limit)),
			clientv3.WithSort(clientv3.SortByCreateRevision, clientv3.SortAscend),
		); err != nil {
			return nil, err
		} else {
			//log.Println(len(getResponse.Kvs), limit)
			if len(getResponse.Kvs) != limit {
				// 超过了范围了
				return nil, nil
			} else {
				kvP := getResponse.Kvs[len(getResponse.Kvs)-1]
				// 后面会根据这个来做分页
				prevLastKeyCreateRevision = kvP.CreateRevision
				//log.Println(prevLastKeyCreateRevision, string(kvP.Key), "prevLastKeyCreateRevision")
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
	if getResponse, err = r.etcd.KV.Get(
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
		//log.Println(string(kvPair.Key), kvPair.ModRevision, kvPair.CreateRevision, kvPair.Version)
		count += 1
		if needDropPrevLastKey {
			if count > pageSize {
				log.Println("count > pageSize")
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
					//log.Println("count = 1")
					return jobs, nil
				}
			}
		}

		//	对值序列化
		job = &datamodels.Job{}
		if err = json.Unmarshal(kvPair.Value, job); err != nil {
			log.Println(err, string(kvPair.Value))
			continue
		} else {
			// 如果job未保存key，那么就添加一下
			if job.EtcdKey == "" {
				job.EtcdKey = string(kvPair.Key)
			}
			jobs = append(jobs, job)
		}
	}

	//	返回结果
	return jobs, nil

}
