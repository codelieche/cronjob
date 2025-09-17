package controllers

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/codelieche/cronjob/apiserver/pkg/core"
	"github.com/codelieche/cronjob/apiserver/pkg/utils/controllers"
	"github.com/gin-gonic/gin"
)

// LockController 分布式锁控制器
type LockController struct {
	controllers.BaseController
	locker core.Locker
}

// NewLockController 创建LockController实例
func NewLockController(locker core.Locker) *LockController {
	return &LockController{
		locker: locker,
	}
}

// Acquire 获取分布式锁
// @Summary 获取分布式锁
// @Description 获取一个分布式锁，如果锁已被占用则返回失败
// @Tags 分布式锁
// @Accept  json
// @Produce  json
// @Param key query string true "锁的键名"
// @Param expire query int false "过期时间(秒)，默认60秒"
// @Success 200 {object} map[string]string "成功响应，包含key和value"
// @Failure 400 {object} map[string]string "参数错误"
// @Failure 409 {object} map[string]string "锁已被占用"
// @Failure 500 {object} map[string]string "服务器错误"
// @Router /api/v1/lock/acquire [get]
func (controller *LockController) Acquire(c *gin.Context) {
	// 1. 获取请求参数
	key := c.Query("key")
	expireStr := c.DefaultQuery("expire", "60")

	if key == "" {
		controller.HandleError(c, errors.New("key is required"), http.StatusBadRequest)
		return
	}

	// 2. 解析过期时间
	expireSeconds, err := strconv.ParseInt(expireStr, 10, 64)
	if err != nil || expireSeconds <= 0 {
		expireSeconds = 60 // 默认60秒
	}
	expire := time.Duration(expireSeconds) * time.Second

	// 3. 尝试获取锁
	lock, err := controller.locker.TryAcquire(c.Request.Context(), key, expire)
	if err != nil && err != core.ErrLockAlreadyAcquired {
		controller.HandleError(c, err, http.StatusInternalServerError)
		return
	}

	// 5. 如果锁已经被占用
	if lock == nil || err == core.ErrLockAlreadyAcquired {
		response := gin.H{
			"key":     key,
			"success": false,
			"status":  "failed",
			"value":   "",
			"message": err.Error(),
		}
		// controller.HandleError(c, errors.New("lock already acquired"), http.StatusConflict)
		controller.HandleOK(c, response)
		return
	}

	// 6. 返回成功响应
	response := gin.H{
		"key":     lock.Key(),
		"value":   lock.Value(),
		"success": true,
		"status":  "success",
		"message": "lock acquired successfully",
	}

	controller.HandleOK(c, response)
}

// Release 释放分布式锁
// @Summary 释放分布式锁
// @Description 通过key和value释放分布式锁
// @Tags 分布式锁
// @Accept  json
// @Produce  json
// @Param key query string true "锁的键名"
// @Param value query string true "锁的值"
// @Success 200 {object} map[string]string "成功响应"
// @Failure 400 {object} map[string]string "参数错误"
// @Failure 500 {object} map[string]string "服务器错误"
// @Router /api/v1/lock/release [get]
func (controller *LockController) Release(c *gin.Context) {
	// 1. 获取请求参数
	key := c.Query("key")
	value := c.Query("value")

	if key == "" || value == "" {
		controller.HandleError(c, errors.New("key and value are required"), http.StatusBadRequest)
		return
	}

	// 2. 使用ReleaseByKeyAndValue方法释放锁
	err := controller.locker.ReleaseByKeyAndValue(c.Request.Context(), key, value)
	if err != nil {
		controller.HandleError(c, err, http.StatusInternalServerError)
		return
	}

	// 4. 返回成功响应
	response := gin.H{
		"key":     key,
		"status":  "success",
		"message": "lock released successfully",
	}

	controller.HandleOK(c, response)
}

// Check 检查锁状态
// @Summary 检查锁状态
// @Description 检查指定键名的锁是否有效
// @Tags 分布式锁
// @Accept  json
// @Produce  json
// @Param key query string true "锁的键名"
// @Param value query string false "锁的值，如果提供则验证值是否匹配"
// @Success 200 {object} map[string]interface{} "成功响应，包含锁状态"
// @Failure 400 {object} map[string]string "参数错误"
// @Failure 500 {object} map[string]string "服务器错误"
// @Router /api/v1/lock/check [get]
func (controller *LockController) Check(c *gin.Context) {
	// 1. 获取请求参数
	key := c.Query("key")
	value := c.Query("value")

	if key == "" {
		controller.HandleError(c, errors.New("key is required"), http.StatusBadRequest)
		return
	}

	// 2. 使用locker服务检查锁状态
	exists, storedValue, err := controller.locker.CheckLock(c.Request.Context(), key, value)
	if err != nil {
		controller.HandleError(c, err, http.StatusInternalServerError)
		return
	}

	// 3. 返回锁状态
	response := gin.H{
		"key":        key,
		"is_locked":  exists,
		"value":      storedValue,
		"status":     "success",
		"check_time": time.Now(),
	}

	// 如果提供了value参数，添加值匹配信息
	if value != "" {
		response["value_matched"] = exists && storedValue == value
	}

	controller.HandleOK(c, response)
}

// Refresh 续租分布式锁
// @Summary 续租分布式锁
// @Description 延长锁的过期时间
// @Tags 分布式锁
// @Accept  json
// @Produce  json
// @Param key query string true "锁的键名"
// @Param value query string true "锁的值"
// @Param expire query int false "过期时间(秒)，默认60秒"
// @Success 200 {object} map[string]string "成功响应"
// @Failure 400 {object} map[string]string "参数错误"
// @Failure 500 {object} map[string]string "服务器错误"
// @Router /api/v1/lock/refresh [get]
func (controller *LockController) Refresh(c *gin.Context) {
	// 1. 获取请求参数
	key := c.Query("key")
	value := c.Query("value")
	expireStr := c.DefaultQuery("expire", "60")

	if key == "" || value == "" {
		controller.HandleError(c, errors.New("key and value are required"), http.StatusBadRequest)
		return
	}

	// 2. 解析过期时间
	expireSeconds, err := strconv.ParseInt(expireStr, 10, 64)
	if err != nil || expireSeconds <= 0 {
		expireSeconds = 60 // 默认60秒
	}
	// 最大3600s
	if expireSeconds > 3600 {
		expireSeconds = 3600
	}
	expire := time.Duration(expireSeconds) * time.Second

	// 3. 使用locker服务续租锁
	err = controller.locker.RefreshLock(c.Request.Context(), key, value, expire)
	if err != nil {
		// 根据错误类型返回适当的状态码
		if strings.Contains(err.Error(), "not owned by this instance") || strings.Contains(err.Error(), "already expired") {
			controller.HandleError(c, err, http.StatusBadRequest)
		} else {
			controller.HandleError(c, err, http.StatusInternalServerError)
		}
		return
	}

	// 4. 返回成功响应
	response := gin.H{
		"key":     key,
		"value":   value,
		"expire":  expire,
		"success": true,
		"status":  "success",
		"message": "lock refreshed successfully",
	}

	controller.HandleOK(c, response)
}
