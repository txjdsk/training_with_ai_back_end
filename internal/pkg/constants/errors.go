package constants

import "fmt"

// AppError 自定义业务错误结构体
type AppError struct {
	HTTPCode int    // HTTP 状态码 (如 200, 400, 401, 500)
	BizCode  int    // 业务自定义状态码 (如 10001)
	Message  string // 给前端显示的提示语
}

// 必须实现 error 接口
func (e *AppError) Error() string {
	return fmt.Sprintf("BizCode: %d, Message: %s", e.BizCode, e.Message)
}

// ---------------- 预定义全局错误字典 ----------------
var (
	Success               = &AppError{HTTPCode: 200, BizCode: 20000, Message: "success"}
	ErrServerInternal     = &AppError{HTTPCode: 500, BizCode: 50000, Message: "server internal error, please try again later"}
	ErrParamInvalid       = &AppError{HTTPCode: 400, BizCode: 40001, Message: "invalid parameter"}
	ErrPasswordInvalid    = &AppError{HTTPCode: 401, BizCode: 40103, Message: "invalid username or password"}
	ErrUserNotFound       = &AppError{HTTPCode: 404, BizCode: 40401, Message: "user not found"}
	ErrPromptNotFound     = &AppError{HTTPCode: 404, BizCode: 40402, Message: "prompt not found"}
	ErrPromptTypeNotFound = &AppError{HTTPCode: 404, BizCode: 40403, Message: "prompt category not found"}
	ErrPromptTypeExists   = &AppError{HTTPCode: 400, BizCode: 40003, Message: "prompt category already exists"}
	ErrPromptTypeInUse    = &AppError{HTTPCode: 400, BizCode: 40004, Message: "prompt category is in use"}
	ErrSessionNotFound    = &AppError{HTTPCode: 404, BizCode: 40410, Message: "session not found"}
	ErrPromptSelectionBad = &AppError{HTTPCode: 400, BizCode: 40010, Message: "invalid prompt selection"}
	ErrSessionNotOngoing  = &AppError{HTTPCode: 400, BizCode: 40011, Message: "session is not ongoing"}
	ErrRateLimited        = &AppError{HTTPCode: 429, BizCode: 42901, Message: "rate limit exceeded"}
	ErrLLMConfigMissing   = &AppError{HTTPCode: 500, BizCode: 50010, Message: "llm config missing"}
	ErrLLMRequestFailed   = &AppError{HTTPCode: 502, BizCode: 50201, Message: "llm request failed"}
	ErrVectorSearchFailed = &AppError{HTTPCode: 502, BizCode: 50202, Message: "vector search failed"}
	ErrTokenInvalid       = &AppError{HTTPCode: 401, BizCode: 40101, Message: "invalid or expired token"}
	ErrTokenMissing       = &AppError{HTTPCode: 401, BizCode: 40102, Message: "token is required"}
	ErrTokenBlacklisted   = &AppError{HTTPCode: 401, BizCode: 40104, Message: "token has been blacklisted"}
	ErrNoPermission       = &AppError{HTTPCode: 403, BizCode: 40301, Message: "no permission to access this resource"}
	ErrUserAlreadyExists  = &AppError{HTTPCode: 400, BizCode: 40002, Message: "user already exists"}
	// ... 其他业务错误
)
