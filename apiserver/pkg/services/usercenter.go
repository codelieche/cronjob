// Package services Usercenter服务实现
//
// 提供与Usercenter系统交互的统一服务层，包括：
//   - 消息发送（站内信）
//   - 用户信息查询
//   - 团队成员查询
//
// 设计理念：
//   - 参考worker中的apiserver service设计
//   - 统一HTTP调用逻辑
//   - 统一错误处理
//   - 便于测试和维护
package services

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/codelieche/cronjob/apiserver/pkg/core"
	"github.com/codelieche/cronjob/apiserver/pkg/utils/logger"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// usercenterService Usercenter服务实现
type usercenterService struct {
	apiURL string       // Usercenter API URL
	apiKey string       // API Key（用于认证）
	client *http.Client // HTTP客户端
}

// NewUsercenterService 创建UsercenterService实例
//
// 参数:
//   - apiURL: Usercenter API地址（如: http://usercenter:9000）
//   - apiKey: API Key（用于认证）
//   - timeout: HTTP请求超时时间（可选，默认10秒）
//
// 返回值:
//   - core.UsercenterService: Usercenter服务接口
func NewUsercenterService(apiURL, apiKey string, timeout time.Duration) core.UsercenterService {
	// 设置默认超时时间
	if timeout == 0 {
		timeout = 10 * time.Second
	}

	return &usercenterService{
		apiURL: apiURL,
		apiKey: apiKey,
		client: &http.Client{
			Timeout: timeout,
		},
	}
}

// CreateMessage 创建单个消息（发送站内信）
//
// 说明:
//   - 调用 POST /api/v1/message/ 接口
//   - 使用 X-API-Key 进行认证
//   - 发送失败会记录日志但不返回错误（避免影响主流程）
func (s *usercenterService) CreateMessage(req *core.MessageCreateRequest) error {
	// 1. 构建请求URL
	url := fmt.Sprintf("%s/message/", s.apiURL) // s.apiURL 已包含 /api/v1

	// 2. 序列化JSON
	jsonData, err := json.Marshal(req)
	if err != nil {
		logger.Error("序列化消息请求失败", zap.Error(err))
		return fmt.Errorf("序列化消息请求失败: %w", err)
	}

	// 3. 创建HTTP请求
	httpReq, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		logger.Error("创建HTTP请求失败", zap.Error(err))
		return fmt.Errorf("创建HTTP请求失败: %w", err)
	}

	// 4. 设置请求头
	httpReq.Header.Set("Content-Type", "application/json")
	if s.apiKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+s.apiKey) // 🔥 使用 Bearer Token 认证
	}

	// 5. 发送请求
	resp, err := s.client.Do(httpReq)
	if err != nil {
		logger.Error("发送HTTP请求失败",
			zap.String("url", url),
			zap.Error(err))
		return fmt.Errorf("发送HTTP请求失败: %w", err)
	}
	defer resp.Body.Close()

	// 6. 读取响应
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		logger.Error("读取响应失败", zap.Error(err))
		return fmt.Errorf("读取响应失败: %w", err)
	}

	// 7. 检查状态码
	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		logger.Error("创建消息失败",
			zap.String("url", url),
			zap.Int("status_code", resp.StatusCode),
			zap.String("response", string(body)))
		return fmt.Errorf("创建消息失败: status=%d, body=%s", resp.StatusCode, string(body))
	}

	// 8. 解析响应
	var result core.UsercenterResponse
	if err := json.Unmarshal(body, &result); err != nil {
		logger.Error("解析响应失败",
			zap.String("response", string(body)),
			zap.Error(err))
		return fmt.Errorf("解析响应失败: %w", err)
	}

	// 9. 检查业务状态码
	if result.Code != 0 {
		logger.Error("创建消息失败（业务错误）",
			zap.Int("code", result.Code),
			zap.String("message", result.Message))
		return fmt.Errorf("创建消息失败: %s", result.Message)
	}

	// 10. 记录成功日志
	logger.Info("创建消息成功",
		zap.String("receiver_id", req.ReceiverID.String()),
		zap.String("platform", req.Platform),
		zap.String("title", req.Title))

	return nil
}

// BatchCreateMessages 批量创建消息
//
// 说明:
//   - 当前实现：循环调用单个接口（简单实现）
//   - 优化方案：调用批量接口 POST /api/v1/message/batch（后续实现）
//   - 单个消息失败不影响其他消息发送
func (s *usercenterService) BatchCreateMessages(reqs []*core.MessageCreateRequest) error {
	if len(reqs) == 0 {
		return nil
	}

	// 方案1: 循环调用单个接口（当前实现）
	// 优点: 简单，不需要修改usercenter API
	// 缺点: 性能较差，网络开销大
	successCount := 0
	failureCount := 0

	for _, req := range reqs {
		if err := s.CreateMessage(req); err != nil {
			failureCount++
			logger.Error("批量创建消息失败（单条）",
				zap.String("receiver_id", req.ReceiverID.String()),
				zap.String("title", req.Title),
				zap.Error(err))
			// 继续发送其他消息，不中断
		} else {
			successCount++
		}
	}

	logger.Info("批量创建消息完成",
		zap.Int("total", len(reqs)),
		zap.Int("success", successCount),
		zap.Int("failure", failureCount))

	// 只要有一个成功就返回nil（避免阻塞主流程）
	if successCount > 0 {
		return nil
	}

	return fmt.Errorf("批量创建消息全部失败，共%d条", len(reqs))

	// TODO: 方案2: 调用批量接口（后续优化）
	// url := fmt.Sprintf("%s/message/batch", s.apiURL) // s.apiURL 已包含 /api/v1
	// batchReq := &core.MessageBatchCreateRequest{Messages: reqs}
	// jsonData, _ := json.Marshal(batchReq)
	// ...
}

// GetUser 获取用户信息
//
// 说明:
//   - 调用 GET /api/v1/user/{id}/ 接口
//   - TODO: 后续实现
func (s *usercenterService) GetUser(userID uuid.UUID) (*core.UsercenterUser, error) {
	// TODO: 实现获取用户信息逻辑
	logger.Warn("GetUser方法未实现", zap.String("user_id", userID.String()))
	return nil, fmt.Errorf("GetUser方法未实现")
}

// GetTeamMembers 获取团队成员列表
//
// 说明:
//   - 调用 GET /api/v1/team/{id}/members/ 接口
//   - TODO: 后续实现
func (s *usercenterService) GetTeamMembers(teamID uuid.UUID) ([]*core.UsercenterUser, error) {
	// TODO: 实现获取团队成员逻辑
	logger.Warn("GetTeamMembers方法未实现", zap.String("team_id", teamID.String()))
	return nil, fmt.Errorf("GetTeamMembers方法未实现")
}

// 确保实现了接口
var _ core.UsercenterService = (*usercenterService)(nil)
