## Tutorial
> 本目录下的代码属于示例代码。项目开发中的技术点，示例程序，放在这里。


### 示例

#### Golang执行命令
- 示例1：简单执行命令：[exec/command01/main.go](./exec/command01/main.go)
- 示例2：执行命令并捕获输出：[exec/command02/main.go](./exec/command02/main.go)
- 示例3：协程中执行命令，带取消上下文函数：[exec/command03/main.go](./exec/command03/main.go)
- 示例4：协程中执行命令，把结果传给主协程: [exec/command04/main.go](./exec/command04/main.go)
- 示例5：协程中执行命令，把结果传给主协程[随机版]: [exec/command05/main.go](./exec/command05/main.go)

#### cronta相关示例
- [示例1：使用cronexp库解析crontab时间](crontab/demo01/main.go)
- [示例2：获取crontab下一个执行时间](crontab/demo02/main.go)
- [示例3：crontab持续调度Demo](crontab/demo03/main.go)

#### etcd示例
> https://godoc.org/go.etcd.io/etcd/clientv3

- [示例1：etcd put操作](etcd/put/main.go)
- [示例2：etcd get操作](etcd/get/main.go)
- [示例3：etcd delete操作](etcd/delete/main.go)
- [示例4：etcd lease(租约)操作](etcd/lease01/main.go)
- [示例5：etcd lease(租约+续租)操作](etcd/lease02/main.go)
- [示例6：etcd watch 操作](etcd/watch/main.go)
- [示例7：etcd operation Get Put Delete](etcd/operation/main.go)
- [示例7：etcd txn(事务)操作](etcd/txn/main.go)

