// Job相关的handler
package handlers

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/codelieche/cronjob/backend/common"
	"github.com/julienschmidt/httprouter"
)

// 创建计划任务Job
// url: /job/create
// Method: POST
func JobCreate(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	log.Println(r.Method, r.URL)
	var (
		err         error
		contentType string
		job         *common.Job
		//job, prevJob *common.Job
		jobValue []byte
		//data         map[string]string
		needParseForm bool

		category, name, time, command, description string
		isActive                                   string // 是否激活:true 或者 1
		saveOutput                                 string // 是否保存输出：true 或者 1
	)

	// 1. 解析POST表单
	// contentType默认是这个： application/x-www-form-urlencoded
	contentType = r.Header.Get("Content-Type")
	if strings.Contains(contentType, ";") {
		contentType = strings.Split(contentType, ";")[0]
	}

	//log.Println(r.Header.Get("Content-Type"))

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
		job = &common.Job{}
		if err = json.NewDecoder(r.Body).Decode(job); err != nil {
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
		// 2. 取表单中的job字段: name, time, command, description
		category = r.PostForm.Get("category")
		name = r.PostForm.Get("name")
		time = r.PostForm.Get("time")
		command = r.PostForm.Get("command")
		description = r.PostForm.Get("description")
		isActive = r.PostForm.Get("is_active")
		saveOutput = r.PostForm.Get("save_output")

		isActiveValue := false
		if isActive == "true" || isActive == "1" {
			isActiveValue = true
		}

		saveOutputValue := false
		if saveOutput == "true" || saveOutput == "1" {
			saveOutputValue = true
		}

		job = &common.Job{
			Category:    category,
			Name:        name,
			Time:        time,
			Command:     command,
			Description: description,
			IsActive:    isActiveValue,
			SaveOutput:  saveOutputValue,
		}
	}

	// log.Println(job)

	// 4. 保存Job到etcd中
	if _, err = etcdManager.SaveJob(job, true); err != nil {
		goto ERR
	} else {
		// 保存成功
		// 传递的是地址，所以无需重新设置Key，在SaveJob里面会赋值
		//jobKey := fmt.Sprintf("%s%s/%s", common.ETCD_JOBS_DIR, job.Category, name)
		//jobKey = strings.ReplaceAll(jobKey, "//", "/")
		//job.Key = jobKey

		// 可以通过job和返回的prevJob做对比：就可知道哪里做了修改
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
	errCode := 500
	if err == common.NOT_FOUND {
		errCode = 404
	}
	http.Error(w, err.Error(), errCode)
	return
}

// Job详情
func JobDetail(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	log.Println(r.Method, r.URL)
	// 1. 获取到job的name
	var (
		name         string
		category     string
		jobKey       string
		job          *common.Job
		err          error
		jobByteValue []byte
	)
	category = ps.ByName("category")
	name = ps.ByName("name")
	jobKey = fmt.Sprintf("%s%s/%s", common.ETCD_JOBS_DIR, category, name)

	// 2. 从etcd中获取数据
	if job, err = etcdManager.GetJob(jobKey); err != nil {
		http.Error(w, err.Error(), 404)
		return
	} else {
		// 3. 对job序列化
		if jobByteValue, err = json.Marshal(job); err != nil {
			http.Error(w, err.Error(), 500)
			return
		} else {
			w.Header().Set("Content-Type", "application/json")
			w.Write(jobByteValue)
			return
		}
	}
}

// job更新操作
// Method：PUT
// 参数：和create类似
func JobUpdate(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	log.Println(r.Method, r.URL)
	var (
		err         error
		contentType string
		job         *common.Job
		//job, prevJob *common.Job
		jobValue []byte
		//data         map[string]string
		needParseForm bool

		category, name, time, command, description string
		isActive                                   string // 是否激活:true 或者 1
		saveOutput                                 string // 是否保存输出：true 或者 1
	)

	// 1. 解析POST表单
	// contentType默认是这个： application/x-www-form-urlencoded
	contentType = r.Header.Get("Content-Type")
	if strings.Contains(contentType, ";") {
		contentType = strings.Split(contentType, ";")[0]
	}

	// 通过url获取参数
	category = ps.ByName("category")
	name = ps.ByName("name")

	//log.Println(r.Header.Get("Content-Type"))

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
		job = &common.Job{}
		if err = json.NewDecoder(r.Body).Decode(job); err != nil {
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
		// 2. 取表单中的job字段: name, time, command, description
		// category = r.PostForm.Get("category")
		// name = r.PostForm.Get("name")
		time = r.PostForm.Get("time")
		command = r.PostForm.Get("command")
		description = r.PostForm.Get("description")
		isActive = r.PostForm.Get("is_active")
		saveOutput = r.PostForm.Get("save_output")

		isActiveValue := false
		if isActive == "true" || isActive == "1" {
			isActiveValue = true
		}

		saveOutputValue := false
		if saveOutput == "true" || saveOutput == "1" {
			saveOutputValue = true
		}

		job = &common.Job{
			Category:    category,
			Name:        name,
			Time:        time,
			Command:     command,
			Description: description,
			IsActive:    isActiveValue,
			SaveOutput:  saveOutputValue,
		}
	} else {
		// PUT传分类和名字是无效的，Category和Name是不可修改的字段
		job.Category = category
		job.Name = name
	}

	// log.Println(job)

	// 4. 保存Job到etcd中
	if _, err = etcdManager.SaveJob(job, false); err != nil {
		goto ERR
	} else {
		// 保存成功
		// 传递的是地址，所以无需重新设置Key，在SaveJob里面会赋值
		//jobKey := fmt.Sprintf("%s%s/%s", common.ETCD_JOBS_DIR, job.Category, name)
		//jobKey = strings.ReplaceAll(jobKey, "//", "/")
		//job.Key = jobKey

		// 可以通过job和返回的prevJob做对比：就可知道哪里做了修改
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
	errCode := 500
	if err == common.NOT_FOUND {
		errCode = 404
	}
	http.Error(w, err.Error(), errCode)
	return
}

// Job Delete
func JobDelete(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	log.Println(r.Method, r.URL)
	// 定义变量
	var (
		category string
		name     string
		jobKey   string
		success  bool
		err      error
	)

	// 1. 获取到name
	category = ps.ByName("category")
	name = ps.ByName("name")

	jobKey = fmt.Sprintf("%s%s/%s", common.ETCD_JOBS_DIR, category, name)

	// 2. 从etcd中删除key
	if success, err = etcdManager.DeleteJob(jobKey); err != nil {
		http.Error(w, err.Error(), 404)
		return
	} else {
		if success {
			w.WriteHeader(204)
		} else {
			//	不存在
			w.WriteHeader(404)
		}
		return
	}
}

// Job List
func JobList(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	log.Println(r.Method, r.URL)
	var (
		listResponse JobListResponse
		results      []*common.Job
		err          error
		data         []byte
		//prevLastKey  string // 上一页最后一条数据
		pageStr     string // 页码
		page        int    // 页码
		pageSizeStr string // 页面大小
		pageSize    int
	)

	r.ParseForm()
	//prevLastKey = r.FormValue("lastKey")
	pageStr = r.FormValue("page")
	pageSizeStr = r.FormValue("pageSize")

	if pageStr == "" {
		page = 1
	} else {
		if page, err = strconv.Atoi(pageStr); err != nil {
			// 解析页码错误，就设置默认页码为1
			page = 1
		}
	}

	if pageSizeStr == "" {
		pageSize = 10
	} else {
		if pageSize, err = strconv.Atoi(pageSizeStr); err != nil {
			pageSize = 10
		}
	}

	// 获取列表数据
	if results, err = etcdManager.ListJobs(page, pageSize); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	// 对结果序列化
	listResponse = JobListResponse{
		Count:   len(results),
		Next:    "",
		Results: results,
	}

	if data, err = json.Marshal(listResponse); err != nil {
		http.Error(w, err.Error(), 500)
		return
	} else {
		w.Header().Set("Content-Type", "application/json")
		w.Write(data)
		return
	}

}
