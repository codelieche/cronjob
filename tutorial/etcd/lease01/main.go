package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/coreos/etcd/clientv3"
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

		//	Lease 租约
		fmt.Println("\n==== Lease 租约 ====")
		lease := clientv3.NewLease(client)

		//	申请一个10秒的租约
		// Grant是租约，KeepAlive是续租
		if leaseGrantResp, err := lease.Grant(context.TODO(), 10); err != nil {
			log.Panic(err)
		} else {
			log.Println(leaseGrantResp)
			//	租约ID
			leaseID := leaseGrantResp.ID

			//  put一个kv，让它与租约关联起来，实现10秒后过期
			kv := clientv3.NewKV(client)
			//  和以前Put的区别是，加入了clientv3.WithLease(leaseID)
			if putResp, err := kv.Put(context.TODO(),
				"/study/lease/k1", "value002", clientv3.WithLease(leaseID)); err != nil {
				log.Panic(err)
			} else {
				log.Println(putResp)
			}

			//	写个循环不断的查询这个k1
			for {
				if getResp, err := kv.Get(context.TODO(), "/study/lease/k1"); err != nil {
					log.Panic(err)
				} else {
					log.Println(getResp.Kvs)
					if getResp.Count == 0 {
						log.Println("key：/study/lease/k1 过期了")
						break
					}
					time.Sleep(2 * time.Second)
				}
			}

			log.Println("==== Done ===")

		}

	}

}
