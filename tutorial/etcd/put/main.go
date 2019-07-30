package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"go.etcd.io/etcd/clientv3"
)

func main() {
	//	客户端配置
	config := clientv3.Config{
		Endpoints:   []string{"127.0.0.1:2379"},
		DialTimeout: 5 * time.Second,
	}

	//	建立连接
	if client, err := clientv3.New(config); err != nil {
		log.Panic(err)
	} else {
		log.Println(client)

		//	写入数据方式一
		fmt.Println("\n==== Put数据方式1 ====")
		if response, err := client.Put(context.TODO(), "/study/name1", "value 000001"); err != nil {
			log.Panic(err)
		} else {
			log.Println(response.Header)
			log.Println(response.Header.RaftTerm)
		}

		//	写入数据方式二
		fmt.Println("\n==== Put数据方式2 ====")
		kv := clientv3.NewKV(client)
		if response, err := kv.Put(context.TODO(), "/study/name2", "value 000002", clientv3.WithPrevKV()); err != nil {
			log.Panic(err)
		} else {
			log.Println(response.Header)
			log.Println(response.Header.RaftTerm)
			log.Println(response.PrevKv.Value)
			log.Println("PrevKv.Value", string(response.PrevKv.Value))
		}

		fmt.Println("\n==== Put 操作3 ====")
		//  批量插入一些数据
		var i = 0
		for {
			i += 1
			k := fmt.Sprintf("/demo/k%d", i)
			v := fmt.Sprintf("value %d", i)
			if response, err := kv.Put(context.TODO(), k, v); err != nil {
				log.Panic(err)
			} else {
				log.Println(response.Header)
			}

			if i > 100 {
				break
			}
		}
	}

}
