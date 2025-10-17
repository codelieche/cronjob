package runner

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/codelieche/cronjob/worker/pkg/core"
	"github.com/google/uuid"
)

// TestDatabaseRunner_ParseArgs 测试参数解析
func TestDatabaseRunner_ParseArgs(t *testing.T) {
	tests := []struct {
		name    string
		args    string
		wantErr bool
		errMsg  string
	}{
		{
			name: "有效的MySQL配置",
			args: `{
				"db_type": "mysql",
				"credential_id": "test-cred-id",
				"host": "127.0.0.1",
				"port": 3306,
				"database": "testdb",
				"sql": "SELECT * FROM users"
			}`,
			wantErr: false,
		},
		{
			name: "有效的PostgreSQL配置",
			args: `{
				"db_type": "postgresql",
				"credential_id": "test-cred-id",
				"host": "127.0.0.1",
				"port": 5432,
				"database": "testdb",
				"sql": "SELECT * FROM users"
			}`,
			wantErr: false,
		},
		{
			name: "缺少数据库类型",
			args: `{
				"credential_id": "test-cred-id",
				"host": "127.0.0.1",
				"database": "testdb",
				"sql": "SELECT * FROM users"
			}`,
			wantErr: true,
			errMsg:  "数据库类型",
		},
		{
			name: "不支持的数据库类型",
			args: `{
				"db_type": "oracle",
				"credential_id": "test-cred-id",
				"host": "127.0.0.1",
				"database": "testdb",
				"sql": "SELECT * FROM users"
			}`,
			wantErr: true,
			errMsg:  "不支持的数据库类型",
		},
		{
			name: "缺少凭证ID",
			args: `{
				"db_type": "mysql",
				"host": "127.0.0.1",
				"database": "testdb",
				"sql": "SELECT * FROM users"
			}`,
			wantErr: true,
			errMsg:  "凭证ID",
		},
		{
			name: "缺少SQL语句",
			args: `{
				"db_type": "mysql",
				"credential_id": "test-cred-id",
				"host": "127.0.0.1",
				"database": "testdb"
			}`,
			wantErr: true,
			errMsg:  "SQL语句",
		},
		{
			name: "端口自动设置（MySQL）",
			args: `{
				"db_type": "mysql",
				"credential_id": "test-cred-id",
				"host": "127.0.0.1",
				"database": "testdb",
				"sql": "SELECT 1"
			}`,
			wantErr: false,
		},
		{
			name: "MaxRows 超过上限",
			args: `{
				"db_type": "mysql",
				"credential_id": "test-cred-id",
				"host": "127.0.0.1",
				"database": "testdb",
				"sql": "SELECT * FROM users",
				"max_rows": 100001
			}`,
			wantErr: true,
			errMsg:  "不能超过 100000",
		},
		{
			name: "MaxRows 在上限内",
			args: `{
				"db_type": "mysql",
				"credential_id": "test-cred-id",
				"host": "127.0.0.1",
				"database": "testdb",
				"sql": "SELECT * FROM users",
				"max_rows": 100000
			}`,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runner := NewDatabaseRunner()
			task := &core.Task{
				ID:   uuid.New(),
				Args: tt.args,
			}

			err := runner.ParseArgs(task)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseArgs() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErr && err != nil {
				if tt.errMsg != "" && !contains(err.Error(), tt.errMsg) {
					t.Errorf("ParseArgs() error = %v, 期望包含 %v", err, tt.errMsg)
				}
			}

			// 验证默认值设置
			if !tt.wantErr {
				config := runner.config
				// 注意：Timeout 已移除，使用 Task.Timeout
				if config.MaxRows <= 0 {
					t.Error("MaxRows 应该有默认值")
				}
				// 验证端口默认值
				if config.Port == 0 {
					switch config.DBType {
					case "mysql":
						if config.Port != 3306 {
							t.Errorf("MySQL 默认端口应该是 3306，实际: %d", config.Port)
						}
					case "postgresql":
						if config.Port != 5432 {
							t.Errorf("PostgreSQL 默认端口应该是 5432，实际: %d", config.Port)
						}
					}
				}
			}
		})
	}
}

// TestDatabaseRunner_DetectSQLType 测试SQL类型检测
func TestDatabaseRunner_DetectSQLType(t *testing.T) {
	runner := NewDatabaseRunner()

	tests := []struct {
		name     string
		sql      string
		expected string
	}{
		// DQL 测试
		{name: "SELECT查询", sql: "SELECT * FROM users", expected: "DQL"},
		{name: "SELECT查询（小写）", sql: "select id, name from users", expected: "DQL"},
		{name: "SHOW命令", sql: "SHOW TABLES", expected: "DQL"},
		{name: "DESCRIBE命令", sql: "DESCRIBE users", expected: "DQL"},
		{name: "DESC命令", sql: "DESC users", expected: "DQL"},
		{name: "EXPLAIN命令", sql: "EXPLAIN SELECT * FROM users", expected: "DQL"},

		// DML 测试
		{name: "INSERT语句", sql: "INSERT INTO users (name) VALUES ('test')", expected: "DML"},
		{name: "UPDATE语句", sql: "UPDATE users SET name='test' WHERE id=1", expected: "DML"},
		{name: "DELETE语句", sql: "DELETE FROM users WHERE id=1", expected: "DML"},

		// DDL 测试
		{name: "CREATE TABLE", sql: "CREATE TABLE test (id INT)", expected: "DDL"},
		{name: "DROP TABLE", sql: "DROP TABLE test", expected: "DDL"},
		{name: "ALTER TABLE", sql: "ALTER TABLE users ADD COLUMN age INT", expected: "DDL"},
		{name: "TRUNCATE TABLE", sql: "TRUNCATE TABLE users", expected: "DDL"},

		// MAINTENANCE 测试
		{name: "OPTIMIZE", sql: "OPTIMIZE TABLE users", expected: "MAINTENANCE"},
		{name: "VACUUM", sql: "VACUUM ANALYZE users", expected: "MAINTENANCE"},
		{name: "ANALYZE", sql: "ANALYZE TABLE users", expected: "MAINTENANCE"},
		{name: "REINDEX", sql: "REINDEX TABLE users", expected: "MAINTENANCE"},

		// DCL 测试（禁止）
		{name: "GRANT", sql: "GRANT ALL ON users TO test_user", expected: "DCL_FORBIDDEN"},
		{name: "REVOKE", sql: "REVOKE ALL ON users FROM test_user", expected: "DCL_FORBIDDEN"},

		// 带空格和换行的SQL
		{name: "带前置空格", sql: "  SELECT * FROM users", expected: "DQL"},
		{name: "带换行", sql: "\nSELECT * FROM users", expected: "DQL"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := runner.detectSQLType(tt.sql)
			if result != tt.expected {
				t.Errorf("detectSQLType(%q) = %v, 期望 %v", tt.sql, result, tt.expected)
			}
		})
	}
}

// TestDatabaseRunner_BuildDSN 测试DSN构建
func TestDatabaseRunner_BuildDSN(t *testing.T) {
	tests := []struct {
		name     string
		dbType   string
		host     string
		port     int
		database string
		username string
		password string
		wantErr  bool
	}{
		{
			name:     "MySQL DSN",
			dbType:   "mysql",
			host:     "127.0.0.1",
			port:     3306,
			database: "testdb",
			username: "root",
			password: "password",
			wantErr:  false,
		},
		{
			name:     "PostgreSQL DSN",
			dbType:   "postgresql",
			host:     "127.0.0.1",
			port:     5432,
			database: "testdb",
			username: "postgres",
			password: "password",
			wantErr:  false,
		},
		{
			name:     "不支持的数据库类型",
			dbType:   "oracle",
			host:     "127.0.0.1",
			port:     1521,
			database: "testdb",
			username: "user",
			password: "password",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runner := NewDatabaseRunner()
			runner.config = DatabaseConfig{
				DBType:   tt.dbType,
				Host:     tt.host,
				Port:     tt.port,
				Database: tt.database,
			}
			// 注意：Timeout 已移除，使用 Task.Timeout
			runner.Task = &core.Task{Timeout: 60}

			dsn, err := runner.buildDSN(tt.username, tt.password)
			if (err != nil) != tt.wantErr {
				t.Errorf("buildDSN() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				if dsn == "" {
					t.Error("DSN 不应该为空")
				}

				// 验证 DSN 包含必要信息
				if !contains(dsn, tt.username) {
					t.Errorf("DSN 应该包含用户名 %s", tt.username)
				}
				if !contains(dsn, tt.host) {
					t.Errorf("DSN 应该包含主机 %s", tt.host)
				}
				if !contains(dsn, tt.database) {
					t.Errorf("DSN 应该包含数据库名 %s", tt.database)
				}

				t.Logf("生成的 DSN: %s", dsn)
			}
		})
	}
}

// TestDatabaseRunner_GetDriverName 测试驱动名称获取
func TestDatabaseRunner_GetDriverName(t *testing.T) {
	tests := []struct {
		dbType   string
		expected string
	}{
		{dbType: "mysql", expected: "mysql"},
		{dbType: "postgresql", expected: "postgres"},
		{dbType: "unknown", expected: ""},
	}

	for _, tt := range tests {
		t.Run(tt.dbType, func(t *testing.T) {
			runner := NewDatabaseRunner()
			runner.config.DBType = tt.dbType

			result := runner.getDriverName()
			if result != tt.expected {
				t.Errorf("getDriverName() = %v, 期望 %v", result, tt.expected)
			}
		})
	}
}

// TestDatabaseRunner_SetApiserver 测试依赖注入
func TestDatabaseRunner_SetApiserver(t *testing.T) {
	runner := NewDatabaseRunner()
	mockApiserver := &mockApiserverService{}

	runner.SetApiserver(mockApiserver)

	if runner.Apiserver != mockApiserver {
		t.Error("SetApiserver() 应该正确设置 apiserver")
	}
}

// TestDatabaseRunner_GetStatus 测试状态获取
func TestDatabaseRunner_GetStatus(t *testing.T) {
	runner := NewDatabaseRunner()

	// 初始状态应该是 Pending
	if runner.GetStatus() != core.StatusPending {
		t.Errorf("初始状态应该是 Pending，实际: %v", runner.GetStatus())
	}

	// 修改状态
	runner.mutex.Lock()
	runner.Status = core.StatusRunning
	runner.mutex.Unlock()

	if runner.GetStatus() != core.StatusRunning {
		t.Errorf("状态应该是 Running，实际: %v", runner.GetStatus())
	}
}

// TestDatabaseRunner_Cleanup 测试资源清理
func TestDatabaseRunner_Cleanup(t *testing.T) {
	runner := NewDatabaseRunner()

	// 设置一些状态
	ctx, cancel := context.WithCancel(context.Background())
	runner.Cancel = cancel
	runner.Status = core.StatusRunning
	runner.Result = &core.Result{Status: core.StatusSuccess}

	// 执行清理
	err := runner.Cleanup()
	if err != nil {
		t.Errorf("Cleanup() 不应该返回错误: %v", err)
	}

	// 验证清理效果
	if runner.Status != core.StatusPending {
		t.Errorf("清理后状态应该是 Pending，实际: %v", runner.Status)
	}

	if runner.Result != nil {
		t.Error("清理后 result 应该为 nil")
	}

	// 确保 context 被取消
	select {
	case <-ctx.Done():
		// 正确取消
	case <-time.After(100 * time.Millisecond):
		t.Error("context 应该被取消")
	}
}

// TestDatabaseRunner_OutputFormat 测试输出格式
func TestDatabaseRunner_OutputFormat(t *testing.T) {
	tests := []struct {
		name     string
		sqlType  string
		metadata map[string]interface{}
	}{
		{
			name:    "DQL输出格式",
			sqlType: "DQL",
			metadata: map[string]interface{}{
				"sql_type":    "DQL",
				"row_count":   100,
				"columns":     []string{"id", "name", "email"},
				"duration_ms": 250,
				"export_file": "/logs/test.xlsx",
			},
		},
		{
			name:    "DML输出格式",
			sqlType: "DML",
			metadata: map[string]interface{}{
				"sql_type":       "DML",
				"affected_rows":  15,
				"last_insert_id": 1001,
				"duration_ms":    120,
			},
		},
		{
			name:    "DDL输出格式",
			sqlType: "DDL",
			metadata: map[string]interface{}{
				"sql_type":    "DDL",
				"duration_ms": 80,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 序列化为 JSON
			jsonData, err := json.Marshal(tt.metadata)
			if err != nil {
				t.Fatalf("序列化失败: %v", err)
			}

			// 验证可以反序列化
			var output map[string]interface{}
			if err := json.Unmarshal(jsonData, &output); err != nil {
				t.Fatalf("反序列化失败: %v", err)
			}

			// 验证必填字段
			if output["sql_type"] != tt.sqlType {
				t.Errorf("sql_type = %v, 期望 %v", output["sql_type"], tt.sqlType)
			}

			if _, ok := output["duration_ms"]; !ok {
				t.Error("输出应该包含 duration_ms")
			}

			// 根据类型验证特定字段
			switch tt.sqlType {
			case "DQL":
				if _, ok := output["row_count"]; !ok {
					t.Error("DQL 输出应该包含 row_count")
				}
				if _, ok := output["columns"]; !ok {
					t.Error("DQL 输出应该包含 columns")
				}
			case "DML":
				if _, ok := output["affected_rows"]; !ok {
					t.Error("DML 输出应该包含 affected_rows")
				}
			}

			t.Logf("输出格式验证通过: %s", string(jsonData))
		})
	}
}

// mockApiserverService 模拟的 API Server 服务
type mockApiserverService struct{}

func (m *mockApiserverService) GetCategory(category string) (*core.Category, error) {
	return &core.Category{Name: category}, nil
}

func (m *mockApiserverService) GetTask(taskID string) (*core.Task, error) {
	id, _ := uuid.Parse(taskID)
	return &core.Task{ID: id}, nil
}

func (m *mockApiserverService) AppendTaskLog(taskID string, content string) error {
	return nil
}

func (m *mockApiserverService) AcquireLock(key string, expire int) (string, string, error) {
	return key, "mock-value", nil
}

func (m *mockApiserverService) PingWorker(workerID string) error {
	return nil
}

func (m *mockApiserverService) GetCredential(credentialID string) (*core.Credential, error) {
	return &core.Credential{
		ID:       credentialID,
		Category: "username_password",
		Name:     "测试凭证",
		Value: map[string]interface{}{
			"username": "testuser",
			"password": "testpass",
		},
		IsActive: true,
	}, nil
}

func (m *mockApiserverService) CreateApproval(data map[string]interface{}) (string, error) {
	// 返回一个模拟的审批ID
	return uuid.New().String(), nil
}

// contains 函数已在 security_test.go 中定义，这里直接使用
