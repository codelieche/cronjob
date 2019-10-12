package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/codelieche/cronjob/backend/common"
	"github.com/julienschmidt/httprouter"
)

// 获取worker的列表
func LogList(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	var (
		logList []*common.JobExecuteLog
		err     error
		page    string
		pageNum int

		responseData []byte
	)
	page = ps.ByName("page")
	if pageNum, err = strconv.Atoi(page); err != nil {
		pageNum = 1
	}
	if pageNum < 1 {
		pageNum = 1
	}

	//	从etcd中获取数据
	if logList, err = logHandler.List(pageNum, 10); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	//	返回列表
	if len(logList) > 0 {
		if responseData, err = json.Marshal(logList); err != nil {
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
