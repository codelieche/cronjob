package repositories

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"

	"github.com/coreos/etcd/mvcc/mvccpb"

	"github.com/codelieche/cronjob/backend/common"
	"github.com/codelieche/cronjob/backend/common/datamodels"
	"github.com/coreos/etcd/clientv3"
)

// 保存Category到Etcd中
func (r *categoryRepository) saveCategoryToEtcd(category *datamodels.Category, isCreate bool) (prevCategory *datamodels.Category, err error) {
	// 把任务保存到:/crontab/jobs/:id中
	// 把分类保存到：/crontab/categories/:name中

	// 1. 定义变量
	var (
		categoryEtcdKey   string
		categoryEtcdValue []byte
		putResponse       *clientv3.PutResponse
	)

	// 2. 先处理下Category在etcd中 的key
	if category.Name == "" {
		err = fmt.Errorf("Category的Name不能为空")
		return nil, err
	}

	// 组合categoryKey
	categoryEtcdKey = common.ETCD_JOBS_CATEGORY_DIR + category.Name
	// 设置key
	category.EtcdKey = categoryEtcdKey

	// 3. 判断category是否存在
	if prevCategory, err = r.getCategoryFromEtcd(category.Name); err != nil {
		if err == common.NotFountError {
			// 未找到分类
			if !isCreate {
				// 更新操作，必须存在
				return nil, err
			}
		}
	} else {
		// 找到了
		if isCreate {
			// 由于是创建：存在的话，报错
			err = fmt.Errorf("分类%s已经存在于etcd中，不可创建", category.Name)
			return prevCategory, err
		} else {
			// 更新操作：在调用的地方记得校验name
		}
	}

	// 4. 保存category到etcd中
	// 4-1: 对category信息序列化
	if categoryEtcdValue, err = json.Marshal(category); err != nil {
		return nil, err
	}

	// 4-2: 保存数据到etcd中
	if putResponse, err = r.etcd.KV.Put(
		context.TODO(),            // 上下文
		categoryEtcdKey,           // key
		string(categoryEtcdValue), //值
		clientv3.WithPrevKV(),     // 返回上一个版本的值
	); err != nil {
		return nil, err
	} else {
		// 插入成功
		// 回写etcdkey
		updateFields := map[string]interface{}{"etcd_key": categoryEtcdKey}
		r.Update(category, updateFields)
	}

	// 5. 返回
	// 如果是更新，那么需返回上一个版本的job
	if putResponse.PrevKv != nil {
		// 对旧值反序列化下
		if err = json.Unmarshal(putResponse.PrevKv.Value, &prevCategory); err != nil {
			log.Println(err)
			// 这里虽然反序列化出错了，但是不影响保存的操作，这里我们可以把err设置为空
			return nil, nil
		} else {
			// 返回上一个的旧值
			return prevCategory, err
		}
	} else {
		// 没有上一个job的值
		return nil, nil
	}
}

// 从etcd中获取计划任务的分类
func (r *categoryRepository) getCategoryFromEtcd(name string) (category *datamodels.Category, err error) {
	// 1. 定义变量
	var (
		categoryEtcdKey string
		getResponse     *clientv3.GetResponse
		keyValue        *mvccpb.KeyValue
		i               int
	)

	// 2. 对key做校验
	name = strings.TrimSpace(name)
	if name == "" {
		err = fmt.Errorf("传入的category为空")
		return nil, err
	}

	categoryEtcdKey = common.ETCD_JOBS_CATEGORY_DIR + name

	// 3. 从etcd中获取对象
	if getResponse, err = r.etcd.KV.Get(context.TODO(), categoryEtcdKey); err != nil {
		return nil, err
	}

	// 4. 获取kv对象
	//log.Println(getResponse.Header)
	//log.Println(getResponse.Kvs)

	if len(getResponse.Kvs) == 1 {
		// 4-1: 获取到etcd中的value
		for i = range getResponse.Kvs {
			keyValue = getResponse.Kvs[i]
			//log.Println(keyValue.Value)
			// 4-2：json反序列化
			category = &datamodels.Category{}
			if err = json.Unmarshal(keyValue.Value, category); err != nil {
				log.Println("获取category反序列化出错：", err)
				return nil, err
			} else {
				category.EtcdKey = categoryEtcdKey
				return category, nil
			}
		}
		goto NotFound
	} else {
		goto NotFound
	}
NotFound:
	err = common.NotFountError
	return nil, err
}

// 从etcd获取分类的列表
// 获取的是全部的分类，未做分页的
func (r *categoryRepository) listCategoriesFromEtcd() (categoriesArry []*datamodels.Category, err error) {
	// 1. 定义变量
	var (
		categoriesDirKey string
		getResponse      *clientv3.GetResponse
		kvPair           *mvccpb.KeyValue
		category         *datamodels.Category
	)

	// 2. 获取分类所在的key的前缀
	categoriesDirKey = common.ETCD_JOBS_CATEGORY_DIR

	//	3. 从etcd中获取数据
	if getResponse, err = r.etcd.KV.Get(
		context.TODO(),
		categoriesDirKey,
		clientv3.WithPrefix(),
	); err != nil {
		return nil, err
	} else {
		// 获取成功
	}

	//	4. 处理结果
	for _, kvPair = range getResponse.Kvs {
		// 4-1: 对value反序列化
		category = &datamodels.Category{}
		if err = json.Unmarshal(kvPair.Value, category); err != nil {
			log.Println(err)
			continue
		} else {
			// 反序列化成功
			// 4-2: 加入到结果集中
			categoriesArry = append(categoriesArry, category)
		}
	}

	// 5. 返回结果
	return categoriesArry, nil
}
