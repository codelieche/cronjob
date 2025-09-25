package config

const TaskLockerKeyFormat = "task:lock:%s"

// WebsocketMaxTasksPerMessage Websocket最大消息条数
const WebsocketMaxTasksPerMessage = 5

// WebsocketMessageSeparator Websocket消息分隔符
const WebsocketMessageSeparator = "\x00223399AABB2233CC"

// WebsocketPingInterval ping的间隔
const WebsocketPingInterval = 30 // 单位：秒，与core配置保持一致

const MaxMessageSize = 1024000
