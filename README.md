## 说明

### 目录
- `docs`: 项目文档
- `tutorial`: 项目技术点的示例代码
- `entry`: 程序的入口
- `backend`: 后端源代码
- `front`: 前端源代码

### 开发环境
- 操作系统：`MacOS 10.14.3`
- go版本：`go version go1.11.1 darwin/amd64`

### Package
- [etcd](https://github.com/etcd-io/etcd/tree/master/clientv3): `go get github.com/coreos/etcd/clientv3`
- [cronexpr](https://github.com/gorhill/cronexpr)：`go get github.com/gorhill/cronexpr`
- [go-elasticsearch](https://github.com/elastic/go-elasticsearch/tree/6.x)
  - `git clone --branch 6.x https://github.com/elastic/go-elasticsearch.git $GOPATH/src/github.com/elastic/go-elasticsearch`
- [mongo](https://github.com/mongodb/mongo-go-driver): `go get go.mongodb.org/mongo-driver`

### 参考文档
- [etcd docs](https://godoc.org/github.com/coreos/etcd/clientv3)