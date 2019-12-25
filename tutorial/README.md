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
> https://godoc.org/github.com/coreos/etcd/clientv3

- [示例1：etcd put操作](etcd/put/main.go)
- [示例2：etcd get操作](etcd/get/main.go)
- [示例3：etcd delete操作](etcd/delete/main.go)
- [示例4：etcd lease(租约)操作](etcd/lease01/main.go)
- [示例5：etcd lease(租约+续租)操作](etcd/lease02/main.go)
- [示例6：etcd watch 操作](etcd/watch/main.go)
- [示例7：etcd operation Get Put Delete](etcd/operation/main.go)
- [示例8：etcd txn(事务)操作](etcd/txn/main.go)

#### mongod示例
> https://godoc.org/go.mongodb.org/mongo-driver/mongo

- [示例1：mongo insert基本使用](mongo/insert01/main.go)
- [示例2：mongo InserMany 插入多行记录](mongo/insert02/main.go)
- [示例3：mongo Delete 基本使用](mongo/delete01/main.go)
- [示例4：mongo Find 基本使用](mongo/find01/main.go)

#### Elasticsearch示例
> https://github.com/elastic/go-elasticsearch/tree/6.x  
 https://godoc.org/github.com/elastic/go-elasticsearch/

- [示例1：elaseticsearch 基本使用](elasticsearch/demo01/main.go)


#### http示例
- [示例1：启动个简单的http服务](http/demo01/main.go)
- [示例2：http mux(多路复用器)基本使用](http/mux01/main.go)
- [示例3：http mux(多路复用器)串联2个处理器函数](http/mux01/main.go)
- [示例4：http andler(处理器)基本使用](http/handler01/main.go)
- [示例5：http cookie 基本使用](http/cookie01/main.go)
- [示例6：http template(模板) 基本使用](http/template01/main.go)
- [示例7：httprouter 的基本使用](http/router/main.go)

#### GORM示例
- [示例1：快速入门](gorm/helloworld/main.go)

#### iris示例
- [示例1：基本使用](iris/base/main.go)
- [示例2：websocket基本使用](iris/socket/main.go)
