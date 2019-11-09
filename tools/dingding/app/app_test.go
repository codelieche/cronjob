package app

// go get github.com/gavv/httpexpect

import (
	"testing"

	"github.com/kataras/iris/httptest"
)

func TestBasicAuth(t *testing.T) {
	app := newApp()
	e := httptest.New(t, app)

	// 访问首页
	e.GET("/").Expect().Status(httptest.StatusUnauthorized)

	e.GET("/").WithBasicAuth("user01", "password01")
}
