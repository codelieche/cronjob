package handlers

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/codelieche/cronjob/backend/common"
	"github.com/julienschmidt/httprouter"
)

// 计划任务的分类

// 创建分类
func CategoryCreate(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	// 定义变量
	var (
		contentType   string
		needParseForm bool
		category      *common.Category
		categoryData  []byte
		err           error
		name          string // 分类的名称
		isActive      string // 是否激活:true 或者 1
		check         string // 分类命令：检查
		setup         string // 分类命令：初始化worker
		tearDown      string // 分类命令：worker退出执行命令
		description   string // 分类的描述说明

	)

	// 从请求总获取数据: 根据不同的Content-Type做不同的处理
	// 解析postForm
	contentType = r.Header.Get("Content-Type")
	if strings.Contains(contentType, ";") {
		contentType = strings.Split(contentType, ";")[0]
	}

	switch contentType {
	case "multipart/form-data":
		// log.Println("multipart/form-data")
		if err = r.ParseMultipartForm(1024); err != nil {
			goto ERR
		}
		needParseForm = true
	case "application/x-www-form-urlencoded":
		// log.Println("application/x-www-form-urlencoded")
		if err = r.ParseForm(); err != nil {
			goto ERR
		}
		needParseForm = true

	case "application/json":
		// log.Println("application/json")
		// log.Println(r.Body)
		category = &common.Category{}
		if err = json.NewDecoder(r.Body).Decode(category); err != nil {
			goto ERR
		} else {
			// log.Println(category)
		}
	default:
		err = fmt.Errorf("传入的Content-Type有误：%s", contentType)
		goto ERR
	}

	if needParseForm {
		// 取表单中的值
		name = r.PostForm.Get("name")
		isActive = r.PostForm.Get("is_active")
		check = r.PostForm.Get("check_cmd")
		setup = r.PostForm.Get("setup_cmd")
		tearDown = r.PostForm.Get("tear_down_cmd")
		description = r.PostForm.Get("description")

		// 实例化Category
		category = &common.Category{
			Key:         "",
			Name:        name,
			CheckCmd:    check,
			SetupCmd:    setup,
			TearDownCmd: tearDown,
			Description: description,
		}
		// 设置是否激活
		if isActive == "true" || isActive == "1" {
			category.IsActive = true
		}
	}

	// log.Println(category)
	// 保存category到etcd中
	if _, err = etcdManager.SaveCategory(category); err != nil {
		goto ERR
	} else {
		// 返回结果
		if categoryData, err = json.Marshal(category); err != nil {
			goto ERR
		} else {
			w.Header().Set("Content-Type", "application/json")
			w.Write(categoryData)
			return
		}
	}

ERR:
	log.Println(err.Error())
	http.Error(w, err.Error(), 500)
	return
}

// 获取分类
func CategoryDetail(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	// 定义变量
	var (
		name     string           // 分类的名称
		category *common.Category // 分类
		err      error
		errCode  int

		categoryValue []byte
	)
	// 默认错误代码是500
	errCode = 500

	name = ps.ByName("name")
	if name == "" {
		errCode = 400
		err = errors.New("传入的分类名称为空")
		goto ERR
	}

	if category, err = etcdManager.GetCategory(name); err != nil {
		errCode = 404
		err = errors.New("分类未找到")
		goto ERR
	} else {
		// 返回数据响应
		if categoryValue, err = json.Marshal(category); err != nil {
			goto ERR
		} else {
			w.Header().Set("Content-Type", "application/json")
			w.Write(categoryValue)
			return
		}
	}

ERR:
	http.Error(w, err.Error(), errCode)
	return
}

// 获取分类的列表
func CategoriesList(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	// 定义变量
	var (
		categoriesArr  []*common.Category
		categoriesData []byte
		err            error
	)

	if categoriesArr, err = etcdManager.ListCategories(); err != nil {
		goto ERR
	}

	// 对结果序列化分为
	if categoriesData, err = json.Marshal(categoriesArr); err != nil {
		goto ERR
	} else {
		w.Header().Set("Content-Type", "application/json")
		w.Write(categoriesData)
		return
	}
ERR:
	http.Error(w, err.Error(), 400)
	return
}
