package controller

import (
	"io"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"training_with_ai/internal/model/dto"
	"training_with_ai/internal/pkg/constants"
	"training_with_ai/internal/pkg/response"
	"training_with_ai/internal/service"
)

// 1. 定义 Controller 结构体
type SessionController struct {
	// 注入 Service 接口
	svc service.SessionService
}

// 2. 构造函数
func NewSessionController(svc service.SessionService) *SessionController {
	return &SessionController{
		svc: svc,
	}
}

// ---------------- 以下为路由 Handler 示例 ----------------
// Create 创建训练
func (c *SessionController) Create(ctx *gin.Context) {
	userID, ok := ctx.Get("user_id")
	if !ok {
		response.Fail(ctx, constants.ErrTokenInvalid)
		return
	}

	var req dto.CreateSessionReq
	if err := ctx.ShouldBindJSON(&req); err != nil {
		response.Fail(ctx, constants.ErrParamInvalid)
		return
	}

	resp, err := c.svc.CreateSession(ctx.Request.Context(), uint64(userID.(int64)), &req)
	if err != nil {
		response.Fail(ctx, err)
		return
	}

	response.Success(ctx, resp)
}

// Stream 建立 SSE 连接
func (c *SessionController) Stream(ctx *gin.Context) {
	sessionID := ctx.Param("id")
	if sessionID == "" {
		response.Fail(ctx, constants.ErrParamInvalid)
		return
	}

	ch, err := c.svc.OpenStream(ctx.Request.Context(), sessionID)
	if err != nil {
		response.Fail(ctx, err)
		return
	}

	ctx.Writer.Header().Set("Content-Type", "text/event-stream")
	ctx.Writer.Header().Set("Cache-Control", "no-cache")
	ctx.Writer.Header().Set("Connection", "keep-alive")
	ctx.Writer.Flush()

	ctx.Stream(func(w io.Writer) bool {
		msg, ok := <-ch
		if !ok {
			return false
		}
		ctx.SSEvent("message", msg)
		return true
	})
}

// Chat 发送消息并触发评分
func (c *SessionController) Chat(ctx *gin.Context) {
	sessionID := ctx.Param("id")
	if sessionID == "" {
		response.Fail(ctx, constants.ErrParamInvalid)
		return
	}

	var req dto.ChatMessageReq
	if err := ctx.ShouldBindJSON(&req); err != nil {
		response.Fail(ctx, constants.ErrParamInvalid)
		return
	}

	resp, err := c.svc.SendMessage(ctx.Request.Context(), sessionID, req.Message)
	if err != nil {
		response.Fail(ctx, err)
		return
	}

	response.Success(ctx, resp)
}

// Terminate 主动停止训练
func (c *SessionController) Terminate(ctx *gin.Context) {
	sessionID := ctx.Param("id")
	if sessionID == "" {
		response.Fail(ctx, constants.ErrParamInvalid)
		return
	}

	// 调用 Service
	if err := c.svc.TerminateSession(ctx.Request.Context(), sessionID); err != nil {
		response.Fail(ctx, err)
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"code": constants.Success.BizCode, "msg": "训练已终止，记录将在5分钟后清除"})
}

// GetDetail 获取训练记录详情
func (c *SessionController) GetDetail(ctx *gin.Context) {
	role, _ := ctx.Get("role")
	isAdmin := role == "admin"

	recordID, err := strconv.ParseInt(ctx.Param("id"), 10, 64)
	if err != nil {
		response.Fail(ctx, constants.ErrParamInvalid)
		return
	}

	resp, err := c.svc.GetRecordDetail(ctx.Request.Context(), recordID, isAdmin)
	if err != nil {
		response.Fail(ctx, err)
		return
	}

	response.Success(ctx, resp)
}

// GetList 获取训练历史列表
func (c *SessionController) GetList(ctx *gin.Context) {
	role, _ := ctx.Get("role")
	isAdmin := role == "admin"
	userID, _ := ctx.Get("user_id")

	var req dto.AdminRecordFilterReq
	if err := ctx.ShouldBindQuery(&req); err != nil {
		response.Fail(ctx, constants.ErrParamInvalid)
		return
	}

	resp, err := c.svc.ListRecords(ctx.Request.Context(), userID.(int64), isAdmin, req)
	if err != nil {
		response.Fail(ctx, err)
		return
	}

	response.Success(ctx, resp)
}

// Delete 删除训练记录
func (c *SessionController) Delete(ctx *gin.Context) {
	role, _ := ctx.Get("role")
	isAdmin := role == "admin"
	userID, _ := ctx.Get("user_id")

	recordID, err := strconv.ParseInt(ctx.Param("id"), 10, 64)
	if err != nil {
		response.Fail(ctx, constants.ErrParamInvalid)
		return
	}

	if err := c.svc.DeleteRecord(ctx.Request.Context(), recordID, userID.(int64), isAdmin); err != nil {
		response.Fail(ctx, err)
		return
	}

	response.Success(ctx, nil)
}
