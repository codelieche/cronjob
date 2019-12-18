package app

import (
	"log"
	"os"
	"strconv"
	"time"

	"github.com/codelieche/cronjob/backend/common"

	"github.com/kataras/iris/v12"
	"github.com/kataras/iris/v12/sessions"
	"github.com/kataras/iris/v12/sessions/sessiondb/redis"
)

var sess *sessions.Sessions
var redisDB *redis.Database

func initSession() {
	// 1. 连接Redis
	config := common.Config
	cfg := redis.Config{
		Network:   "tcp",
		Addr:      config.Master.Redis.Host,
		Clusters:  config.Master.Redis.Clusters,
		Password:  "",
		Database:  strconv.Itoa(config.Master.Redis.DB),
		MaxActive: 10,
		Timeout:   time.Second * 20,
		Prefix:    "",
		Delim:     "-",
		Driver:    nil,
	}
	redisDB = redis.New(cfg)
	//log.Println(redisDB)
	// 检查redis是否连接ok

	defer func() {
		if r := recover(); r != nil {
			log.Println("捕获到错误！连接Redis出错", r)
			os.Exit(1)
		}
	}()

	// 获取redis中：d03b09dd-f1f0-456e-945f-fd0588a577f6-test的值
	// 如果出错，会被recover捕获到，说明redis没起来
	redisDB.Get("d03b09dd-f1f0-456e-945f-fd0588a577f6", "test")

	//	2. 实例化session
	sess = sessions.New(sessions.Config{
		Cookie:                      "sessionid",
		CookieSecureTLS:             false,
		AllowReclaim:                true,
		Encode:                      nil,
		Decode:                      nil,
		Encoding:                    nil,
		Expires:                     time.Minute * 60,
		SessionIDGenerator:          nil,
		DisableSubdomainPersistence: false,
		//Expires:                     time.Second * 10,
	})

	// 3. use database
	sess.UseDatabase(redisDB)
}

func useSessionMiddleware(app *iris.Application) {
	if sess == nil {
		initSession()
	}

	app.Use(sess.Handler())
}
