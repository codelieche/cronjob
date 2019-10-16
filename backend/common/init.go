package common

import (
	"log"
	"os"
)

func init() {
	if err := ParseConfig(); err != nil {
		log.Println(err)
		os.Exit(1)
	}
}
