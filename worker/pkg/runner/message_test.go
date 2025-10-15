package runner

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/codelieche/cronjob/worker/pkg/core"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

// TestMessageRunner_ParseArgs 测试消息配置解析
func TestMessageRunner_ParseArgs(t *testing.T) {
	tests := []struct {
		name    string
		args    string
		wantErr bool
		errMsg  string
	}{
		{
			name: "有效的邮件配置",
			args: `{
				"type": "email",
				"credential_id": "cred-123",
				"to": ["user@example.com"],
				"subject": "测试邮件",
				"content": "邮件内容",
				"content_type": "text"
			}`,
			wantErr: false,
		},
		{
			name: "有效的企业微信应用配置",
			args: `{
				"type": "wechat_work",
				"credential_id": "cred-456",
				"to_user": "UserID1|UserID2",
				"content": "企业微信消息内容"
			}`,
			wantErr: false,
		},
		{
			name: "有效的企业微信机器人配置",
			args: `{
				"type": "wechat_work_bot",
				"credential_id": "cred-789",
				"content": "机器人消息",
				"at_mobiles": ["13800138000"]
			}`,
			wantErr: false,
		},
		{
			name: "有效的飞书机器人配置",
			args: `{
				"type": "feishu_bot",
				"credential_id": "cred-abc",
				"content": "飞书消息",
				"content_type": "markdown",
				"is_at_all": true
			}`,
			wantErr: false,
		},
		{
			name: "缺少type字段",
			args: `{
				"credential_id": "cred-123",
				"content": "消息内容"
			}`,
			wantErr: true,
			errMsg:  "消息类型（type）不能为空",
		},
		{
			name: "缺少credential_id字段",
			args: `{
				"type": "email",
				"content": "消息内容"
			}`,
			wantErr: true,
			errMsg:  "凭证ID（credential_id）不能为空",
		},
		{
			name: "缺少content字段",
			args: `{
				"type": "email",
				"credential_id": "cred-123"
			}`,
			wantErr: true,
			errMsg:  "消息内容（content）不能为空",
		},
		{
			name: "邮件缺少to字段",
			args: `{
				"type": "email",
				"credential_id": "cred-123",
				"content": "邮件内容",
				"subject": "测试"
			}`,
			wantErr: true,
			errMsg:  "邮件接收人（to）不能为空",
		},
		{
			name: "邮件缺少subject字段",
			args: `{
				"type": "email",
				"credential_id": "cred-123",
				"to": ["user@example.com"],
				"content": "邮件内容"
			}`,
			wantErr: true,
			errMsg:  "邮件主题（subject）不能为空",
		},
		{
			name: "企业微信应用缺少接收目标",
			args: `{
				"type": "wechat_work",
				"credential_id": "cred-123",
				"content": "消息内容"
			}`,
			wantErr: true,
			errMsg:  "企业微信应用消息至少需要指定一个接收目标",
		},
		{
			name: "不支持的消息类型",
			args: `{
				"type": "unknown_type",
				"credential_id": "cred-123",
				"content": "消息内容"
			}`,
			wantErr: true,
			errMsg:  "不支持的消息类型",
		},
		{
			name:    "无效的JSON",
			args:    `{invalid json}`,
			wantErr: true,
			errMsg:  "解析消息配置失败",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runner := NewMessageRunner()
			task := &core.Task{
				ID:       uuid.New(),
				Category: "message",
				Command:  "message",
				Args:     tt.args,
			}

			err := runner.ParseArgs(task)

			if tt.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, runner.config)
			}
		})
	}
}

// TestMessageRunner_ParseArgs_DefaultValues 测试默认值设置
func TestMessageRunner_ParseArgs_DefaultValues(t *testing.T) {
	runner := NewMessageRunner()
	task := &core.Task{
		ID:       uuid.New(),
		Category: "message",
		Command:  "message",
		Args: `{
			"type": "email",
			"credential_id": "cred-123",
			"to": ["user@example.com"],
			"subject": "测试",
			"content": "内容"
		}`,
	}

	err := runner.ParseArgs(task)
	assert.NoError(t, err)

	// 验证默认content_type为text
	assert.Equal(t, "text", runner.config.ContentType)
}

// TestMessageRunner_GetExpectedCredentialCategory 测试凭证类型映射
func TestMessageRunner_GetExpectedCredentialCategory(t *testing.T) {
	tests := []struct {
		msgType          string
		expectedCategory string
	}{
		{"email", "email"},
		{"wechat_work", "wechat_work"},
		{"wechat_work_bot", "api_token"},
		{"feishu_bot", "api_token"},
		{"unknown", ""},
	}

	for _, tt := range tests {
		t.Run(tt.msgType, func(t *testing.T) {
			runner := &MessageRunner{
				config: MessageConfig{
					Type: tt.msgType,
				},
			}

			category := runner.getExpectedCredentialCategory()
			assert.Equal(t, tt.expectedCategory, category)
		})
	}
}

// TestMessageRunner_GetTypeLabel 测试消息类型显示名称
func TestMessageRunner_GetTypeLabel(t *testing.T) {
	tests := []struct {
		msgType       string
		expectedLabel string
	}{
		{"email", "邮件"},
		{"wechat_work", "企业微信应用消息"},
		{"wechat_work_bot", "企业微信群机器人"},
		{"feishu_bot", "飞书群机器人"},
		{"unknown", "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.msgType, func(t *testing.T) {
			runner := &MessageRunner{
				config: MessageConfig{
					Type: tt.msgType,
				},
			}

			label := runner.getTypeLabel()
			assert.Equal(t, tt.expectedLabel, label)
		})
	}
}

// TestMessageRunner_BuildErrorResult 测试错误结果构建
func TestMessageRunner_BuildErrorResult(t *testing.T) {
	runner := &MessageRunner{
		BaseRunner: BaseRunner{
			StartTime: time.Now(),
		},
	}

	result := runner.buildErrorResult("测试错误", assert.AnError)

	assert.Equal(t, core.StatusError, result.Status)
	assert.Contains(t, result.Output, "测试错误")
	assert.Contains(t, result.Output, assert.AnError.Error())
	assert.False(t, result.StartTime.IsZero())
	assert.False(t, result.EndTime.IsZero())
}

// TestMessageRunner_InterfaceCompliance 测试接口实现完整性
func TestMessageRunner_InterfaceCompliance(t *testing.T) {
	var _ core.Runner = (*MessageRunner)(nil)
}

// TestMessageConfig_JSONSerialization 测试配置的JSON序列化/反序列化
func TestMessageConfig_JSONSerialization(t *testing.T) {
	original := MessageConfig{
		Type:         "email",
		CredentialID: "cred-123",
		To:           []string{"user1@example.com", "user2@example.com"},
		Subject:      "测试主题",
		Content:      "测试内容",
		ContentType:  "html",
		ToUser:       "UserID1|UserID2",
		ToParty:      "PartyID1",
		ToTag:        "TagID1",
		AtMobiles:    []string{"13800138000"},
		AtUserIds:    []string{"UserID1"},
		IsAtAll:      true,
	}

	// 序列化
	jsonData, err := json.Marshal(original)
	assert.NoError(t, err)

	// 反序列化
	var decoded MessageConfig
	err = json.Unmarshal(jsonData, &decoded)
	assert.NoError(t, err)

	// 验证
	assert.Equal(t, original.Type, decoded.Type)
	assert.Equal(t, original.CredentialID, decoded.CredentialID)
	assert.Equal(t, original.To, decoded.To)
	assert.Equal(t, original.Subject, decoded.Subject)
	assert.Equal(t, original.Content, decoded.Content)
	assert.Equal(t, original.ContentType, decoded.ContentType)
	assert.Equal(t, original.ToUser, decoded.ToUser)
	assert.Equal(t, original.ToParty, decoded.ToParty)
	assert.Equal(t, original.ToTag, decoded.ToTag)
	assert.Equal(t, original.AtMobiles, decoded.AtMobiles)
	assert.Equal(t, original.AtUserIds, decoded.AtUserIds)
	assert.Equal(t, original.IsAtAll, decoded.IsAtAll)
}
