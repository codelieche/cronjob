package worker

import (
	"flag"
)

var webMonitorPort int

func parseParams() {
	port := flag.Int("port", 8080, "web端口号")
	flag.Parse()
	webMonitorPort = *port
}
