package datasources

import (
	"crypto/tls"
	"errors"
	"log"
	"os"
	"time"

	"github.com/codelieche/cronjob/backend/common"
	"github.com/coreos/etcd/clientv3"
	"github.com/coreos/etcd/pkg/transport"
)

// etcd对象
type Etcd struct {
	Client  *clientv3.Client
	KV      clientv3.KV
	Lease   clientv3.Lease
	Watcher clientv3.Watcher
}

var etcd *Etcd

func connectEtcd(etcdConfig *common.EtcdConfig) {
	// 1. 定义变量
	var (
		config    clientv3.Config
		client    *clientv3.Client
		kv        clientv3.KV
		lease     clientv3.Lease
		watcher   clientv3.Watcher
		err       error
		tlsInfo   transport.TLSInfo
		tlsConfig *tls.Config
	)

	// log.Println(etcdConfig.TLS)

	// 2. 判断是否配置了TLS
	if etcdConfig.TLS != nil {
		// 检查其三个字段是否为空
		if etcdConfig.TLS.CertFile == "" || etcdConfig.TLS.KeyFile == "" || etcdConfig.TLS.CaFile == "" {
			log.Println(etcdConfig.TLS)
			err = errors.New("传入的TLS配置不可为空")
			log.Println(err)
			os.Exit(1)
		} else {
			tlsInfo = transport.TLSInfo{
				CertFile:      etcdConfig.TLS.CertFile,
				KeyFile:       etcdConfig.TLS.KeyFile,
				TrustedCAFile: etcdConfig.TLS.CaFile,
			}
			if tlsConfig, err = tlsInfo.ClientConfig(); err != nil {
				log.Println(err)
				os.Exit(1)
			}
		}
	}

	//	初始化etcd配置
	config = clientv3.Config{
		//Endpoints:   []string{"127.0.0.1:2379"}, // 集群地址
		Endpoints:   etcdConfig.Endpoints,    // 集群地址
		DialTimeout: 5000 * time.Microsecond, // 连接超时
		TLS:         tlsConfig,
	}

	// 建立连接
	if client, err = clientv3.New(config); err != nil {
		log.Println(err)
		os.Exit(1)
	} else {
		// 连接成功
	}

	// 得到KV的Lease的API子集
	kv = clientv3.NewKV(client)
	lease = clientv3.NewLease(client)
	watcher = clientv3.NewWatcher(client)

	// 实例化etcd
	etcd = &Etcd{
		Client:  client,
		KV:      kv,
		Lease:   lease,
		Watcher: watcher,
	}

}

func GetEtcd() *Etcd {
	// 1. 判断etcd是否存在
	if etcd != nil {
		return etcd
	} else {
		// 2. 获取配置
		config := common.Config
		etcdConfig := config.Master.Etcd
		connectEtcd(etcdConfig)
	}
	return etcd
}

//func NewEtcdClientKvLeaseWatcher(etcdConfig *common.EtcdConfig) (*clientv3.Client, clientv3.KV, clientv3.Lease, clientv3.Watcher, error) {
//	var (
//		config    clientv3.Config
//		client    *clientv3.Client
//		kv        clientv3.KV
//		lease     clientv3.Lease
//		watcher   clientv3.Watcher
//		err       error
//		tlsInfo   transport.TLSInfo
//		tlsConfig *tls.Config
//	)
//
//	// log.Println(etcdConfig.TLS)
//
//	if etcdConfig.TLS != nil {
//		// 检查其三个字段是否为空
//		if etcdConfig.TLS.CertFile == "" || etcdConfig.TLS.KeyFile == "" || etcdConfig.TLS.CaFile == "" {
//			log.Println(etcdConfig.TLS)
//			err = errors.New("传入的TLS配置不可为空")
//			return nil, nil, nil, nil, err
//		} else {
//			tlsInfo = transport.TLSInfo{
//				CertFile:      etcdConfig.TLS.CertFile,
//				KeyFile:       etcdConfig.TLS.KeyFile,
//				TrustedCAFile: etcdConfig.TLS.CaFile,
//			}
//			if tlsConfig, err = tlsInfo.ClientConfig(); err != nil {
//				return nil, nil, nil, nil, err
//			}
//		}
//	}
//
//	//	初始化etcd配置
//	config = clientv3.Config{
//		//Endpoints:   []string{"127.0.0.1:2379"}, // 集群地址
//		Endpoints:   etcdConfig.Endpoints,    // 集群地址
//		DialTimeout: 5000 * time.Microsecond, // 连接超时
//		TLS:         tlsConfig,
//	}
//
//	// 建立连接
//	if client, err = clientv3.New(config); err != nil {
//		return nil, nil, nil, nil, err
//	} else {
//		// 连接成功
//	}
//
//	// 得到KV的Lease的API子集
//	kv = clientv3.NewKV(client)
//	lease = clientv3.NewLease(client)
//	watcher = clientv3.NewWatcher(client)
//	return client, kv, lease, watcher, err
//}
