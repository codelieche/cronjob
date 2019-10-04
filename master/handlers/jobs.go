// Job相关的handler
package handlers

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/codelieche/cronjob/common"
	"github.com/julienschmidt/httprouter"
)

// 创建计划任务Job
// url: /job/create
// Method: POST
func JobCreate(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	var (
		err         error
		contentType string
		job         *common.Job
		//job, prevJob *common.Job
		jobValue []byte
		//data         map[string]string
		needParseForm bool

		name, time, command, description string
	)

	// 1. 解析POST表单
	// contentType默认是这个： application/x-www-form-urlencoded
	contentType = r.Header.Get("Content-Type")
	if strings.Contains(contentType, ";") {
		contentType = strings.Split(contentType, ";")[0]
	}

	log.Println(r.Header.Get("Content-Type"))

	switch contentType {
	case "multipart/form-data":
		// ParseMultipartForm
		// 可以让Form，PostForm得到multipart/form-data的数据

		//log.Println("multipart/form-data")
		if err = r.ParseMultipartForm(1024); err != nil {
			goto ERR
		}
		needParseForm = true
	case "application/x-www-form-urlencoded":
		// application/x-www-form-urlencoded 的话就用ParseForm()
		//log.Println("application/x-www-form-urlencoded")
		if err = r.ParseForm(); err != nil {
			goto ERR
		}

		needParseForm = true

	case "application/json":
		//log.Println("application/json")
		//log.Println(r.Body)
		if err = json.NewDecoder(r.Body).Decode(&job); err != nil {
			goto ERR
		}
	default:
		err = fmt.Errorf("传入的Content-Type有误：%s", contentType)
		goto ERR
	}

	//log.Println(r.PostForm)      // 能得到POST Content-Type是form-urlencoded的(ParseForm)
	//log.Println(r.Form)          // 能得到GET传参的，或者是form-urlencoded的
	//log.Println(r.MultipartForm) // ParseMultipartForm能得到POST传递类型为:multipart/form-data的数据

	// 3. 实例化Job
	if needParseForm {
		// 2. 去表单中的job字段: name, time, command, description
		name = r.PostForm.Get("name")
		time = r.PostForm.Get("time")
		command = r.PostForm.Get("command")
		description = r.PostForm.Get("description")

		job = &common.Job{
			Name:        name,
			Time:        time,
			Command:     command,
			Description: description,
		}
	}

	//log.Println(job)

	// 4. 保存Job到etcd中
	if _, err = jobManager.SaveJob(job); err != nil {
		goto ERR
	} else {
		// 保存成功
		//log.Println(prevJob)
	}

	// 5. 返回正常应答
	// 对job序列号
	if jobValue, err = json.Marshal(job); err != nil {
		goto ERR
	} else {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, string(jobValue))
		return
	}

	// 错误处理
ERR:
	log.Println(err.Error())
	http.Error(w, err.Error(), 500)
	return
}
