package config

const SystemCode = "cronjob_apiserver"

// DispatchLockerKeyFormat 计划任务调度的key
const DispatchLockerKeyFormat = "cronjob:dispatch:%s"

// TaskLockerKeyFormat 计划任务执行/操作的key
const TaskLockerKeyFormat = "task:lock:%s"

// WebsocketMaxTasksPerMessage Websocket最大消息条数
const WebsocketMaxTasksPerMessage = 5

// WebsocketMessageSeparator Websocket消息分隔符
const WebsocketMessageSeparator = "\x00223399AABB2233CC"
