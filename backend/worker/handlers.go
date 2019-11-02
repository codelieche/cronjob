package worker

import (
	"encoding/json"
	"net/http"

	"github.com/julienschmidt/httprouter"
)

// web handlers

// worker节点的信息
func workerInfoHandler(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	var (
		workerInfoData []byte
		err            error
	)

	if workerInfoData, err = json.Marshal(app); err != nil {
		goto ERR
	} else {
		w.Header().Set("Content-Type", "application/json")
		w.Write(workerInfoData)
		return
	}
ERR:
	http.Error(w, err.Error(), 500)
	return
}

// 查看worker分类的列表
func categoriesListHandler(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {

	if data, err := json.Marshal(app.Categories); err != nil {
		http.Error(w, err.Error(), 500)
		return
	} else {
		w.Header().Set("Content-Type", "application/json")
		w.Write(data)
		return
	}
}

// 增加worker的分类
func categoryAddHandler(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	var (
		name    string
		success bool
		err     error
		//contentType string
	)

	r.ParseForm()
	name = r.PostForm.Get("name")
	if name == "" {
		http.Error(w, "传入的分类名为空", 400)
		return
	}

	if success, err = app.addExecuteCategory(name); err != nil {
		http.Error(w, err.Error(), 400)
		return
	} else {
		if success {
			w.Write([]byte("add category success"))
			return
		} else {
			w.Write([]byte("add category false"))
			return
		}

	}
}

// 移除worker的category
func removeCategoryHandler(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	var (
		name   string // 要移除的分类名称
		result bool   // 是否移除成功
		err    error
	)
	name = ps.ByName("name")

	if result, err = app.removeExecuteCategory(name); err != nil {
		http.Error(w, err.Error(), 400)
	} else {
		if result {
			w.WriteHeader(204)
			return
		} else {
			w.WriteHeader(400)
			return
		}
	}

}