package datasources

import (
	"log"
	"testing"
	"time"
)

func TestGetEtcd(t *testing.T) {
	etcd := GetEtcd()
	log.Println(etcd)
}

func TestEtcd_NewEtcdLock(t *testing.T) {
	// 1. get etcd
	etcd := GetEtcd()

	// 2. new etcd lock
	if etcdLock, err := etcd.NewEtcdLock("jobs/default/abc"); err != nil {
		t.Error(err)
	} else {
		// 3. 尝试上锁
		if err = etcdLock.TryLock(); err != nil {
			t.Error(err)
		} else {
			log.Println("上锁成功：", etcdLock.Name, etcdLock.LeaseID, etcdLock.Secret)
			// 4. 连续10次续租
			i := 0
			for i < 10 {
				i++
				duration := time.Duration(i*5) * time.Second
				time.Sleep(duration)
				if success, err := etcdLock.KeepAliveOnce(etcdLock.Secret); err != nil {
					log.Println("续租失败：", err)
					t.Error(err)
					return
				} else {
					if success {
						log.Println("续租成功:", i)
					} else {
						log.Println("续租失败")
					}
				}

				time.Sleep(time.Second * 5)
			}

		}
	}

}
