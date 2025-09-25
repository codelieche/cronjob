package core

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// Worker 工作节点
type Worker struct {
	ID          uuid.UUID       `json:"id"`
	Name        string          `json:"name"`
	Description string          `json:"description"`
	IsActive    *bool           `json:"is_active" form:"is_active"`
	LastActive  *time.Time      `json:"last_active"`
	Metadata    json.RawMessage `json:"metadata"`
}
