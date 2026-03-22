package dto

// PageReq 通用分页请求参数
type PageReq struct {
	Page int `form:"page" binding:"omitempty,min=1"` // 默认第1页
	Size int `form:"size" binding:"omitempty,min=1"` // 默认10条/20条
}

// PageResp 通用分页响应数据
type PageResp struct {
	Total int64       `json:"total"` // 总条数
	List  interface{} `json:"list"`  // 数据列表
}
