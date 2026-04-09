package entity

import (
	"time"
)

// ==========================================
// 辅助结构体 (用于 JSONB 字段的解析)
// ==========================================

// DialogueMessage 对应训练记录中的单轮对话日志
type DialogueMessage struct {
	Round             int    `json:"round"`
	UserMsg           string `json:"user_msg"`
	CustomerMsg       string `json:"customer_msg"`
	CustomerSentiment string `json:"customer_sentiment"`
	AngerBefore       int    `json:"anger_before"`
	AngerDelta        int    `json:"anger_delta"`
	AngerAfter        int    `json:"anger_after"`
	ExpertCritique    string `json:"expert_critique"`
	PolishReply       string `json:"polish_reply"`
	ReferenceAnswer   string `json:"reference_answer"`
}

// ==========================================
// 数据库实体结构体
// ==========================================

// User 用户表
type User struct {

	// ID: BIGSERIAL -> int64
	ID int64 `gorm:"primaryKey" json:"id"`

	// Username: VARCHAR(50), 唯一, 非空
	Username string `gorm:"type:varchar(50);unique;not null" json:"username"`

	// PasswordHash: 存加密后的串，JSON 输出时建议隐藏，防止误传给前端
	PasswordHash string `gorm:"type:varchar(255);not null" json:"-"`

	// Role: 默认为 user
	Role string `gorm:"type:varchar(20);default:'user'" json:"role"`

	// CreatedAt: 注册时间
	CreatedAt time.Time `gorm:"autoCreateTime" json:"created_at"`

	// 关联关系: 一个用户有多条训练记录
	TrainingRecords []TrainingRecord `gorm:"foreignKey:UserID" json:"training_records,omitempty"`
}

// PromptCategory 类别表
type PromptCategory struct {
	// ID: SERIAL -> int (或 uint)
	ID uint `gorm:"primaryKey" json:"id"`

	// Name: VARCHAR(100)
	Name string `gorm:"type:varchar(100);not null" json:"name"`
}

// Prompt 提示词表
type Prompt struct {
	ID int64 `gorm:"primaryKey" json:"id"`

	// CategoryID: 外键
	CategoryID uint `gorm:"index" json:"category_id"`

	// 关联关系: 属于某个类别
	Category PromptCategory `gorm:"foreignKey:CategoryID" json:"category,omitempty"`

	// Content: 文本内容
	Content string `gorm:"type:text;not null" json:"content"`

	// Note: 可能为空，使用指针 *string。
	// 如果数据库是 NULL，Go 里就是 nil；如果是字符串，Go 里就是指针。
	Note *string `gorm:"type:text" json:"note"`

	CreatedAt time.Time `gorm:"autoCreateTime" json:"created_at"`
}

// TrainingRecord 训练记录表
type TrainingRecord struct {
	ID int64 `gorm:"primaryKey" json:"id"`

	// UserID: 外键，级联删除在数据库层面配置，Go 这里只需定义字段
	UserID int64 `gorm:"index" json:"user_id"`

	// 关联关系: 属于某个用户 (通常查询记录时可能需要带出用户信息)
	User User `gorm:"foreignKey:UserID" json:"user,omitempty"`

	// Score: 允许小数 (例如 100.0 / 99.5)
	Score float64 `gorm:"type:numeric(6,2)" json:"score"`

	// UsedPromptIDs: 存 []int，数据库存 JSONB
	// gorm:"serializer:json" 是 GORM v2 的特性，自动把 Go 切片转为 DB 的 JSON
	UsedPromptIDs []int `gorm:"type:jsonb;serializer:json" json:"used_prompt_ids"`

	// DialogueLog: 存复杂对象数组，数据库存 JSONB
	DialogueLog []DialogueMessage `gorm:"type:jsonb;serializer:json" json:"dialogue_log"`

	// FinishedAt: 训练完成时间
	FinishedAt time.Time `json:"finished_at"`

	// Duration: 耗时 (秒)
	Duration int `json:"duration"`
}
