package apiserver

func NewApiServer() *ApiServer {
	router := newApiRouter()

	apiServer := &ApiServer{
		Router: router,
	}

	return apiServer
}
