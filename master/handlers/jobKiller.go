package handlers

import (
	"encoding/json"
	"log"
	"net/http"

	"cronjob.codelieche/common"

	"github.com/julienschmidt/httprouter"
)

// 杀死计划任务进程
// URL: /job/kill/create
// Method: POST
func JobKill(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	// 1. 定义变量
	var (
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
	name = r.PostForm.Get("name")

	// 3. 添加kill 任务
	if err = jobManager.KillJob(name); err != nil {
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
