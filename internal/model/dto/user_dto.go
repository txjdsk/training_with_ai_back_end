package dto

import "time"

// AuthLoginReq 登录请求参数
type AuthLoginReq struct {
	Username   string `json:"username" binding:"required,min=3,max=50"`
	Password   string `json:"password" binding:"required,min=6,max=50"`
	RememberMe bool   `json:"rememberMe"` // 前端勾选的“记住我”
}

// AuthRegisterReq 注册请求参数
type AuthRegisterReq struct {
	Username string `json:"username" binding:"required,min=3,max=50"`
	Password string `json:"password" binding:"required,min=6,max=50"`
}

// AuthLoginResp 登录成功返回信息
type AuthLoginResp struct {
	Token string   `json:"token"`
	User  UserResp `json:"user"`
}

// UserResp 用户基础信息响应（脱敏，绝对不包含 password_hash）
type UserResp struct {
	ID        uint64    `json:"id"`
	Username  string    `json:"username"`
	Role      string    `json:"role"`
	CreatedAt time.Time `json:"created_at"`
}

// UpdateProfileReq 用户修改个人信息请求
type UpdateProfileReq struct {
	Username    string `json:"username" binding:"omitempty,min=3,max=50"`
	OldPassword string `json:"old_password" binding:"omitempty,min=6"`
	NewPassword string `json:"new_password" binding:"omitempty,min=6"`
}

// AdminUserFilterReq 管理员筛选用户请求参数
type AdminUserFilterReq struct {
	PageReq
	Username  string `form:"username"`   // 模糊搜索
	Role      string `form:"role"`       // 角色筛选
	StartTime string `form:"start_time"` // 创建时间起始 (RFC3339)
	EndTime   string `form:"end_time"`   // 创建时间结束 (RFC3339)
}

// AdminCreateUserReq 管理员新增用户
type AdminCreateUserReq struct {
	Username string `json:"username" binding:"required,min=3,max=50"`
	Password string `json:"password" binding:"required,min=6,max=50"`
	Role     string `json:"role" binding:"omitempty,oneof=user admin root"`
}

// AdminUpdateUserReq 管理员修改用户
type AdminUpdateUserReq struct {
	Username string `json:"username" binding:"omitempty,min=3,max=50"`
	Password string `json:"password" binding:"omitempty,min=6,max=50"`
	Role     string `json:"role" binding:"omitempty,oneof=user admin root"`
}
