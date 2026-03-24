package dto

import "time"

// ================= 分类 DTO =================
type PromptCategoryReq struct {
	ID   uint   `json:"id"`
	Name string `json:"name" binding:"required,max=100"`
}

type PromptCategoryResp struct {
	ID   uint   `json:"id"`
	Name string `json:"name"`
}

// ================== 提示词列表 DTO =================
// PromptListItem 提示词列表项（前端展示的字段）
type PromptListItem struct {
	ID   uint64 `json:"id"`   // 提示词ID
	Note string `json:"note"` // 提示词备注/场景说明
	// 注意：这里没有 Content 字段，普通用户列表不展示核心指令内容
	Type uint `json:"type"` // 提示词类型（对应筛选的type）

}

// PromptListResp 提示词列表响应DTO
type PromptListResp struct {
	List []PromptListItem `json:"list"` // 筛选后的提示词列表
}

type PromptListReq struct {
	Type   *uint  `form:"type" json:"type"`     // 提示词类型（URL查询参数）
	Search string `form:"search" json:"search"` // 搜索关键词（URL查询参数）
}

// ================= 提示词 DTO =================

// PromptReq 新增/修改提示词请求
type PromptReq struct {
	CategoryID uint   `json:"category_id" binding:"gte=0"`
	Content    string `json:"content" binding:"required"`
	Note       string `json:"note" binding:"omitempty"`
}

// AdminPromptResp 管理员查看的提示词详情（全量数据）
type AdminPromptResp struct {
	ID         uint64    `json:"id"`
	CategoryID uint      `json:"category_id"`
	Content    string    `json:"content"` // 核心指令，管理员可见
	Note       string    `json:"note"`
	CreatedAt  time.Time `json:"created_at"`
}

// UserPromptResp 普通用户查看的提示词详情（脱敏数据）
type UserPromptResp struct {
	ID         uint64 `json:"id"`
	CategoryID uint   `json:"category_id"`
	Note       string `json:"note"` // 普通用户只能看备注/场景说明
	// 注意：这里没有 Content 字段
}

// PromptOptimizeReq AI辅助修改提示词请求
type PromptOptimizeReq struct {
	Requirement string `json:"requirement" binding:"required"` // 用户的修改诉求
}

type PromptOptimizeResp struct {
	PromptID         uint64 `json:"prompt_id"`
	OriginalContent  string `json:"original_content"`
	OptimizedContent string `json:"optimized_content"`
}
