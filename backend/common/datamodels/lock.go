package datamodels

// 锁
type Lock struct {
	Name     string `json:"name"`      // 锁的名字
	TTL      int64  `json:"ttl"`       // 锁的时间：time to live的简写
	Password string `json:"password"`  // 锁的密码
	LeaseID  int64  `json:"lease_id"`  // 租约ID
	IsActive bool   `json:"is_active"` // 是否有效
}
