package app

import (
	"time"

	"github.com/kataras/iris"
	"github.com/kataras/iris/middleware/basicauth"
)

// 设置Basic Auth仅供测试使用
func appAddBasictAuth(app *iris.Application) {
	authConfig := basicauth.Config{
		Users:   map[string]string{"user01": "password01", "user02": "password02"},
		Realm:   "Atuhorization Required",
		Expires: time.Duration(10) * time.Second,
		OnAsk:   nil,
	}

	authentication := basicauth.New(authConfig)

	//设置authentication
	app.Use(authentication)
}
