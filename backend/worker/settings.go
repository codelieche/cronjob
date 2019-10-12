package worker

import (
	"flag"
	_ "testing"
)

var webMonitorPort int

func parseParams() {
	port := flag.Int("port", 8080, "web端口号")
	if !flag.Parsed() {
		flag.Parse()
	}
	webMonitorPort = *port
}
