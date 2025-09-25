package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
)

type WorkerConfig struct {
	Tasks []string `json:"tasks"` // 支持处理的tasks类型
}

// Worker 配置
type Worker struct {
	Name        string       `json:"name"`        // 服务名称
	ID          uuid.UUID    `json:"id"`          // 认证token
	Description string       `json:"description"` // 描述
	Metadata    WorkerConfig `json:"metadata"`    // 元数据
}

var WorkerInstance *Worker

// parseWorker 解析worker配置
func parseWorker() {
	// 1. 定义配置文件路径
	configFilePath := "config_worker.json"

	// 2. 尝试从配置文件加载Worker信息
	if _, err := os.Stat(configFilePath); err == nil {
		// 文件存在，尝试读取
		fileContent, readErr := os.ReadFile(configFilePath)
		if readErr == nil {
			// 尝试解析JSON
			var worker Worker
			jsonErr := json.Unmarshal(fileContent, &worker)
			if jsonErr == nil && (worker.ID != uuid.Nil || worker.Name != "") {
				// 解析成功，使用文件中的配置
				WorkerInstance = &worker
				return
			}
		}
	}

	// 3. 如果文件不存在或解析失败，从环境变量创建Worker信息
	name := GetDefaultEnv("WORKER_NAME", "worker："+uuid.New().String())
	description := GetDefaultEnv("WORKER_DESCRIPTION", "cronjob worker")
	workIDStr := GetDefaultEnv("WORKER_ID", "")
	var workerID uuid.UUID
	var err error

	if workIDStr != "" {
		workerID, err = uuid.Parse(workIDStr)
		if err != nil {
			panic(err)
		}
	} else {
		workerID = uuid.New()
	}

	// 4. 解析worker的元数据
	metadataTaskListStr := GetDefaultEnv("WORKER_TASKS_LIST", "default")
	metadataTaskList := strings.Split(metadataTaskListStr, ",")
	var metadata WorkerConfig
	metadata.Tasks = metadataTaskList

	// 5. 创建Worker实例
	WorkerInstance = &Worker{
		Name:        name,
		ID:          workerID,
		Description: description,
		Metadata:    metadata,
	}

	// 6. 将Worker信息保存到配置文件
	data, marshalErr := json.MarshalIndent(WorkerInstance, "", "  ")
	if marshalErr == nil {
		// 确保目录存在
		dir := filepath.Dir(configFilePath)
		if dir != "." {
			os.MkdirAll(dir, 0755)
		}
		writeErr := os.WriteFile(configFilePath, data, 0644)
		if writeErr != nil {
			// 记录错误但不中断程序
			// 注意：这里不能直接使用logger，因为logger可能还未初始化
		}
	}
}

// parseWorker 解析worker配置
func init() {
	parseWorker()
}
