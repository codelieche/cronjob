package common

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"

	"go.etcd.io/etcd/clientv3"
	"go.etcd.io/etcd/mvcc/mvccpb"
)

// 保存Category到etcd中
// 返回上一次的Job和错误信息
// 如果isCreate为true就表示添加新的，false就表示更新
func (etcdManager *EtcdManager) SaveCategory(category *Category, isCreate bool) (prevCategory *Category, err error) {
	// 把任务保存到/crontab/jobs/:name中
	var (
		categoryKey   string
		categoryValue []byte
		putResponse   *clientv3.PutResponse
	)

	// 先处理下Job在etcd中的key
	if category.Name == "" {
		err = fmt.Errorf("Category的Name不能为空")
		return nil, err
	}

	// categoryKey = ETCD_JOBS_DIR + job.Name

	// 组合categoryKey
	categoryKey = ETCD_JOBS_CATEGORY_DIR + category.Name
	// 设置key
	category.Key = categoryKey
	// 判断category是否存在
	if prevCategory, err = etcdManager.GetCategory(category.Name); err != nil {
		if err == NOT_FOUND {
			// 未找到
			if !isCreate {
				// 是更新操作，必须存在
				// err = fmt.Errorf("分类%s不存在，不可更新", category.Name)
				return nil, err
			}
		}
	} else {
		// 找到了
		if isCreate {
			// 由于是创建：存在的话，报错
			err = fmt.Errorf("分类%s已经存在，不可创建", category.Name)
			return prevCategory, err
		} else {
			// 更新操作：在调用的地方记得校验name
		}
	}

	// 任务信息json：对category序列化一下
	if categoryValue, err = json.Marshal(category); err != nil {
		return nil, err
	}

	//	保存到etcd中
	if putResponse, err = etcdManager.kv.Put(
		context.TODO(),        // 上下文
		categoryKey,           // Key
		string(categoryValue), // 值
		clientv3.WithPrevKV(), // 返回上一个版本的值
	); err != nil {
		return nil, err
	}

	// 如果是更新，那么返回上一个版本的job
	if putResponse.PrevKv != nil {
		//	对旧值反序列化下
		if err = json.Unmarshal(putResponse.PrevKv.Value, &prevCategory); err != nil {
			log.Println(err.Error())
			// 这里虽然反序列化出错了，但是不影响保存的操作，这里我们可以把err设置为空
			return nil, nil
		} else {
			// 返回上一个的旧值
			return prevCategory, err
		}
	} else {
		// 没有上一个的job值，直接返回
		return nil, nil
	}
}

// 获取计划任务的分类
func (etcdManager *EtcdManager) GetCategory(name string) (category *Category, err error) {
	// 定义变量
	var (
		categoryKey string
		getResponse *clientv3.GetResponse
		keyValue    *mvccpb.KeyValue
		i           int
	)
	// 1. 对jobKey做校验
	name = strings.TrimSpace(name)
	if name == "" {
		err = fmt.Errorf("传入的category为空")
		return nil, err
	}

	categoryKey = ETCD_JOBS_CATEGORY_DIR + name

	// 2. 从etcd中获取对象
	if getResponse, err = etcdManager.kv.Get(context.TODO(), categoryKey); err != nil {
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
			category = &Category{}
			if err = json.Unmarshal(keyValue.Value, category); err != nil {
				return nil, err
			} else {
				category.Key = categoryKey
				return category, nil
			}
		}
		goto NotFound
	} else {
		goto NotFound
	}

NotFound:
	//err = errors.New("category not fount")
	err = NOT_FOUND
	return nil, err
}

// 获取分类的列表
func (etcdManager *EtcdManager) ListCategories() (categoriesArry []*Category, err error) {
	// 定义变量
	var (
		categoriesDirKey string
		getResponse      *clientv3.GetResponse
		kvPair           *mvccpb.KeyValue
		category         *Category
	)

	categoriesDirKey = ETCD_JOBS_CATEGORY_DIR

	// 从etcd中获取数据
	if getResponse, err = etcdManager.kv.Get(
		context.TODO(),
		categoriesDirKey,
		clientv3.WithPrefix(),
	); err != nil {
		return nil, err
	}

	// 处理结果
	for _, kvPair = range getResponse.Kvs {
		// 对value序列化
		category = &Category{}
		if err = json.Unmarshal(kvPair.Value, category); err != nil {
			log.Println(err)
			continue
		} else {
			// 反序列化成功
			categoriesArry = append(categoriesArry, category)
		}
	}

	// 返回
	return
}
