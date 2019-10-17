package common

import (
	"io/ioutil"
	"log"
	"os"
	"regexp"
	"strings"

	"github.com/go-yaml/yaml"
	//"gopkg.in/yaml.v2"
)

var Config *MasterWorkerConfig

// http Web相关的配置
type HttpConfig struct {
	Host    string `json:"host", yaml:"host"`
	Port    int    `json:"port", yaml: "port"`
	Timeout int    `json:"timeout", yaml: "timeout"`
}

// master etcd的相关配置
type EtcdConfig struct {
	Endpoints []string `json:"endpoints", yaml:"endpoints"`
}

// master mongodb config
type MongoConfig struct {
	Hosts    []string `json:"hosts", yaml:"hosts"`
	User     string   `json:"user", yaml:"user"`
	Password string   `json:"password", yaml:"password"`
}

// master相关的配置
type MasterConfig struct {
	Http  *HttpConfig  `json:"http", yaml:"http"`
	Etcd  *EtcdConfig  `json:"etcd", yaml:"etcd"`
	Mongo *MongoConfig `json:"mongo", yaml:"mongo"`
}

// worker相关的配置
type WorkerConfig struct {
	Http       *HttpConfig  `json:"http", yaml:"http"`
	Etcd       *EtcdConfig  `json:"etcd", yaml:"etcd"`
	Mongo      *MongoConfig `json:"mongo", yaml:"mongo"`
	Categories []string     `json:"categories", yaml: "categories"`
}

// Master Worker相关的配置
type MasterWorkerConfig struct {
	Master *MasterConfig `json:"master", yaml:"master"`
	Worker *WorkerConfig `json:"worker", yaml: "worker"`
}

func ParseConfig() (err error) {
	var (
		fileName     string
		masterConfig *MasterConfig
		workerConfig *WorkerConfig
		content      []byte
		contentStr   string
	)

	if Config != nil {
		log.Println(*Config)
		return
	}

	// 获取配置文件: 每次要调试，执行的时候工作路径不同，所以设置成用环境变量来处理
	// 如果传递的最后一个参数是.yaml那么它是配置文件
	if strings.HasSuffix(os.Args[len(os.Args)-1], ".yaml") {
		fileName = os.Args[len(os.Args)-1]
	} else {
		if os.Getenv("CRONJOB_CONFIG_FILENAME") != "" {
			fileName = os.Getenv("CRONJOB_CONFIG_FILENAME")
		} else {
			fileName = "./config.yaml"
		}
	}

	if content, err = ioutil.ReadFile(fileName); err != nil {
		return
	} else {
		contentStr = string(content)
		//log.Println(contentStr)

		// 正则替换环境变量
		r := regexp.MustCompile(`\$\{(.*?)\}`)
		results := r.FindAllStringSubmatch(contentStr, -1)

		for _, envStr := range results {
			var envName, envValue, envDefault string
			if envStr[1] != "" {
				envNameAndDefaultArry := strings.Split(envStr[1], ":")
				envName = envNameAndDefaultArry[0]
				envValue = os.Getenv(envName)
				if len(envNameAndDefaultArry) == 2 {
					envDefault = envNameAndDefaultArry[1]

				}
				if envValue == "" && envDefault != "" {
					envValue = envDefault
				}
			}
			// 对环境变量进行替换
			contentStr = strings.ReplaceAll(contentStr, envStr[0], envValue)
		}

		// 替换完了置换，修改content
		content = []byte(contentStr)

	}

	// 解析配置
	masterConfig = &MasterConfig{
		Http: &HttpConfig{
			Host:    "0.0.0.0",
			Port:    9000,
			Timeout: 5000,
		},
		Etcd: &EtcdConfig{
			Endpoints: []string{"127.0.0.1:2379"},
		},
		Mongo: &MongoConfig{
			Hosts:    []string{"127.0.0.1:27017"},
			User:     "admin",
			Password: "password",
		},
	}

	workerConfig = &WorkerConfig{
		Http: &HttpConfig{
			Host:    "0.0.0.0",
			Port:    9000,
			Timeout: 5000,
		},
		Etcd: &EtcdConfig{
			Endpoints: []string{"127.0.0.1:2379"},
		},
		Mongo: &MongoConfig{
			Hosts:    []string{"127.0.0.1:27017"},
			User:     "admin",
			Password: "password",
		},
	}

	Config = &MasterWorkerConfig{
		Master: masterConfig,
		Worker: workerConfig,
	}

	if err = yaml.Unmarshal(content, Config); err != nil {
		return err
	} else {
		// 解析配置成功
		//if data, e := json.Marshal(Config); e != nil {
		//	log.Println(e)
		//	return
		//} else {
		//	log.Println(string(data))
		//}
	}

	return
}
