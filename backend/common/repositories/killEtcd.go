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

func (r *jobKillRepository) saveJobkillToEtcd(jobKill *datamodels.JobKill, isCreate bool) (prevKillEtcd *datamodels.KillEtcd, err error) {
	// 把jobKill保存到：/crontab/kill/
	// isCreate是否是创建
	// 创建的时候：需要判断job是否存在，存在就报错
	// 不是创建，那么就是更新，需要判断是否存在，不存在就报错
	var (
		jobKillEtcd     *datamodels.KillEtcd
		jobsKillEtcdDir string // job在etcd中的上级目录
		jobKillEtcdKey  string // job在etcd中的key
		jobEtcdValue    []byte // job在etcd中的value
		putResponse     *clientv3.PutResponse
	)

	// 先处理下jobKill的etcd中的key
	if jobKill.JobID <= 0 {
		err = fmt.Errorf("JobKill的Job ID不能为空")
		return nil, err
	}

	// 对job的分类进行校验
	if jobKill.Category == "" {
		err = fmt.Errorf("job Kill的Category不可为空")
		return nil, err
	}

	// 获取job存储的key
	jobsKillEtcdDir = common.ETCD_JOB_KILL_DIR
	if strings.HasSuffix(jobsKillEtcdDir, "/") {
		jobsKillEtcdDir = string(jobsKillEtcdDir[:len(jobsKillEtcdDir)-1])
	}

	jobKillEtcdKey = fmt.Sprintf("%s/%s/%d", jobsKillEtcdDir, jobKill.Category, jobKill.JobID)
	// 检查一下etcd的Key
	if jobKill.EtcdKey == "" {
		// 设置一下job的etcdKey
		jobKill.EtcdKey = jobKillEtcdKey
		updateFields := make(map[string]interface{})
		updateFields["EtcdKey"] = jobKillEtcdKey
		// 更新job的key
		r.db.Model(jobKill).Limit(1).Update(updateFields)
	}

	// 判断job是否已经存在了
	// TOOD: 这里应该加个锁，抢到锁才创建，要不大量频繁创建，可能会造成name重复
	if prevKillEtcd, err = r.getJobKillFromEtcd(jobKill.Category, jobKill.ID); err != nil {
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
			err = fmt.Errorf("jobKill:%s已经存在于etcd中，不可创建", jobKillEtcdKey)
			return prevKillEtcd, err
		} else {
			// 更新操作：在调用的地方记得校验name
		}
	}

	// 对jobEtcd反序列化
	jobKillEtcd = jobKill.ToEtcdDataStruct()
	if jobEtcdValue, err = json.Marshal(jobKillEtcd); err != nil {
		return nil, err
	}

	// 保存数据到etcd中
	//log.Println(jobEtcdKey, jobEtcdValue)
	if putResponse, err = r.etcd.KV.Put(
		context.TODO(),        // 上下文
		jobKillEtcdKey,        // key
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
		if err = json.Unmarshal(putResponse.PrevKv.Value, &prevKillEtcd); err != nil {
			log.Println(err)
			// 这里虽然反序列化出错了，但是不影响保存的操作，这里我们可以把err设置为空
			return jobKillEtcd, nil
		} else {
			// 返回上一个就的值
			return prevKillEtcd, nil
		}
	} else {
		// 没有上一个job的值
		return jobKillEtcd, nil
	}
}

func (r *jobKillRepository) getJobKillFromEtcd(categoryName string, id uint) (killEtcd *datamodels.KillEtcd, err error) {
	// 1. 定义变量
	var (
		jobEtcdKey  string
		getResponse *clientv3.GetResponse
		keyValue    *mvccpb.KeyValue
		i           int
	)

	// 2. 对key做校验
	categoryName = strings.TrimSpace(categoryName)

	jobEtcdKey = fmt.Sprintf("%s%s/%d", common.ETCD_JOB_KILL_DIR, categoryName, id)
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
			killEtcd = &datamodels.KillEtcd{}
			if err = json.Unmarshal(keyValue.Value, killEtcd); err != nil {
				log.Println("获取Job反序列化出错：", err)
				return nil, err
			} else {
				//jobEtcd.EtcdKey = jobEtcdKey
				return killEtcd, nil
			}
		}
		return nil, nil
	} else {
		return nil, common.NotFountError
	}
}

// 从etcd中删除JobKill
// 从etcd中删除JobKill
func (r *jobKillRepository) deleteJobKillFromEtcd(jobKill *datamodels.JobKill) (success bool, err error) {
	var etcdKey string

	// 从etcd中删除
	if jobKill.EtcdKey != "" {
		etcdKey = jobKill.EtcdKey
	} else {
		etcdKey = fmt.Sprintf("%s%s/%d", common.ETCD_JOB_KILL_DIR, jobKill.Category, jobKill.JobID)
	}
	return r.deleteFromEtcd(etcdKey)
}

func (r *jobKillRepository) deleteFromEtcd(etcdKey string) (success bool, err error) {
	// 1. 定义变量
	var (
		deleteResponse *clientv3.DeleteResponse
		ctx            context.Context
	)

	// 2. 对jobkey做判断
	etcdKey = strings.TrimSpace(etcdKey)
	if !strings.HasPrefix(etcdKey, common.ETCD_JOB_KILL_DIR) {
		err = errors.New("传入的key前缀不正确")
		return false, err
	}

	if etcdKey == "" {
		err = errors.New("jobKill etcdKey不可为空")
		return false, err
	}

	// 3. 操作删除
	etcdKey = strings.Replace(etcdKey, "//", "/", -1)
	ctx, _ = context.WithTimeout(context.TODO(), time.Duration(5)*time.Second)

	if deleteResponse, err = r.etcd.KV.Delete(
		ctx,
		etcdKey,
		clientv3.WithPrevKV(),
	); err != nil {
		return false, err
	}

	// 4. 校验被删除的keyValue
	if len(deleteResponse.PrevKvs) < 1 {
		err = fmt.Errorf("%s不存在", etcdKey)
		return false, err
	} else {
		// 删除成功
		return true, nil
	}
}
