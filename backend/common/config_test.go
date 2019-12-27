package common

import (
	"log"
	"net/url"
	"testing"
)

func TestGetConfig(t *testing.T) {
	config := GetConfig()
	log.Println(*config.Worker)
}

func TestGetConfigCheckMasterUrl(t *testing.T) {
	config := GetConfig()
	log.Println(*config.Worker)
	masterUrlStr := config.Worker.MasterUrl

	if masterUrl, err := url.Parse(masterUrlStr); err != nil {
		t.Error(err.Error())
	} else {
		log.Println(masterUrl.Path, masterUrl.Host, masterUrl.Port(), masterUrl.Scheme)
	}

	log.Println(config.Worker.GetSocketUrl())
}
