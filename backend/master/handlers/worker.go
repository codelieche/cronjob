package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/codelieche/cronjob/backend/common"

	"github.com/julienschmidt/httprouter"
)

// 获取worker的列表
func WorkerList(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	var (
		workerList []*common.WorkerInfo
		err        error

		responseData []byte
	)

	//	从etcd中获取数据
	if workerList, err = workerManager.ListWorekr(); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	//	返回列表
	if len(workerList) > 0 {
		if responseData, err = json.Marshal(workerList); err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
	} else {
		responseData = []byte("[]")
	}

	//	写入响应数据
	w.Header().Set("Content-Type", "application/json")
	w.Write(responseData)
	return

}
