package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"go.etcd.io/etcd/clientv3"
)

func main() {

	// etcd配置
	config := clientv3.Config{
		Endpoints:   []string{"127.0.0.1:2379"},
		DialTimeout: 20 * time.Second,
	}

	// 连接etcd
	client, err := clientv3.New(config)
	if err != nil {
		log.Panic(err)
	}

	// 实例化kv
	kv := clientv3.NewKV(client)

	fmt.Println("=== Operation Put ===")
	//	实例化op
	opPut := clientv3.OpPut("/study/name1", "value 0001", clientv3.WithPrevKV())

	//	执行op
	if opResponse, err := kv.Do(context.TODO(), opPut); err != nil {
		log.Println(err.Error())
	} else {
		// 处理响应数据
		putResp := opResponse.Put()
		prevKv := putResp.PrevKv
		fmt.Printf("\tHeaders：%s\n", putResp.Header)
		fmt.Printf("\t上一个版本的Value是：%s\n\n", prevKv.Value)

	}

	fmt.Println("=== Operation Get ===")
	//  实例化op
	opGet := clientv3.OpGet("/study/", clientv3.WithPrefix())

	//	执行op
	if opResponse, err := kv.Do(context.TODO(), opGet); err != nil {
		log.Println(err.Error())
	} else {
		//	打印出获取到的数据
		getResp := opResponse.Get()
		for _, item := range getResp.Kvs {
			fmt.Printf("\tKey: %s \t Value: %s\t Version: %d\n", item.Key, item.Value, item.Version)
		}
	}

}
