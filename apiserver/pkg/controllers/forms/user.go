package forms

import (
	"fmt"
	"regexp"

	"github.com/codelieche/cronjob/apiserver/pkg/core"
)

// UserCreateForm 用户创建表单

type UserCreateForm struct {
	Nickname    string `json:"nickname" form:"nickname"`
	Username    string `json:"name" form:"name" binding:"required"`
	Phone       string `json:"phone" form:"phone"`
	Email       string `json:"email" form:"email"`
	Description string `json:"description" form:"description"`
	Comment     string `json:"comment" form:"comment"`
	WechatID    string `json:"wechat_id" form:"wechat_id"`
	IsActive    *bool  `json:"is_active" form:"is_active"`
}

// Validate 验证表单
func (form *UserCreateForm) Validate() error {
	var err error

	// 1. 验证用户名
	if form.Username == "" {
		err = fmt.Errorf("用户名不能为空")
		return err
	}

	// 2. 验证邮箱格式
	if form.Email != "" {
		emailRegex := regexp.MustCompile(`^[a-z0-9._%+\-]+@[a-z0-9.\-]+\.[a-z]{2,4}$`)
		if !emailRegex.MatchString(form.Email) {
			err = fmt.Errorf("邮箱格式不正确")
			return err
		}
	}

	// 3. 验证手机号格式
	if form.Phone != "" {
		phoneRegex := regexp.MustCompile(`^1[3-9]\d{9}$`)
		if !phoneRegex.MatchString(form.Phone) {
			err = fmt.Errorf("手机号格式不正确")
			return err
		}
	}

	return nil
}

// ToUser 将表单转换为用户模型
func (form *UserCreateForm) ToUser() *core.User {
	isActive := false
	if form.IsActive != nil {
		isActive = *form.IsActive
	} else {
		form.IsActive = &isActive
	}

	return &core.User{
		Nickname:    form.Nickname,
		Username:    form.Username,
		Phone:       form.Phone,
		Email:       form.Email,
		Description: form.Description,
		Comment:     form.Comment,
		WechatID:    form.WechatID,
		IsActive:    form.IsActive,
	}
}

// UserInfoForm 用户信息表单（用于更新）
type UserInfoForm struct {
	Nickname    string `json:"nickname" form:"nickname"`
	Username    string `json:"name" form:"name"`
	Phone       string `json:"phone" form:"phone"`
	Email       string `json:"email" form:"email"`
	Description string `json:"description" form:"description"`
	Comment     string `json:"comment" form:"comment"`
	WechatID    string `json:"wechat_id" form:"wechat_id"`
	IsActive    *bool  `json:"is_active" form:"is_active"`
}

// Validate 验证表单
func (form *UserInfoForm) Validate() error {
	var err error

	// 1. 验证邮箱格式
	if form.Email != "" {
		emailRegex := regexp.MustCompile(`^[a-z0-9._%+\-]+@[a-z0-9.\-]+\.[a-z]{2,4}$`)
		if !emailRegex.MatchString(form.Email) {
			err = fmt.Errorf("邮箱格式不正确")
			return err
		}
	}

	// 2. 验证手机号格式
	if form.Phone != "" {
		phoneRegex := regexp.MustCompile(`^1[3-9]\d{9}$`)
		if !phoneRegex.MatchString(form.Phone) {
			err = fmt.Errorf("手机号格式不正确")
			return err
		}
	}

	return nil
}

// UpdateUser 根据表单更新用户信息
func (form *UserInfoForm) UpdateUser(user *core.User) {
	if form.Nickname != "" {
		user.Nickname = form.Nickname
	}
	if form.Username != "" {
		user.Username = form.Username
	}
	if form.Phone != "" {
		user.Phone = form.Phone
	}
	if form.Email != "" {
		user.Email = form.Email
	}
	if form.Description != "" {
		user.Description = form.Description
	}
	if form.Comment != "" {
		user.Comment = form.Comment
	}
	if form.WechatID != "" {
		user.WechatID = form.WechatID
	}
	if form.IsActive != nil {
		user.IsActive = form.IsActive
	}
}
