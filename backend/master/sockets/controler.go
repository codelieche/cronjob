package sockets

import (
	"errors"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/codelieche/cronjob/backend/common/datamodels"

	"github.com/kataras/iris/v12/mvc"

	"github.com/gorilla/websocket"
	"github.com/kataras/iris/v12"
	"github.com/kataras/iris/v12/sessions"
)

// mvc websocket controller
type WebsocketController struct {
	Ctx     iris.Context
	Session sessions.Session
}

func (c *WebsocketController) Get(ctx iris.Context) {
	// 判断app是否为空
	if app == nil {
		initApp()
	}

	r := ctx.Request()
	w := ctx.ResponseWriter()

	upgrader := websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
		CheckOrigin: func(r *http.Request) bool {
			return true
		},
	}

	if conn, err := upgrader.Upgrade(w, r, nil); err != nil {
		log.Println(err)
	} else {
		//defer conn.Close()
		log.Println("websocket收到连接", conn.RemoteAddr())
		clientStr := fmt.Sprintf("%s", conn.RemoteAddr())
		client := &Client{
			conn:       conn,
			RemoteAddr: conn.RemoteAddr().String(),
			IsActive:   true,
		}

		app.clients[clientStr] = client
		// 启动一个处理不断接收消息的协程
		go readLoop(client)
	}
}

// socket连接
func (c *WebsocketController) GetClient(ctx iris.Context) {
	ctx.ServeFile("./web/templates/socket.html", false)
}

// 查看当前系统中的锁
func (c *WebsocketController) GetEtcdlockList(ctx iris.Context) {
	if app == nil {
		initApp()
	}

	ctx.JSON(app.etcdLocksMap)
}

// 通过post获取锁
// 这个接口，单节点的master没有问题
// 但是如果是多个master, 不好分布式，因为lock信息是存在内存中的，就可能没法续租等【需调整】
func (c *WebsocketController) PostLockSync(ctx iris.Context) mvc.Result {
	// 1. 定义变量
	var (
		etcdLock *datamodels.EtcdLock
		randInt  int
		isExist  bool
		lockName string
		secret   string
		result   datamodels.LockResponse
		err      error
	)

	// 2. 获取变量
	if app == nil {
		initApp()
	}
	lockName = ctx.PostValue("name")
	lockName = strings.TrimSpace(lockName)
	if lockName == "" {
		err = errors.New("锁的名字不可为空")
		goto ERR
	}

	// 3. 开始上锁
	// 3-1：判断是否
	// 如果锁，存在，那么就直接返回
	if etcdLock, isExist = app.etcdLocksMap[lockName]; isExist {
		// 锁存在，直接返回
		//log.Println(etcdLock)
		if etcdLock.IsLocked {
			err = errors.New("锁已经存在当前节点")
			goto ERR
		}
	}

	if etcdLock, err = app.etcd.NewEtcdLock(lockName, 10); err != nil {
		log.Println("获取锁失败：", err)
		goto ERR
	} else {
		//etcdLock.Description = remoteAddr
	}

	// 3. 尝试上锁
	if err = etcdLock.TryLock(); err != nil {
		// log.Println(err)
		goto ERR
	} else {
		// 上锁成功
		// 设置个秘钥
		//	生成个随机数
		rand.Seed(time.Now().UnixNano())
		randInt = rand.Intn(100000000)
		secret = strconv.Itoa(randInt)

		app.opEtcdLockMux.Lock()
		etcdLock.Secret = secret
		app.etcdLocksMap[lockName] = etcdLock
		//log.Println("unlock")
		app.opEtcdLockMux.Unlock()

		// 设置自动过期
		go etcdLock.SetAutoKillTicker(func() {
			// 需要发送kill信息
			//log.Println("到期后，后期处理函数")
			//log.Println(conn)
			releaseLockEventHandler(lockName, nil)
		})

		// 响应结果
		result = datamodels.LockResponse{
			Success: true,
			Name:    lockName,
			Secret:  secret,
			Message: "上锁成功",
		}
		return mvc.Response{
			Code:   200,
			Object: result,
		}

	}

ERR:
	// log.Println(err)
	result = datamodels.LockResponse{
		Success: false,
		Name:    lockName,
		Secret:  "",
		Message: err.Error(),
	}
	return mvc.Response{
		Object: result,
	}
}
