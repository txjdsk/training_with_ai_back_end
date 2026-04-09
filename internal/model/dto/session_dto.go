package dto

import "time"

// ============================================================================
// 1. 核心共享结构 (Redis缓存 与 最终给前端展示用的核心数据)
// ============================================================================

// DialogueRound 单轮对话日志结构
type DialogueRound struct {
	Round             int    `json:"round"`              // 当前轮次
	UserMsg           string `json:"user_msg"`           // 真实用户发送的回复
	CustomerMsg       string `json:"customer_msg"`       // 模拟顾客(LLM 1)的回复文本
	CustomerSentiment string `json:"customer_sentiment"` // 模拟顾客情绪评价 (积极/一般积极/一般消极/消极)
	AngerBefore       int    `json:"anger_before"`       // 本轮回复前的怒气值
	AngerDelta        int    `json:"anger_delta"`        // 本轮怒气值增减量 (+15 或 -10 等)
	AngerAfter        int    `json:"anger_after"`        // 本轮结算后的怒气值
	ExpertCritique    string `json:"expert_critique"`    // 客服专家(LLM 2)对本轮的点评
	PolishReply       string `json:"polish_reply"`       // 客服专家(LLM 2)斧正后的应答
	ReferenceAnswer   string `json:"reference_answer"`   // 客服专家(LLM 2)给出的参考回答
}

// SessionCache Redis 中存储的训练会话结构
type SessionCache struct {
	SessionID     string          `json:"session_id"`
	UserID        uint64          `json:"user_id"`
	UsedPromptIDs []uint64        `json:"used_prompt_ids"`
	PromptText    string          `json:"prompt_text"`
	Preview       string          `json:"preview"`
	Difficulty    string          `json:"difficulty"`
	Status        string          `json:"status"`
	CurrentAnger  int             `json:"current_anger"`
	MaxAnger      int             `json:"max_anger"`
	TurnCount     int             `json:"turn_count"`
	CreatedAt     time.Time       `json:"created_at"`
	LastActiveAt  time.Time       `json:"last_active_at"`
	DialogueLog   []DialogueRound `json:"dialogue_log"`
}

// ============================================================================
// 2. 请求 Request DTO (用于接收并校验前端传来的参数)
// ============================================================================

// CreateSessionReq 创建训练请求
type CreateSessionReq struct {
	// 用户选择的场景/角色对应的 Prompt ID 列表，不能为空
	PromptIDs []uint64 `json:"prompt_ids" binding:"required,min=1"`
	// 难度选择，限定只能传 low, normal, high
	Difficulty string `json:"difficulty" binding:"required,oneof=low normal high"`
}

// ChatMessageReq 对话时用户发送的消息 (如果是纯 SSE 也可以通过 URL/Body 传参)
type ChatMessageReq struct {
	Message string `json:"message" binding:"required,max=1000"` // 限制用户一次最多发1000字
}

// AdminRecordFilterReq 管理员筛选训练历史记录的请求参数
type AdminRecordFilterReq struct {
	PageReq            // 继承通用分页 (Page, Size)
	Username  string   `form:"username" binding:"omitempty"`   // 按用户名模糊搜索
	MinScore  *float64 `form:"min_score" binding:"omitempty"`  // 最低分边界 (用指针区分0和未传)
	MaxScore  *float64 `form:"max_score" binding:"omitempty"`  // 最高分边界
	PromptID  uint64   `form:"prompt_id" binding:"omitempty"`  // 按特定提示词查询
	StartTime string   `form:"start_time" binding:"omitempty"` // 完成时间起始 (RFC3339)
	EndTime   string   `form:"end_time" binding:"omitempty"`   // 完成时间结束 (RFC3339)
}

// ============================================================================
// 3. 响应 Response DTO (用于向前端返回脱敏/组装后的数据)
// ============================================================================

// CreateSessionResp 创建训练响应
type CreateSessionResp struct {
	SessionID string `json:"session_id"` // 返回生成的 UUID，前端拿这个去连 SSE
}

// ChatResponse 单轮对话返回结果
type ChatResponse struct {
	Round        DialogueRound `json:"round"`
	Status       string        `json:"status"`
	CurrentAnger int           `json:"current_anger"`
	MaxAnger     int           `json:"max_anger"`
	TurnCount    int           `json:"turn_count"`
}

// SessionSSEEvent SSE实时推送载荷
type SessionSSEEvent struct {
	Event           string `json:"event,omitempty"`
	Round           int    `json:"round,omitempty"`
	Preview         string `json:"preview,omitempty"`
	CustomerMsg     string `json:"customer_msg"`
	CurrentAnger    int    `json:"current_anger"`
	MaxAnger        int    `json:"max_anger"`
	TurnCount       int    `json:"turn_count"`
	Status          string `json:"status"`
	ExpertCritique  string `json:"expert_critique,omitempty"`
	PolishReply     string `json:"polish_reply,omitempty"`
	ReferenceAnswer string `json:"reference_answer,omitempty"`
}

// EvaluateResp 训练结束后的结算/打分响应
type EvaluateResp struct {
	Score      float64 `json:"score"`       // 总得分 (基于你的特殊公式)
	MaxAnger   int     `json:"max_anger"`   // 历史最高怒气值
	TurnCount  int     `json:"turn_count"`  // 总对话轮次
	Duration   int     `json:"duration"`    // 总耗时(秒)
	FinalState string  `json:"final_state"` // 最终结局标识: success(平息), failed(爆表), timeout(超时拖沓)
}

// ---------------- 以下为权限隔离的详情数据 ----------------

// AdminRecordDetailResp 【管理员专属】记录详情视图 (毫无保留的全量数据)
type AdminRecordDetailResp struct {
	ID            string          `json:"id"`
	UserID        uint64          `json:"user_id"`
	Username      string          `json:"username"` // 联表查出的用户名，方便管理员看
	Score         float64         `json:"score"`
	UsedPromptIDs []uint64        `json:"used_prompt_ids"` // 原生暴露提示词 ID
	DialogueLog   []DialogueRound `json:"dialogue_log"`    // 对话全量日志
	FinishedAt    time.Time       `json:"finished_at"`
	Duration      int             `json:"duration"`
}

// UserRecordDetailResp 【普通用户专属】记录详情视图 (数据脱敏)
type UserRecordDetailResp struct {
	ID          string          `json:"id"`
	Score       float64         `json:"score"`
	DialogueLog []DialogueRound `json:"dialogue_log"`
	FinishedAt  time.Time       `json:"finished_at"`
	Duration    int             `json:"duration"`

	// 【重点隔离逻辑】:
	// 普通用户不能看具体的 PromptID，也不能看 Prompt 核心的 Content 指令
	// 只能看这些提示词当时配置的“场景备注 (Note)”
	PromptsNotes []string `json:"prompts_notes"`
}

// ---------------- 列表查询用的简要结构 ----------------

// RecordListItemResp 训练历史列表的单条数据 (不包含庞大的 DialogueLog，节省带宽)
type RecordListItemResp struct {
	ID         string    `json:"id"`
	Username   string    `json:"username,omitempty"` // 管理员看时才有这个字段
	Score      float64   `json:"score"`
	FinishedAt time.Time `json:"finished_at"`
	Duration   int       `json:"duration"`
}
