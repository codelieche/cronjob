package controllers

import (
	"github.com/codelieche/cronjob/apiserver/pkg/utils/controllers"
	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// MetricsController Prometheus指标控制器
type MetricsController struct {
	controllers.BaseController
}

// NewMetricsController 创建MetricsController实例
func NewMetricsController() *MetricsController {
	return &MetricsController{}
}

// Metrics 提供Prometheus指标端点
func (mc *MetricsController) Metrics(c *gin.Context) {
	// 使用Prometheus的HTTP处理器
	handler := promhttp.Handler()
	handler.ServeHTTP(c.Writer, c.Request)
}
