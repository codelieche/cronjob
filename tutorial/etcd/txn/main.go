package main

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"time"

	"go.etcd.io/etcd/clientv3"
)

func main() {
	// 定义变量
	var (
		// etcd 连接相关
		config clientv3.Config
		client *clientv3.Client
		err    error

		//	kv相关
		kv clientv3.KV

		//  租约相关
		lease             clientv3.Lease
		leaseGrantResp    *clientv3.LeaseGrantResponse            // 申请租约的响应
		leaseID           clientv3.LeaseID                        // 申请的租约ID
		keepAliveRespChan <-chan *clientv3.LeaseKeepAliveResponse // 自动续租的响应chan
		keepResp          *clientv3.LeaseKeepAliveResponse        // 发起自动续租的响应

		// 上下文相关
		ctx        context.Context
		cancelFunc context.CancelFunc

		//	事务相关
		txn     clientv3.Txn
		txnResp *clientv3.TxnResponse
	)

	// 客户端配置
	config = clientv3.Config{
		Endpoints:   []string{"127.0.0.1:2379"},
		DialTimeout: 20 * time.Second,
	}

	// 建立连接
	client, err = clientv3.New(config)
	if err != nil {
		log.Panic(err)
	}

	// lease实现锁自动过期
	// op操作
	// txn事务：if else then

	//	1. 上锁（创建租约，自动续租，拿着租约去抢占一个key）

	//  1-1: 申请个十秒的租约
	lease = clientv3.NewLease(client)
	if leaseGrantResp, err = lease.Grant(context.TODO(), 10); err != nil {
		log.Println("创建租约出错：", err.Error())
		return
	} else {
		leaseID = leaseGrantResp.ID

		//  1-2: 自动续租
		ctx, cancelFunc = context.WithCancel(context.TODO())
		if keepAliveRespChan, err = lease.KeepAlive(ctx, leaseID); err != nil {
			log.Panic(err)
		} else {
			// 最后记得要关闭自动续租！！！
			defer cancelFunc()
			// 释放锁的时候，想立刻让租约关联的key过期
			defer lease.Revoke(context.TODO(), leaseID)

			// 1-3:  不断的消耗续租的响应
			go func() {
				for {
					select {
					case keepResp = <-keepAliveRespChan:
						if keepResp == nil {
							log.Println("租约已经失效了")
							goto END
						} else {
							log.Println("收到自动续租的响应：", keepResp.ID)
						}
					}
				}
			END:
				log.Println("消耗续租的协程Done！")
			}()
		}

	}

	//	2. 处理业务
	//  if 不存在key， then 设置它，else 抢锁失败

	// 2-1: 实例化kv, txn
	kv = clientv3.NewKV(client)
	txn = kv.Txn(context.TODO())

	// 2-2: 定义事务
	// 如果key不存在
	rand.Seed(time.Now().UnixNano())
	key := fmt.Sprintf("/study/lock/job%d", rand.Intn(100))
	log.Println(key)
	txn.If(clientv3.Compare(clientv3.CreateRevision(key), "=", 0)).
		Then(clientv3.OpPut(key, "value", clientv3.WithLease(leaseID))).
		Else(clientv3.OpGet(key)) // 否则抢锁失败
	//  当代码走到这里就已经在锁内了，很安全
	if txnResp, err = txn.Commit(); err != nil {
		log.Println(err.Error())
		return
	}

	//  判断是否抢到了锁
	if !txnResp.Succeeded {
		//  没抢到锁
		log.Println("锁被占用了", txnResp.Responses[0].GetResponseRange().Kvs[0])
		return
	}

	// 处理业务
	log.Println("现在抢到了锁，处理业务吧！")
	time.Sleep(10 * time.Second)
	log.Println("现在抢到了锁，业务处理完毕！")

	//  3. 释放锁（取消自动续租，释放租约[立即把key删除]）
	//	defer 会把租约释放掉，关联的kv会被删除

	log.Println("=== Done ===")

}
