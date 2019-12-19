package datasources

import (
	"log"
	"testing"
)

func TestGetEtcd(t *testing.T) {
	etcd := GetEtcd()
	log.Println(etcd)
}
