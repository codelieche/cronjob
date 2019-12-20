package common

import (
	"crypto/md5"
	"fmt"
	"math/rand"
	"os"
	"time"
)

// 随机生成个名字
// 创建job，未传入name的时候会调用它
func GenerateName() (string, error) {
	var (
		name            string
		hostName        string
		pid             int
		timeNowUnixNano int64
		randInt         int
		err             error
	)

	timeNowUnixNano = time.Now().UnixNano()
	if hostName, err = os.Hostname(); err != nil {
		hostName = ""
	}
	// 获取进程号
	pid = os.Getppid()

	//	生成个随机数
	rand.Seed(time.Now().UnixNano())
	randInt = rand.Intn(100000000)

	// 得到name
	name = fmt.Sprintf("%s-%d %d %d", hostName, pid, timeNowUnixNano, randInt)

	// Hash:md5
	md5Hash := md5.New()
	md5Hash.Write([]byte(name))
	name = fmt.Sprintf("%x", md5Hash.Sum([]byte("")))
	return name, nil
}
