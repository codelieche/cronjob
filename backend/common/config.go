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
	Timeout int    `json:"timeout", yaml: "timeout"` // 超时时间 毫秒
}

type EtcdTLSConfig struct {
	CertFile string `json:"cert_file",yaml:"certfile"`
	KeyFile  string `json:"key_file", yaml:"keyfile"`
	CaFile   string `json:"ca_file", yaml:"cafile"`
}

// master etcd的相关配置
type EtcdConfig struct {
	Endpoints []string       `json:"endpoints", yaml:"endpoints"`
	Timeout   int            `json:"timeout"` // etcd操作的超时时间 秒
	TLS       *EtcdTLSConfig `json:"tls", yaml:"tls"`
}

// master mongodb config
type MongoConfig struct {
	Hosts    []string `json:"hosts" yaml:"hosts"`       // 主机列表
	User     string   `json:"user" yaml:"user"`         // 用户名
	Password string   `json:"password" yaml:"password"` // MongoDB的用户密码
	Database string   `json:"database" yaml:"database"` // 数据库的名字
}

// master相关的配置
type MasterConfig struct {
	Http  *HttpConfig    `json:"http" yaml:"http"`
	MySQL *MySQLDatabase `json:"mysql" yaml:"mysql"`
	Redis *RedisDatabase `json:"redis" yaml:"redis"`
	Etcd  *EtcdConfig    `json:"etcd" yaml:"etcd"`
	Mongo *MongoConfig   `json:"mongo" yaml:"mongo"`
}

// worker相关的配置
type WorkerConfig struct {
	Http       *HttpConfig     `json:"http" yaml:"http"`
	Etcd       *EtcdConfig     `json:"etcd" yaml:"etcd"`
	Mongo      *MongoConfig    `json:"mongo" yaml:"mongo"`
	Categories map[string]bool `json:"categories" yaml: "categories"`
}

// Master Worker相关的配置
type MasterWorkerConfig struct {
	Master *MasterConfig `json:"master" yaml:"master"`
	Worker *WorkerConfig `json:"worker" yaml:"worker"`
	Debug  bool          `json:"debug" yaml:"debug"`
}

// MySQL数据库相关配置
type MySQLDatabase struct {
	Host     string `json:"host" yaml:"host"`         // 数据库地址
	Port     int    `json:"port" yaml:"port"`         // 端口号
	User     string `json:"user" yaml:"user"`         // 用户
	Password string `json:"password" yaml:"password"` // 用户密码
	Database string `json:"database" yaml:"database"` // 数据库
}

// Redis配置
type RedisDatabase struct {
	Host     string   `json:"host" yaml:"host"`         // redis主机，不填会是默认的127.0.0.1：6739
	Clusters []string `json:"clusters" yaml:"clusters"` // Redis集群地址
	Password string   `json:"password" yaml:"password"` // redis的密码
	DB       int      `json:"db" yaml:db`               // 哪个库
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

	// 判断文件是否存在
	if _, err = os.Stat(fileName); err != nil {
		if os.IsNotExist(err) {
			if fileName == "./config.yaml" {
				fileName = "../config.yaml"
			} else {
				log.Println("配置文件不存在：", fileName)
				return
			}
		}
	}
	//log.Println(fileName)

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
		MySQL: &MySQLDatabase{
			Host:     "127.0.0.1",
			Port:     0,
			User:     "root",
			Password: "root",
			Database: "cronjob",
		},
		Etcd: &EtcdConfig{
			Endpoints: []string{"127.0.0.1:2379"},
			Timeout:   5000,
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
			Timeout:   5000,
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
	//log.Println(string(content))
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
		//log.Println(Config.Worker.Etcd)
		if Config.Master.Etcd.Timeout < 1000 {
			Config.Master.Etcd.Timeout = 1000
		}
		if Config.Worker.Etcd.Timeout < 1000 {
			Config.Worker.Etcd.Timeout = 1000
		}
	}

	return
}
