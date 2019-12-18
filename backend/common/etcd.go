package common

import (
	"crypto/tls"
	"errors"
	"log"
	"time"

	"github.com/coreos/etcd/clientv3"
	"github.com/coreos/etcd/pkg/transport"
)

func NewEtcdClientKvLeaseWatcher(etcdConfig *EtcdConfig) (*clientv3.Client, clientv3.KV, clientv3.Lease, clientv3.Watcher, error) {
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

	if etcdConfig.TLS != nil {
		// 检查其三个字段是否为空
		if etcdConfig.TLS.CertFile == "" || etcdConfig.TLS.KeyFile == "" || etcdConfig.TLS.CaFile == "" {
			log.Println(etcdConfig.TLS)
			err = errors.New("传入的TLS配置不可为空")
			return nil, nil, nil, nil, err
		} else {
			tlsInfo = transport.TLSInfo{
				CertFile:      etcdConfig.TLS.CertFile,
				KeyFile:       etcdConfig.TLS.KeyFile,
				TrustedCAFile: etcdConfig.TLS.CaFile,
			}
			if tlsConfig, err = tlsInfo.ClientConfig(); err != nil {
				return nil, nil, nil, nil, err
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
		return nil, nil, nil, nil, err
	} else {
		// 连接成功
	}

	// 得到KV的Lease的API子集
	kv = clientv3.NewKV(client)
	lease = clientv3.NewLease(client)
	watcher = clientv3.NewWatcher(client)
	return client, kv, lease, watcher, err
}
