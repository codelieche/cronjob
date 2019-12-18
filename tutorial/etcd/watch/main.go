package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/coreos/etcd/mvcc/mvccpb"

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

		//	Watch
		fmt.Println("\n==== Watch ====")

		// 实例化个watcher
		watcher := clientv3.NewWatcher(client)

		// 60秒后自动取消watch
		ctx, cancelFun := context.WithCancel(context.TODO())
		watchFlag := true
		time.AfterFunc(10*time.Second, func() {
			log.Println("现在关闭watch.......")
			watchFlag = false
			cancelFun()
		})

		watchChan := watcher.Watch(ctx, "/study", clientv3.WithPrefix(), clientv3.WithPrevKV())

		//	消费watchChan的响应
		for watchFlag {

			select {
			case watchResp := <-watchChan:
				//log.Println(watchResp.Header)
				//log.Println(watchResp.Events)
				for _, event := range watchResp.Events {
					switch event.Type {
					case mvccpb.PUT:
						fmt.Println("\t修改操作")

					case mvccpb.DELETE:
						fmt.Println("\t删除操作")
					}

					fmt.Println("\t是否是创建？", event.IsCreate())
					fmt.Println("\t是否是修改？", event.IsModify())
					fmt.Printf("事件：Key: %s, Value: %s, Version: %d\n",
						event.Kv.Key, event.Kv.Value, event.Kv.Version)
					if event.IsModify() {
						log.Println(event.PrevKv)
					}

				}
				//default:
				//	log.Println("==== Default ====")
			}

		}

		log.Println("=== Done ===")

	}

}
