package common

import (
	"fmt"
	"io/ioutil"
	"log"
	"net/url"
	"os"
	"regexp"
	"strings"

	"github.com/go-yaml/yaml"
	//"gopkg.in/yaml.v2"
)

var config *Config

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
	Http *HttpConfig `json:"http" yaml:"http"`
	//MySQL *MySQLDatabase `json:"mysql" yaml:"mysql"`
}

// worker相关的配置
type WorkerConfig struct {
	Http       *HttpConfig     `json:"http" yaml:"http"`
	MasterUrl  string          `json:"master_url" yaml:"master_url"`
	Categories map[string]bool `json:"categories" yaml: "categories"`
}

// Master Worker相关的配置
type Config struct {
	Master *MasterConfig  `json:"master" yaml:"master"`
	Worker *WorkerConfig  `json:"worker" yaml:"worker"`
	MySQL  *MySQLDatabase `json:"mysql" yaml:"mysql"`
	Redis  *RedisDatabase `json:"redis" yaml:"redis"`
	Etcd   *EtcdConfig    `json:"etcd" yaml:"etcd"`
	Mongo  *MongoConfig   `json:"mongo" yaml:"mongo"`
	Debug  bool           `json:"debug" yaml:"debug"`
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

	if config != nil {
		log.Println(*config)
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
	}

	workerConfig = &WorkerConfig{
		Http: &HttpConfig{
			Host:    "0.0.0.0",
			Port:    9000,
			Timeout: 5000,
		},
		MasterUrl: "http://127.0.0.1:9000",
	}

	config = &Config{
		Master: masterConfig,
		Worker: workerConfig,
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
	//log.Println(string(content))
	if err = yaml.Unmarshal(content, config); err != nil {
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
		if config.Etcd.Timeout < 1000 {
			config.Etcd.Timeout = 1000
		}
	}

	return
}

// 获取配置
func GetConfig() *Config {
	if config != nil {
		return config
	} else {
		if err := ParseConfig(); err != nil {
			log.Println("解析配置文件出错", err)
			os.Exit(1)
			return nil
		} else {
			return config
		}
	}
}

// 获取socket的连接地址
func (workerConfig *WorkerConfig) GetSocketUrl() (socketUrl string, err error) {
	// 1. 定义变量
	var (
		masterUrl *url.URL
		path      string
		schema    string
		port      string
	)
	workerConfig.MasterUrl = strings.TrimSpace(workerConfig.MasterUrl)
	if workerConfig.MasterUrl == "" {
		workerConfig.MasterUrl = "http://127.0.0.1/"
	}

	if masterUrl, err = url.Parse(workerConfig.MasterUrl); err != nil {
		return "", err
	} else {
		// 获取socket的地址
		if masterUrl.Scheme == "http" {
			schema = "ws"
		} else {
			schema = "wss"
		}

		if strings.HasSuffix(masterUrl.Path, "/") {
			path = fmt.Sprintf("%s%s", masterUrl.Path, "websocket")
		} else {
			path = fmt.Sprintf("%s/%s", masterUrl.Path, "websocket")
		}

		port = masterUrl.Port()
		if masterUrl.Port() != "" {
			socketUrl = fmt.Sprintf("%s://%s:%s%s", schema, masterUrl.Host, port, path)
		} else {
			socketUrl = fmt.Sprintf("%s://%s%s", schema, masterUrl.Host, path)
		}
		return socketUrl, nil
	}

}
