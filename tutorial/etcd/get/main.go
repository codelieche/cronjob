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

		//	Get数据方式一
		fmt.Println("==== Get数据方式1 ====")
		if response, err := client.Get(context.TODO(), "/study/name1"); err != nil {
			log.Panic(err)
		} else {
			log.Println(response.Header)
			log.Println(response.Kvs)
		}

		//	Get数据方式二
		fmt.Println("\n==== Get数据方式2 ====")
		kv := clientv3.NewKV(client)
		if response, err := kv.Get(context.TODO(), "/study/name2"); err != nil {
			log.Panic(err)
		} else {
			log.Println(response.Header)
			log.Println(response.Header.RaftTerm)
			log.Println(response.Kvs)
			for i := range response.Kvs {
				item := response.Kvs[i]
				fmt.Printf("\tKey: %s \tValue: %s\n", item.Key, item.Value)
			}
		}

		//	Get数据方式二
		fmt.Println("\n==== Get数据方式3 ====")
		kv = clientv3.NewKV(client)
		if response, err := kv.Get(context.TODO(), "/study", clientv3.WithPrefix()); err != nil {
			log.Panic(err)
		} else {
			log.Println(response.Header)
			log.Println(response.Header.RaftTerm)
			log.Println(response.Kvs)
			for i := range response.Kvs {
				item := response.Kvs[i]
				fmt.Printf("\t\t\tKey: %s \tValue: %s\n", item.Key, item.Value)
			}
		}
	}

}
