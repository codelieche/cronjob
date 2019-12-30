package worker

import (
	"log"
	"testing"
	"time"
)

func TestRegister_post(t *testing.T) {
	register, err := newRegister()
	if err != nil {
		t.Error(err)
		return
	}

	// 发起注册请求
	log.Println(register.Info)
	if err = register.postWorkerInfoToMaster(); err != nil {
		t.Error(err)
		return
	} else {
		time.Sleep(time.Second * 10)
		log.Println(register.deleteWorkerInfo())
	}

}
