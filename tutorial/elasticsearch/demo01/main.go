package main

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/elastic/go-elasticsearch/esapi"

	"github.com/elastic/go-elasticsearch"
)

func main() {

	// 连接配置
	config := elasticsearch.Config{
		Addresses: []string{
			"http://127.0.0.1:9200",
		},
		Username: "elastic",
		Password: "password",
	}

	//	连接客户端
	es, err := elasticsearch.NewClient(config)
	if err != nil {
		log.Panic(err)
	}

	//	连接成功，可以操作了
	//	查看Info
	if response, err := es.Info(); err != nil {
		log.Panic(err)
	} else {
		fmt.Println("=== response.String() ===")
		fmt.Println(response.String())
	}

	//	插入条数据
	if res, err := es.Index(
		"study", // Index Name
		strings.NewReader(`{"user": "user 0111", "age": 18}`), // Document Body
	); err != nil {
		log.Panic(err)
	} else {
		defer res.Body.Close()
		log.Println(res.StatusCode, res.Status())
		log.Println(res)
	}

	//	根据_id查询数据: 方式一
	fmt.Println("=== 根据 index, _id 查询数据方式1 ===")
	if res, err := es.GetSource(
		"study",
		"28RVS2wBhNo02APP0CDJ",
	); err != nil {
		log.Panic(err)
	} else {
		log.Println(res)
	}

	fmt.Println("=== 根据 index, _id 查询数据方式2 ===")

	request := esapi.GetRequest{
		Index:        "study",
		DocumentType: "_doc",
		DocumentID:   "LsRfS2wBhNo02APPCCuM",
	}

	if response, err := request.Do(context.TODO(), es); err != nil {
		log.Panic(err)
	} else {
		log.Println(response)
	}

	// 搜索请求
	reqSearch := esapi.SearchRequest{
		Index:        []string{"study"},
		DocumentType: []string{"_doc"},
	}
	// 执行搜索请求
	if response, err := reqSearch.Do(context.TODO(), es); err != nil {
		log.Panic(err)
	} else {
		defer response.Body.Close()
		fmt.Println(response)
	}

}
