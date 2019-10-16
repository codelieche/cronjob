package handlers

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/codelieche/cronjob/backend/common"
	"github.com/julienschmidt/httprouter"
)

// 杀死计划任务进程
// URL: /job/kill/create
// Method: POST
func JobKill(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	// 1. 定义变量
	var (
		category string
		name     string
		err      error
		response common.Response
		data     []byte
	)

	// 2. 解析POST表单
	if err := r.ParseForm(); err != nil {
		goto ERR
	}

	// 得到name字段
	category = r.PostForm.Get("category")
	name = r.PostForm.Get("name")

	if category == "" {
		category = "default"
	}

	// 3. 添加kill 任务

	if err = etcdManager.KillJob(category, name); err != nil {
		goto ERR
	} else {
		// 4. 响应结果
		response = common.Response{
			Status:  true,
			Message: "添加kill数据成功",
		}
		// 序列化
		if data, err = json.Marshal(response); err != nil {
			goto ERR
		} else {
			w.Header().Set("Content-Type", "application/json")
			w.Write(data)
			return
		}
	}
ERR:
	log.Println(err.Error())
	http.Error(w, err.Error(), 500)
	return
}
