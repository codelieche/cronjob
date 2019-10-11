package worker

import (
	_ "expvar"
	"fmt"
	"log"
	"net/http"
	"os"
)

func runMonitorWeb() {
	address := fmt.Sprintf(":%d", webMonitorPort)
	log.Println("monitor web address:", address)
	if err := http.ListenAndServe(address, nil); err != nil {
		log.Println("启动web失败：", err.Error())
		os.Exit(1)
	} else {
		log.Println("monitor web exit")
	}
}
