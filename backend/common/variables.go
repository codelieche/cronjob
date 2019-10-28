package common

import (
	"fmt"
)

const JOB_EVENT_PUT = 0    // Job PUT事件
const JOB_EVENT_DELETE = 1 // Job Delete事件
const JOB_EVENT_KILL = 2   // Job Kill事件

// ETCD相关变量
const ETCD_WORKER_DIR = "/crontab/workers/"
const ETCD_JOBS_DIR = "/crontab/jobs/"
const ETCD_JOBS_CATEGORY_DIR = "/crontab/categories/" // 计划任务的分类
const ETCD_JOB_KILL_DIR = "/crontab/kill/"
const ETCD_JOBS_LOCK_DIR = "/crontab/lock/"

// 错误类
var NOT_FOUND = fmt.Errorf("404 not found")
