package datasources

func Close() {
	// 关闭数据库的连接
	defer db.Close()
}
