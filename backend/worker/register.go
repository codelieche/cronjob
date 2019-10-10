package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"time"

	"cronjob.codelieche/backend/common"

	"go.etcd.io/etcd/clientv3"
)

// 注册节点信息到master
type Register struct {
	client   *clientv3.Client
	kv       clientv3.KV
	lease    clientv3.Lease
	Name     string `json:"name"`     // 节点的名称：Ip:Port（这样就算唯一的了）
	HostName string `json:"hostname"` // 主机名
	Ip       string `json:"ip"`       // IP地址
	Port     int    `json:"port"`     // worker 监控服务的端口
	Pid      int    `json:"pid"`      // Worker的端口号
}

// 注册到：/crontab/workers/目录中
func (register *Register) keepOnlive() {
	var (
		workerKey          string
		workerInfoValue    []byte
		leaseGrantResponse *clientv3.LeaseGrantResponse
		err                error
		leaseID            clientv3.LeaseID

		keepAliveChan     <-chan *clientv3.LeaseKeepAliveResponse
		keepAliveResponse *clientv3.LeaseKeepAliveResponse

		putResponse *clientv3.PutResponse
	)

	for {
		// 注册路径
		workerKey = common.ETCD_WORKER_DIR + register.Name

		// 创建租约
		if leaseGrantResponse, err = register.lease.Grant(context.TODO(), 10); err != nil {
			// 如果出错，可以等下重试
			log.Println(err.Error())
			goto RETRY
		}

		//	自动续租
		leaseID = leaseGrantResponse.ID
		if keepAliveChan, err = register.lease.KeepAlive(context.TODO(), leaseID); err != nil {
			log.Println(err.Error())
			goto RETRY
		}

		// 注册到etcd
		if workerInfoValue, err = json.Marshal(register); err != nil {
			log.Println(err)
			goto RETRY
		}
		if putResponse, err = register.kv.Put(
			context.TODO(), workerKey, string(workerInfoValue),
			clientv3.WithLease(leaseID),
		); err != nil {
			log.Println(err.Error())
			goto RETRY
		} else {
			putResponse = putResponse
		}

		//	处理续租应答
		for {
			select {
			case keepAliveResponse = <-keepAliveChan:
				if keepAliveResponse == nil {
					// 续租失败
					goto RETRY
				}
			}
		}

	RETRY:
		time.Sleep(time.Second * 30)
	}

}

func newRegister() (register *Register, err error) {
	// 先连接etcd相关
	var (
		config clientv3.Config
		client *clientv3.Client
		kv     clientv3.KV
		lease  clientv3.Lease

		hostName  string
		ipAddress string
	)

	// 初始化配置
	config = clientv3.Config{
		Endpoints:   []string{"127.0.0.1:2379"},
		DialTimeout: time.Second * 10,
	}

	// 建立连接
	if client, err = clientv3.New(config); err != nil {
		return
	}

	// 得到KV和Lease的API子集
	kv = clientv3.NewKV(client)
	lease = clientv3.NewLease(client)

	// 获取到主机名
	if hostName, err = os.Hostname(); err != nil {
		return
	}

	// 获取主机的IP
	if ipAddress, err = common.GetFirstLocalIpAddress(); err != nil {
		return
	}

	register = &Register{
		client:   client,
		kv:       kv,
		lease:    lease,
		HostName: hostName,
		Ip:       ipAddress,
		Pid:      os.Getppid(), // 进程号
	}

	register.Name = fmt.Sprintf("%s:%d", register.Ip, register.Pid)

	return register, err
}
