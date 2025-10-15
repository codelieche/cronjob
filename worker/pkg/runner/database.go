package runner

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/codelieche/cronjob/worker/pkg/core"
	"github.com/codelieche/cronjob/worker/pkg/utils/logger"
	"github.com/redis/go-redis/v9"
	"github.com/xuri/excelize/v2"
	"go.uber.org/zap"

	_ "github.com/go-sql-driver/mysql"
	_ "github.com/lib/pq"
)

// DatabaseConfig 数据库配置
type DatabaseConfig struct {
	DBType       string        `json:"db_type"`       // 数据库类型：mysql/postgresql/redis
	CredentialID string        `json:"credential_id"` // 凭证ID（username_password类型）
	Host         string        `json:"host"`          // 主机地址
	Port         int           `json:"port"`          // 端口
	Database     string        `json:"database"`      // 数据库名（MySQL/PostgreSQL）或 DB序号（Redis，0-15）
	SQL          string        `json:"sql"`           // SQL语句（MySQL/PostgreSQL）
	Command      string        `json:"command"`       // Redis命令（Redis专用）
	Args         []string      `json:"args"`          // Redis命令参数（Redis专用）
	Params       []interface{} `json:"params"`        // SQL参数（可选，用于参数化查询）
	MaxRows      int           `json:"max_rows"`      // 最大返回/导出行数，默认10000
	ExportExcel  bool          `json:"export_excel"`  // 是否导出Excel（仅SELECT有效）
	// 注意：不再使用独立的 Timeout 字段，而是复用 Task.Timeout
}

// DatabaseRunner 数据库执行器
//
// 支持 MySQL、PostgreSQL、Redis 数据库操作
// 核心功能：
// - SQL 类型智能识别（DQL/DML/DDL/MAINTENANCE）
// - Redis 常见命令支持（GET/SET/HGETALL/KEYS/SCAN/DEL等）
// - Excel 自动导出（SELECT 查询结果）
// - Task Output 机制（供下游任务使用）
type DatabaseRunner struct {
	BaseRunner // 🔥 嵌入基类

	config DatabaseConfig // 数据库配置
}

// NewDatabaseRunner 创建新的 DatabaseRunner
func NewDatabaseRunner() *DatabaseRunner {
	r := &DatabaseRunner{}
	r.InitBase() // 🔥 初始化基类
	return r
}

// ParseArgs 解析任务参数
func (r *DatabaseRunner) ParseArgs(task *core.Task) error {
	r.Lock() // 🔥 使用基类方法
	defer r.Unlock()

	r.Task = task // 🔥 直接访问公共字段

	// 解析 args（JSON 字符串）
	if err := json.Unmarshal([]byte(task.Args), &r.config); err != nil {
		return fmt.Errorf("解析数据库配置失败: %w", err)
	}

	// 验证必填字段
	if r.config.DBType == "" {
		return fmt.Errorf("数据库类型（db_type）不能为空")
	}

	// 验证数据库类型
	supportedTypes := map[string]bool{
		"mysql":      true,
		"postgresql": true,
		"redis":      true,
		// "mongodb": true,  // 未来扩展
	}
	if !supportedTypes[r.config.DBType] {
		return fmt.Errorf("不支持的数据库类型: %s（当前支持: mysql, postgresql, redis）", r.config.DBType)
	}

	if r.config.CredentialID == "" {
		return fmt.Errorf("凭证ID（credential_id）不能为空")
	}

	if r.config.Host == "" {
		return fmt.Errorf("主机地址（host）不能为空")
	}

	if r.config.Port <= 0 {
		// 设置默认端口
		switch r.config.DBType {
		case "mysql":
			r.config.Port = 3306
		case "postgresql":
			r.config.Port = 5432
		case "redis":
			r.config.Port = 6379
		}
	}

	// 验证字段（根据数据库类型）
	if r.config.DBType == "redis" {
		// Redis 验证
		if r.config.Command == "" {
			return fmt.Errorf("Redis命令（command）不能为空")
		}
		// Redis 的 Database 字段是可选的（默认为 0）
		if r.config.Database == "" {
			r.config.Database = "0"
		}
	} else {
		// MySQL/PostgreSQL 验证
		if r.config.Database == "" {
			return fmt.Errorf("数据库名（database）不能为空")
		}
		if r.config.SQL == "" {
			return fmt.Errorf("SQL语句（sql）不能为空")
		}
	}

	// 设置默认值和上限
	if r.config.MaxRows <= 0 {
		r.config.MaxRows = 10000 // 默认 1 万行
	} else if r.config.MaxRows > 100000 {
		return fmt.Errorf("最大行数（max_rows）不能超过 100000，当前值: %d", r.config.MaxRows)
	}

	return nil
}

// Execute 执行数据库操作
func (r *DatabaseRunner) Execute(ctx context.Context, logChan chan<- string) (*core.Result, error) {
	r.Lock()                            // 🔥 使用基类方法
	if r.Status != core.StatusPending { // 🔥 直接访问公共字段
		r.Unlock()
		return nil, fmt.Errorf("任务状态不正确，当前状态: %s", r.Status)
	}

	r.Status = core.StatusRunning // 🔥 直接访问公共字段
	startTime := time.Now()

	// 创建可取消的上下文（使用 Task.Timeout）
	var execCtx context.Context
	var cancel context.CancelFunc

	if r.Task.Timeout > 0 { // 🔥 直接访问公共字段
		// 有超时设置
		execCtx, cancel = context.WithTimeout(ctx, time.Duration(r.Task.Timeout)*time.Second)
	} else {
		// 无超时限制
		execCtx, cancel = context.WithCancel(ctx)
	}
	r.Cancel = cancel // 🔥 直接访问公共字段
	defer cancel()

	r.Unlock() // 🔥 使用基类方法

	// 发送日志
	r.sendLog(logChan, fmt.Sprintf("📊 开始执行数据库操作: %s\n", r.config.DBType))

	// Redis 使用单独的执行路径
	if r.config.DBType == "redis" {
		return r.executeRedis(execCtx, logChan, startTime)
	}

	// MySQL/PostgreSQL 执行路径
	r.sendLog(logChan, fmt.Sprintf("🗄️ 数据库: %s@%s:%d/%s\n", "<用户名>", r.config.Host, r.config.Port, r.config.Database))

	// 1. 获取并验证凭证
	cred, err := r.getAndValidateCredential(logChan, "数据库")
	if err != nil {
		return r.buildErrorResult("凭证获取失败", err, startTime), err
	}

	// 2. 提取凭证信息
	username, ok := cred.GetString("username")
	if !ok || username == "" {
		err := fmt.Errorf("凭证缺少 username 字段")
		r.sendLog(logChan, fmt.Sprintf("❌ %v\n", err))
		return r.buildErrorResult("凭证配置错误", err, startTime), err
	}

	password, ok := cred.GetString("password")
	if !ok {
		err := fmt.Errorf("凭证缺少 password 字段")
		r.sendLog(logChan, fmt.Sprintf("❌ %v\n", err))
		return r.buildErrorResult("凭证配置错误", err, startTime), err
	}

	// 3. 构建 DSN
	dsn, err := r.buildDSN(username, password)
	if err != nil {
		r.sendLog(logChan, fmt.Sprintf("❌ 构建连接字符串失败: %v\n", err))
		return r.buildErrorResult("连接配置错误", err, startTime), err
	}

	// 4. 连接数据库
	r.sendLog(logChan, fmt.Sprintf("🔗 连接数据库: %s@%s:%d/%s\n", username, r.config.Host, r.config.Port, r.config.Database))
	db, err := sql.Open(r.getDriverName(), dsn)
	if err != nil {
		r.sendLog(logChan, fmt.Sprintf("❌ 连接失败: %v\n", err))
		return r.buildErrorResult("数据库连接失败", err, startTime), err
	}
	defer db.Close()

	// 优化连接池配置（针对单次任务执行）
	db.SetMaxOpenConns(1)    // 单次任务只需要一个连接
	db.SetMaxIdleConns(0)    // 任务完成后不保留空闲连接
	db.SetConnMaxLifetime(0) // 不需要连接池，连接随任务结束而关闭
	// 注意：超时已在 DSN 中设置（MySQL: timeout=XXs, PostgreSQL: connect_timeout=XX）

	// 测试连接
	if err := db.PingContext(execCtx); err != nil {
		r.sendLog(logChan, fmt.Sprintf("❌ 数据库不可达: %v\n", err))
		return r.buildErrorResult("数据库连接测试失败", err, startTime), err
	}
	r.sendLog(logChan, "✅ 数据库连接成功\n")

	// 5. 检测 SQL 类型
	sqlType := r.detectSQLType(r.config.SQL)
	r.sendLog(logChan, fmt.Sprintf("📋 SQL类型: %s\n", sqlType))

	// 6. 禁止 DCL 操作
	if sqlType == "DCL_FORBIDDEN" {
		err := fmt.Errorf("禁止执行权限管理操作（GRANT/REVOKE）")
		r.sendLog(logChan, fmt.Sprintf("❌ %v\n", err))
		return r.buildErrorResult("不支持的SQL类型", err, startTime), err
	}

	// 7. 根据 SQL 类型执行
	var result *core.Result
	switch sqlType {
	case "DQL": // SELECT
		result, err = r.executeDQL(execCtx, db, logChan, startTime)
	case "DML": // INSERT/UPDATE/DELETE
		result, err = r.executeDML(execCtx, db, logChan, startTime)
	case "DDL": // CREATE/DROP/ALTER
		result, err = r.executeDDL(execCtx, db, logChan, startTime)
	case "MAINTENANCE": // OPTIMIZE/VACUUM/ANALYZE
		result, err = r.executeMaintenance(execCtx, db, logChan, startTime)
	default:
		err := fmt.Errorf("未知的SQL类型: %s", sqlType)
		r.sendLog(logChan, fmt.Sprintf("❌ %v\n", err))
		return r.buildErrorResult("不支持的SQL类型", err, startTime), err
	}

	if err != nil {
		r.sendLog(logChan, fmt.Sprintf("❌ SQL执行失败: %v\n", err))
		return result, err
	}

	// 10. 更新状态
	r.Lock()                      // 🔥 使用基类方法
	r.Status = core.StatusSuccess // 🔥 直接访问公共字段
	r.Result = result             // 🔥 直接访问公共字段
	r.Unlock()                    // 🔥 使用基类方法

	endTime := time.Now()
	r.sendLog(logChan, fmt.Sprintf("✅ 数据库操作完成（耗时: %v）\n", endTime.Sub(startTime)))

	return result, nil
}

// Stop 停止任务
func (r *DatabaseRunner) Stop() error {
	r.Lock() // 🔥 使用基类方法
	defer r.Unlock()

	if r.Cancel != nil { // 🔥 直接访问公共字段
		r.Cancel()
		r.Status = core.StatusStopped                                      // 🔥 直接访问公共字段
		logger.Info("数据库任务已停止", zap.String("task_id", r.Task.ID.String())) // 🔥 直接访问公共字段
	}
	return nil
}

// Kill 强制终止任务
func (r *DatabaseRunner) Kill() error {
	return r.Stop() // 数据库操作 Stop 和 Kill 行为一致
}

// GetStatus, GetResult 方法继承自 BaseRunner (增强版本已移除)

// Cleanup 清理资源
func (r *DatabaseRunner) Cleanup() error {
	r.Lock() // 🔥 使用基类方法
	defer r.Unlock()

	if r.Cancel != nil { // 🔥 直接访问公共字段
		r.Cancel()
	}

	r.Status = core.StatusPending // 🔥 直接访问公共字段
	r.Result = nil                // 🔥 直接访问公共字段

	return nil
}

// SetApiserver 继承自 BaseRunner

// getDriverName 获取数据库驱动名称
func (r *DatabaseRunner) getDriverName() string {
	switch r.config.DBType {
	case "mysql":
		return "mysql"
	case "postgresql":
		return "postgres"
	default:
		return ""
	}
}

// buildDSN 构建数据库连接字符串
func (r *DatabaseRunner) buildDSN(username, password string) (string, error) {
	switch r.config.DBType {
	case "mysql":
		return r.buildMySQLDSN(username, password), nil
	case "postgresql":
		return r.buildPostgresDSN(username, password), nil
	default:
		return "", fmt.Errorf("不支持的数据库类型: %s", r.config.DBType)
	}
}

// buildMySQLDSN 构建 MySQL DSN
func (r *DatabaseRunner) buildMySQLDSN(username, password string) string {
	// username:password@tcp(host:port)/database?charset=utf8mb4&parseTime=True&loc=Local
	// 如果有超时设置，添加 timeout 参数
	if r.Task.Timeout > 0 { // 🔥 直接访问公共字段
		return fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=True&loc=Local&timeout=%ds",
			username, password, r.config.Host, r.config.Port, r.config.Database, r.Task.Timeout)
	}
	return fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=True&loc=Local",
		username, password, r.config.Host, r.config.Port, r.config.Database)
}

// buildPostgresDSN 构建 PostgreSQL DSN
func (r *DatabaseRunner) buildPostgresDSN(username, password string) string {
	// host=localhost port=5432 user=postgres password=secret dbname=mydb sslmode=disable
	// 如果有超时设置，添加 connect_timeout
	if r.Task.Timeout > 0 { // 🔥 直接访问公共字段
		return fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=disable connect_timeout=%d",
			r.config.Host, r.config.Port, username, password, r.config.Database, r.Task.Timeout)
	}
	return fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=disable",
		r.config.Host, r.config.Port, username, password, r.config.Database)
}

// detectSQLType 检测 SQL 类型
func (r *DatabaseRunner) detectSQLType(sql string) string {
	sql = strings.TrimSpace(strings.ToUpper(sql))

	// DQL: 数据查询
	if strings.HasPrefix(sql, "SELECT") ||
		strings.HasPrefix(sql, "SHOW") ||
		strings.HasPrefix(sql, "DESCRIBE") ||
		strings.HasPrefix(sql, "DESC") ||
		strings.HasPrefix(sql, "EXPLAIN") {
		return "DQL"
	}

	// DML: 数据操作
	if strings.HasPrefix(sql, "INSERT") ||
		strings.HasPrefix(sql, "UPDATE") ||
		strings.HasPrefix(sql, "DELETE") {
		return "DML"
	}

	// DDL: 结构变更
	if strings.HasPrefix(sql, "CREATE") ||
		strings.HasPrefix(sql, "DROP") ||
		strings.HasPrefix(sql, "ALTER") ||
		strings.HasPrefix(sql, "TRUNCATE") {
		return "DDL"
	}

	// 维护: 数据库优化
	if strings.HasPrefix(sql, "OPTIMIZE") ||
		strings.HasPrefix(sql, "VACUUM") ||
		strings.HasPrefix(sql, "ANALYZE") ||
		strings.HasPrefix(sql, "REINDEX") {
		return "MAINTENANCE"
	}

	// DCL: 权限管理（禁止）
	if strings.HasPrefix(sql, "GRANT") ||
		strings.HasPrefix(sql, "REVOKE") {
		return "DCL_FORBIDDEN"
	}

	return "UNKNOWN"
}

// executeDQL 执行查询操作（SELECT）
func (r *DatabaseRunner) executeDQL(ctx context.Context, db *sql.DB, logChan chan<- string, startTime time.Time) (*core.Result, error) {
	r.sendLog(logChan, "🔍 执行查询操作...\n")

	// 执行查询
	rows, err := db.QueryContext(ctx, r.config.SQL, r.config.Params...)
	if err != nil {
		return r.buildErrorResult("查询执行失败", err, startTime), err
	}
	defer rows.Close()

	// 获取列名
	columns, err := rows.Columns()
	if err != nil {
		return r.buildErrorResult("获取列名失败", err, startTime), err
	}
	r.sendLog(logChan, fmt.Sprintf("📊 查询列: %v\n", columns))

	// 读取数据
	var results []map[string]interface{}
	rowCount := 0
	maxRows := r.config.MaxRows

	for rows.Next() && rowCount < maxRows {
		// 创建值容器
		values := make([]interface{}, len(columns))
		valuePtrs := make([]interface{}, len(columns))
		for i := range values {
			valuePtrs[i] = &values[i]
		}

		// 扫描行
		if err := rows.Scan(valuePtrs...); err != nil {
			return r.buildErrorResult("读取数据失败", err, startTime), err
		}

		// 构建行数据
		row := make(map[string]interface{})
		for i, col := range columns {
			val := values[i]
			// 处理 []byte 类型（转为字符串）
			if b, ok := val.([]byte); ok {
				row[col] = string(b)
			} else {
				row[col] = val
			}
		}
		results = append(results, row)
		rowCount++
	}

	if err := rows.Err(); err != nil {
		return r.buildErrorResult("读取数据错误", err, startTime), err
	}

	r.sendLog(logChan, fmt.Sprintf("📊 查询结果: %d 行\n", rowCount))

	// Excel 导出（如果配置了）
	var exportFile string
	if r.config.ExportExcel && rowCount > 0 {
		r.sendLog(logChan, "📁 开始导出 Excel...\n")
		exportFile, err = r.exportToExcel(columns, results, logChan)
		if err != nil {
			r.sendLog(logChan, fmt.Sprintf("⚠️ Excel 导出失败: %v\n", err))
			// 导出失败不影响任务成功
		} else {
			r.sendLog(logChan, fmt.Sprintf("✅ Excel 已导出: %s\n", exportFile))
		}
	}

	endTime := time.Now()
	duration := endTime.Sub(startTime).Milliseconds()

	// 构建 JSON 格式的 output
	outputData := map[string]interface{}{
		"sql_type":    "DQL",
		"row_count":   rowCount,
		"columns":     columns,
		"duration_ms": duration,
	}

	if exportFile != "" {
		outputData["export_file"] = exportFile
	}

	outputJSON, _ := json.Marshal(outputData)

	return &core.Result{
		Status:     core.StatusSuccess,
		Output:     string(outputJSON), // JSON 格式，供下游任务使用
		ExecuteLog: fmt.Sprintf("查询成功，返回 %d 行", rowCount),
		StartTime:  startTime,
		EndTime:    endTime,
		Duration:   duration,
		ExitCode:   0,
	}, nil
}

// executeDML 执行数据操作（INSERT/UPDATE/DELETE）
func (r *DatabaseRunner) executeDML(ctx context.Context, db *sql.DB, logChan chan<- string, startTime time.Time) (*core.Result, error) {
	r.sendLog(logChan, "✏️ 执行数据操作...\n")

	// 执行操作
	result, err := db.ExecContext(ctx, r.config.SQL, r.config.Params...)
	if err != nil {
		return r.buildErrorResult("数据操作失败", err, startTime), err
	}

	// 获取影响行数
	affectedRows, _ := result.RowsAffected()
	lastInsertID, _ := result.LastInsertId()

	r.sendLog(logChan, fmt.Sprintf("✅ 影响行数: %d\n", affectedRows))
	if lastInsertID > 0 {
		r.sendLog(logChan, fmt.Sprintf("🆔 最后插入ID: %d\n", lastInsertID))
	}

	endTime := time.Now()
	duration := endTime.Sub(startTime).Milliseconds()

	// 构建 JSON 格式的 output
	outputData := map[string]interface{}{
		"sql_type":       "DML",
		"affected_rows":  affectedRows,
		"last_insert_id": lastInsertID,
		"duration_ms":    duration,
	}
	outputJSON, _ := json.Marshal(outputData)

	return &core.Result{
		Status:     core.StatusSuccess,
		Output:     string(outputJSON),
		ExecuteLog: fmt.Sprintf("数据操作成功，影响 %d 行", affectedRows),
		StartTime:  startTime,
		EndTime:    endTime,
		Duration:   duration,
		ExitCode:   0,
	}, nil
}

// executeDDL 执行结构变更（CREATE/DROP/ALTER）
func (r *DatabaseRunner) executeDDL(ctx context.Context, db *sql.DB, logChan chan<- string, startTime time.Time) (*core.Result, error) {
	r.sendLog(logChan, "🔧 执行结构变更...\n")

	// 执行操作
	_, err := db.ExecContext(ctx, r.config.SQL, r.config.Params...)
	if err != nil {
		return r.buildErrorResult("结构变更失败", err, startTime), err
	}

	r.sendLog(logChan, "✅ 结构变更成功\n")

	endTime := time.Now()
	duration := endTime.Sub(startTime).Milliseconds()

	// 构建 JSON 格式的 output
	outputData := map[string]interface{}{
		"sql_type":    "DDL",
		"duration_ms": duration,
	}
	outputJSON, _ := json.Marshal(outputData)

	return &core.Result{
		Status:     core.StatusSuccess,
		Output:     string(outputJSON),
		ExecuteLog: "结构变更执行成功",
		StartTime:  startTime,
		EndTime:    endTime,
		Duration:   duration,
		ExitCode:   0,
	}, nil
}

// executeMaintenance 执行数据库维护（OPTIMIZE/VACUUM/ANALYZE）
func (r *DatabaseRunner) executeMaintenance(ctx context.Context, db *sql.DB, logChan chan<- string, startTime time.Time) (*core.Result, error) {
	r.sendLog(logChan, "🔨 执行数据库维护...\n")

	// 执行操作
	_, err := db.ExecContext(ctx, r.config.SQL, r.config.Params...)
	if err != nil {
		return r.buildErrorResult("维护操作失败", err, startTime), err
	}

	r.sendLog(logChan, "✅ 维护操作成功\n")

	endTime := time.Now()
	duration := endTime.Sub(startTime).Milliseconds()

	// 构建 JSON 格式的 output
	outputData := map[string]interface{}{
		"sql_type":    "MAINTENANCE",
		"duration_ms": duration,
	}
	outputJSON, _ := json.Marshal(outputData)

	return &core.Result{
		Status:     core.StatusSuccess,
		Output:     string(outputJSON),
		ExecuteLog: "维护操作执行成功",
		StartTime:  startTime,
		EndTime:    endTime,
		Duration:   duration,
		ExitCode:   0,
	}, nil
}

// exportToExcel 导出查询结果到 Excel
func (r *DatabaseRunner) exportToExcel(columns []string, results []map[string]interface{}, logChan chan<- string) (string, error) {
	// 1. 创建导出目录（支持环境变量 + 年月分目录）
	baseDir := os.Getenv("CRONJOB_EXPORT_DIR")
	if baseDir == "" {
		baseDir = "./exports/" // 默认使用当前目录的 exports 子目录
	}

	// 添加年月子目录（YYYYMM 格式，如 202510）
	yearMonth := time.Now().Format("200601")
	exportDir := filepath.Join(baseDir, yearMonth)

	// 创建完整目录路径
	if err := os.MkdirAll(exportDir, 0755); err != nil {
		return "", fmt.Errorf("创建导出目录失败 [%s]: %w", exportDir, err)
	}
	r.sendLog(logChan, fmt.Sprintf("📁 导出目录: %s\n", exportDir))

	// 2. 生成文件名：{task_id}_{timestamp}.xlsx
	timestamp := time.Now().Format("20060102_150405")
	filename := fmt.Sprintf("%s_%s.xlsx", r.Task.ID.String(), timestamp) // 🔥 直接访问公共字段
	filePath := filepath.Join(exportDir, filename)

	r.sendLog(logChan, fmt.Sprintf("📝 正在生成 Excel: %s\n", filename))

	// 3. 创建 Excel 文件
	f := excelize.NewFile()
	defer f.Close()

	sheetName := "Sheet1"
	sheetIndex, err := f.GetSheetIndex(sheetName)
	if err != nil || sheetIndex == -1 {
		sheetIndex, _ = f.NewSheet(sheetName)
	}
	f.SetActiveSheet(sheetIndex)

	// 4. 设置列名样式（加粗 + 背景色）
	headerStyle, err := f.NewStyle(&excelize.Style{
		Font: &excelize.Font{
			Bold: true,
			Size: 11,
		},
		Fill: excelize.Fill{
			Type:    "pattern",
			Color:   []string{"#E0E0E0"},
			Pattern: 1,
		},
		Alignment: &excelize.Alignment{
			Horizontal: "center",
			Vertical:   "center",
		},
	})
	if err != nil {
		return "", fmt.Errorf("创建样式失败: %w", err)
	}

	// 5. 写入列名（第一行）
	for i, col := range columns {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		f.SetCellValue(sheetName, cell, col)
		f.SetCellStyle(sheetName, cell, cell, headerStyle)
	}

	// 6. 写入数据行
	for rowIdx, row := range results {
		for colIdx, col := range columns {
			cell, _ := excelize.CoordinatesToCellName(colIdx+1, rowIdx+2)
			value := row[col]

			// 处理 nil 值
			if value == nil {
				f.SetCellValue(sheetName, cell, "")
			} else {
				f.SetCellValue(sheetName, cell, value)
			}
		}
	}

	// 7. 自动调整列宽（可选）
	for i := range columns {
		colName, _ := excelize.ColumnNumberToName(i + 1)
		f.SetColWidth(sheetName, colName, colName, 15)
	}

	// 8. 保存文件
	if err := f.SaveAs(filePath); err != nil {
		return "", fmt.Errorf("保存 Excel 文件失败: %w", err)
	}

	r.sendLog(logChan, fmt.Sprintf("✅ Excel 导出成功: %d 行 x %d 列\n", len(results), len(columns)))

	return filePath, nil
}

// getAndValidateCredential 获取并验证凭证（内部公共方法）
func (r *DatabaseRunner) getAndValidateCredential(logChan chan<- string, logPrefix string) (*core.Credential, error) {
	// 1. 检查 apiserver 是否已注入
	if r.Apiserver == nil { // 🔥 直接访问公共字段
		err := fmt.Errorf("apiserver 未初始化，无法获取凭证")
		r.sendLog(logChan, fmt.Sprintf("❌ %v\n", err))
		return nil, err
	}

	// 2. 获取凭证
	r.sendLog(logChan, fmt.Sprintf("🔐 获取%s凭证...\n", logPrefix))
	cred, err := r.Apiserver.GetCredential(r.config.CredentialID) // 🔥 直接访问公共字段
	if err != nil {
		r.sendLog(logChan, fmt.Sprintf("❌ 获取凭证失败: %v\n", err))
		return nil, err
	}
	r.sendLog(logChan, fmt.Sprintf("✅ 成功获取凭证: %s\n", cred.Name))

	// 3. 验证凭证类型
	if cred.Category != "username_password" {
		err := fmt.Errorf("凭证类型不匹配：期望 username_password，实际 %s", cred.Category)
		r.sendLog(logChan, fmt.Sprintf("❌ %v\n", err))
		return nil, err
	}

	return cred, nil
}

// executeRedis 执行 Redis 命令
func (r *DatabaseRunner) executeRedis(ctx context.Context, logChan chan<- string, startTime time.Time) (*core.Result, error) {
	// 1. 获取并验证凭证
	cred, err := r.getAndValidateCredential(logChan, "Redis")
	if err != nil {
		return r.buildErrorResult("凭证获取失败", err, startTime), err
	}

	// 2. 提取密码（Redis 不需要用户名，但使用 username_password 凭证类型方便统一）
	password, ok := cred.GetString("password")
	if !ok {
		r.sendLog(logChan, "⚠️ 凭证中未找到 password 字段，将使用空密码连接\n")
		password = ""
	} else if password == "" {
		r.sendLog(logChan, "ℹ️ Redis 使用空密码（无认证）\n")
	}

	// 3. 解析 DB 序号
	dbNum := 0
	if r.config.Database != "" && r.config.Database != "0" {
		if _, err := fmt.Sscanf(r.config.Database, "%d", &dbNum); err != nil {
			r.sendLog(logChan, fmt.Sprintf("⚠️ DB序号格式错误，使用默认值 0\n"))
			dbNum = 0
		}
	}

	// 4. 创建 Redis 客户端
	r.sendLog(logChan, fmt.Sprintf("🔗 连接 Redis: %s:%d (DB:%d)\n", r.config.Host, r.config.Port, dbNum))

	// 设置超时（使用 Task.Timeout，如果为 0 则使用默认的 30 秒）
	timeout := 30 * time.Second
	if r.Task.Timeout > 0 { // 🔥 直接访问公共字段
		timeout = time.Duration(r.Task.Timeout) * time.Second
	}

	rdb := redis.NewClient(&redis.Options{
		Addr:         fmt.Sprintf("%s:%d", r.config.Host, r.config.Port),
		Password:     password,
		DB:           dbNum,
		DialTimeout:  timeout,
		ReadTimeout:  timeout,
		WriteTimeout: timeout,
	})
	defer rdb.Close()

	// 5. 测试连接
	if err := rdb.Ping(ctx).Err(); err != nil {
		r.sendLog(logChan, fmt.Sprintf("❌ Redis 连接失败: %v\n", err))
		return r.buildErrorResult("Redis 连接失败", err, startTime), err
	}
	r.sendLog(logChan, "✅ Redis 连接成功\n")

	// 6. 执行 Redis 命令
	command := strings.ToUpper(r.config.Command)
	args := r.config.Args
	r.sendLog(logChan, fmt.Sprintf("💻 执行命令: %s %v\n", command, args))

	result, err := r.executeRedisCommand(ctx, rdb, command, args, logChan, startTime)
	if err != nil {
		r.sendLog(logChan, fmt.Sprintf("❌ 命令执行失败: %v\n", err))
		return r.buildErrorResult("命令执行失败", err, startTime), err
	}

	return result, nil
}

// executeRedisCommand 执行具体的 Redis 命令
func (r *DatabaseRunner) executeRedisCommand(ctx context.Context, rdb *redis.Client, command string, args []string, logChan chan<- string, startTime time.Time) (*core.Result, error) {
	var cmdResult interface{}
	var err error

	// 根据命令类型执行
	switch command {
	case "GET":
		if len(args) < 1 {
			return nil, fmt.Errorf("GET 命令需要 1 个参数 (key)")
		}
		cmdResult, err = rdb.Get(ctx, args[0]).Result()
		if err == redis.Nil {
			cmdResult = nil // key 不存在
			err = nil
		}

	case "SET":
		if len(args) < 2 {
			return nil, fmt.Errorf("SET 命令需要至少 2 个参数 (key value)")
		}
		// SET key value [EX seconds|PX milliseconds]
		if len(args) == 2 {
			cmdResult, err = rdb.Set(ctx, args[0], args[1], 0).Result()
		} else if len(args) == 4 && args[2] == "EX" {
			// SET key value EX 3600
			var expiration int
			fmt.Sscanf(args[3], "%d", &expiration)
			cmdResult, err = rdb.Set(ctx, args[0], args[1], time.Duration(expiration)*time.Second).Result()
		} else {
			return nil, fmt.Errorf("SET 命令格式错误")
		}

	case "DEL":
		if len(args) < 1 {
			return nil, fmt.Errorf("DEL 命令需要至少 1 个参数 (key...)")
		}
		cmdResult, err = rdb.Del(ctx, args...).Result()

	case "EXISTS":
		if len(args) < 1 {
			return nil, fmt.Errorf("EXISTS 命令需要至少 1 个参数 (key...)")
		}
		cmdResult, err = rdb.Exists(ctx, args...).Result()

	case "KEYS":
		if len(args) < 1 {
			return nil, fmt.Errorf("KEYS 命令需要 1 个参数 (pattern)")
		}
		keys, keyErr := rdb.Keys(ctx, args[0]).Result()
		if keyErr != nil {
			err = keyErr
		} else {
			// 限制返回数量
			if len(keys) > r.config.MaxRows {
				r.sendLog(logChan, fmt.Sprintf("⚠️ 结果过多，仅返回前 %d 个 key\n", r.config.MaxRows))
				keys = keys[:r.config.MaxRows]
			}
			cmdResult = keys
		}

	case "SCAN":
		// SCAN cursor [MATCH pattern] [COUNT count]
		var cursor uint64
		var pattern string = "*"
		var count int64 = 10

		if len(args) >= 1 {
			fmt.Sscanf(args[0], "%d", &cursor)
		}
		if len(args) >= 3 && args[1] == "MATCH" {
			pattern = args[2]
		}
		if len(args) >= 5 && args[3] == "COUNT" {
			fmt.Sscanf(args[4], "%d", &count)
		}

		keys, newCursor, scanErr := rdb.Scan(ctx, cursor, pattern, count).Result()
		if scanErr != nil {
			err = scanErr
		} else {
			cmdResult = map[string]interface{}{
				"cursor": newCursor,
				"keys":   keys,
			}
		}

	case "HGET":
		if len(args) < 2 {
			return nil, fmt.Errorf("HGET 命令需要 2 个参数 (key field)")
		}
		cmdResult, err = rdb.HGet(ctx, args[0], args[1]).Result()
		if err == redis.Nil {
			cmdResult = nil
			err = nil
		}

	case "HGETALL":
		if len(args) < 1 {
			return nil, fmt.Errorf("HGETALL 命令需要 1 个参数 (key)")
		}
		cmdResult, err = rdb.HGetAll(ctx, args[0]).Result()

	case "HSET":
		if len(args) < 3 {
			return nil, fmt.Errorf("HSET 命令需要至少 3 个参数 (key field value [field value ...])")
		}
		// HSET key field value
		values := make([]interface{}, 0, len(args)-1)
		for i := 1; i < len(args); i++ {
			values = append(values, args[i])
		}
		cmdResult, err = rdb.HSet(ctx, args[0], values...).Result()

	case "LPUSH", "RPUSH":
		if len(args) < 2 {
			return nil, fmt.Errorf("%s 命令需要至少 2 个参数 (key element...)", command)
		}
		values := make([]interface{}, 0, len(args)-1)
		for i := 1; i < len(args); i++ {
			values = append(values, args[i])
		}
		if command == "LPUSH" {
			cmdResult, err = rdb.LPush(ctx, args[0], values...).Result()
		} else {
			cmdResult, err = rdb.RPush(ctx, args[0], values...).Result()
		}

	case "LRANGE":
		if len(args) < 3 {
			return nil, fmt.Errorf("LRANGE 命令需要 3 个参数 (key start stop)")
		}
		var start, stop int64
		fmt.Sscanf(args[1], "%d", &start)
		fmt.Sscanf(args[2], "%d", &stop)
		cmdResult, err = rdb.LRange(ctx, args[0], start, stop).Result()

	case "TTL":
		if len(args) < 1 {
			return nil, fmt.Errorf("TTL 命令需要 1 个参数 (key)")
		}
		duration, ttlErr := rdb.TTL(ctx, args[0]).Result()
		if ttlErr != nil {
			err = ttlErr
		} else {
			cmdResult = int64(duration.Seconds())
		}

	case "EXPIRE":
		if len(args) < 2 {
			return nil, fmt.Errorf("EXPIRE 命令需要 2 个参数 (key seconds)")
		}
		var seconds int64
		fmt.Sscanf(args[1], "%d", &seconds)
		cmdResult, err = rdb.Expire(ctx, args[0], time.Duration(seconds)*time.Second).Result()

	case "PING":
		cmdResult, err = rdb.Ping(ctx).Result()

	case "DBSIZE":
		cmdResult, err = rdb.DBSize(ctx).Result()

	case "FLUSHDB":
		// 危险操作，需要确认
		r.sendLog(logChan, "⚠️ FLUSHDB 是危险操作，将清空当前数据库！\n")
		cmdResult, err = rdb.FlushDB(ctx).Result()

	default:
		return nil, fmt.Errorf("不支持的 Redis 命令: %s", command)
	}

	if err != nil {
		return nil, err
	}

	// 构建 Output
	endTime := time.Now()
	duration := endTime.Sub(startTime).Milliseconds()

	outputData := map[string]interface{}{
		"command":     command,
		"args":        args,
		"result":      cmdResult,
		"duration_ms": duration,
	}

	outputJSON, _ := json.MarshalIndent(outputData, "", "  ")
	outputStr := string(outputJSON)

	r.sendLog(logChan, fmt.Sprintf("✅ 命令执行成功\n"))
	r.sendLog(logChan, fmt.Sprintf("📊 结果: %v\n", cmdResult))

	r.Lock()                      // 🔥 使用基类方法
	r.Status = core.StatusSuccess // 🔥 直接访问公共字段
	r.Result = &core.Result{      // 🔥 直接访问公共字段
		Status:     core.StatusSuccess,
		Output:     outputStr,
		ExecuteLog: outputStr,
		StartTime:  startTime,
		EndTime:    endTime,
		Duration:   duration,
		ExitCode:   0,
	}
	r.Unlock() // 🔥 使用基类方法

	return r.Result, nil // 🔥 直接访问公共字段
}

// buildErrorResult 构建错误结果
func (r *DatabaseRunner) buildErrorResult(message string, err error, startTime time.Time) *core.Result {
	endTime := time.Now()
	duration := endTime.Sub(startTime).Milliseconds()

	r.Lock()                     // 🔥 使用基类方法
	r.Status = core.StatusFailed // 🔥 直接访问公共字段
	r.Unlock()                   // 🔥 使用基类方法

	output := fmt.Sprintf("%s: %v", message, err)

	return &core.Result{
		Status:     core.StatusFailed,
		Output:     output,
		Error:      output,
		ExecuteLog: output,
		StartTime:  startTime,
		EndTime:    endTime,
		Duration:   duration,
		ExitCode:   -1,
	}
}

// sendLog 发送日志
func (r *DatabaseRunner) sendLog(logChan chan<- string, message string) {
	if logChan != nil {
		select {
		case logChan <- message:
		default:
			// 通道已满，跳过
		}
	}

	if r.Task != nil { // 🔥 直接访问公共字段
		logger.Info("数据库任务日志",
			zap.String("task_id", r.Task.ID.String()),
			zap.String("message", message),
		)
	}
}

// 确保 DatabaseRunner 实现了 Runner 接口
var _ core.Runner = (*DatabaseRunner)(nil)
