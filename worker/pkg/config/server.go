package config

// server 配置
type server struct {
	ApiUrl    string // 服务ApiUrl
	AuthToken string // 认证token
}

// Address 获取server服务监听的地址
func (s *server) Address() string {
	return s.ApiUrl
}

var Server *server

// parseServer 解析server配置
func parseServer() {
	apiUrl := GetDefaultEnv("API_URL", "http://192.168.5.168:8090/api/v1")
	authToken := GetDefaultEnv("AUTH_TOKEN", "")

	Server = &server{
		apiUrl,
		authToken,
	}
}

// parseWeb 解析web配置
func init() {
	parseServer()
}
