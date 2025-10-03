// Package middleware CORS跨域中间件
//
// 提供CORS跨域资源共享支持，允许前端应用访问API
package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// CORSMiddleware CORS跨域中间件
// 处理跨域请求，允许前端应用访问API
// 这个中间件应该在认证中间件之前使用
//
// 使用方式：
//
//	router.Use(middleware.CORSMiddleware())
//	router.Use(middleware.AuthMiddleware())
func CORSMiddleware() gin.HandlerFunc {
	return gin.HandlerFunc(func(c *gin.Context) {
		origin := c.Request.Header.Get("Origin")

		// 设置CORS头
		c.Header("Access-Control-Allow-Origin", origin)
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Origin, Content-Type, Content-Length, Accept-Encoding, Authorization, X-Requested-With, X-TEAM-ID")
		c.Header("Access-Control-Expose-Headers", "Content-Length, Authorization, X-TEAM-ID")
		c.Header("Access-Control-Allow-Credentials", "true")
		c.Header("Access-Control-Max-Age", "86400") // 24小时

		// 处理预检请求
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	})
}
