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
		//log.Println(client)

		//	Delete 数据方式一
		fmt.Println("==== Delete数据方式1 ====")
		if response, err := client.Delete(context.TODO(), "/study/name1"); err != nil {
			log.Panic(err)
		} else {
			log.Println(response.Header)
			log.Println("删除的个数", response.Deleted)
		}

		//	Delete 数据方式二
		fmt.Println("\n==== Delete数据方式2 ====")
		kv := clientv3.NewKV(client)
		if response, err := kv.Delete(context.TODO(), "/demo/key9"); err != nil {
			log.Panic(err)
		} else {
			log.Println(response.Header)
			log.Println("删除的个数", response.Deleted)

		}

		//	Delete 数据方式三
		fmt.Println("\n==== Delete数据方式3 批量 ====")
		kv = clientv3.NewKV(client)
		if response, err := kv.Delete(context.TODO(), "/demo/k9", clientv3.WithPrefix(), clientv3.WithPrevKV()); err != nil {
			log.Panic(err)
		} else {
			log.Println(response.Header)
			log.Println("删除的个数", response.Deleted)
			for i := range response.PrevKvs {
				item := response.PrevKvs[i]
				fmt.Printf("\t\t\tKey: %s \tValue: %s\n", item.Key, item.Value)
			}
		}
	}

}
