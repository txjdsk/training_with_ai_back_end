package controller

import (
	"strconv"
	"training_with_ai/internal/model/dto"
	"training_with_ai/internal/pkg/constants"
	"training_with_ai/internal/pkg/response"
	"training_with_ai/internal/service"

	"github.com/gin-gonic/gin"
)

// 定义 Controller 结构体
type PromptController struct {
	// 注入 Service 接口
	svc service.PromptService
}

func NewPromptController(svc service.PromptService) *PromptController {
	return &PromptController{
		svc: svc,
	}
}

// handler方法
func (c *PromptController) GetList(ctx *gin.Context) {
	// 1. 解析查询参数
	var req dto.PromptListReq
	if err := ctx.ShouldBindQuery(&req); err != nil {
		response.Fail(ctx, constants.ErrParamInvalid)
		return
	}
	// 2. 调用 Service 获取提示词列表
	prompts, err := c.svc.GetPromptList(ctx.Request.Context(), req)
	if err != nil {
		response.Fail(ctx, err)
		return
	}
	response.Success(ctx, prompts)
}

func (c *PromptController) GetDetail(ctx *gin.Context) {
	id, err := strconv.ParseInt(ctx.Param("id"), 10, 64)
	if err != nil {
		response.Fail(ctx, constants.ErrParamInvalid)
		return
	}

	role, _ := ctx.Get("role")
	isAdmin := role == "admin"

	data, err := c.svc.GetPromptDetail(ctx.Request.Context(), id, isAdmin)
	if err != nil {
		response.Fail(ctx, err)
		return
	}

	response.Success(ctx, data)
}

func (c *PromptController) Create(ctx *gin.Context) {
	var req dto.PromptReq
	if err := ctx.ShouldBindJSON(&req); err != nil {
		response.Fail(ctx, constants.ErrParamInvalid)
		return
	}

	data, err := c.svc.CreatePrompt(ctx.Request.Context(), req)
	if err != nil {
		response.Fail(ctx, err)
		return
	}

	response.Success(ctx, data)
}

func (c *PromptController) Update(ctx *gin.Context) {
	id, err := strconv.ParseInt(ctx.Param("id"), 10, 64)
	if err != nil {
		response.Fail(ctx, constants.ErrParamInvalid)
		return
	}

	var req dto.PromptReq
	if err := ctx.ShouldBindJSON(&req); err != nil {
		response.Fail(ctx, constants.ErrParamInvalid)
		return
	}

	data, err := c.svc.UpdatePrompt(ctx.Request.Context(), id, req)
	if err != nil {
		response.Fail(ctx, err)
		return
	}

	response.Success(ctx, data)
}

func (c *PromptController) Delete(ctx *gin.Context) {
	id, err := strconv.ParseInt(ctx.Param("id"), 10, 64)
	if err != nil {
		response.Fail(ctx, constants.ErrParamInvalid)
		return
	}

	if err := c.svc.DeletePrompt(ctx.Request.Context(), id); err != nil {
		response.Fail(ctx, err)
		return
	}

	response.Success(ctx, nil)
}

func (c *PromptController) Optimize(ctx *gin.Context) {
	id, err := strconv.ParseInt(ctx.Param("id"), 10, 64)
	if err != nil {
		response.Fail(ctx, constants.ErrParamInvalid)
		return
	}

	var req dto.PromptOptimizeReq
	if err := ctx.ShouldBindJSON(&req); err != nil {
		response.Fail(ctx, constants.ErrParamInvalid)
		return
	}

	data, err := c.svc.OptimizePrompt(ctx.Request.Context(), id, req)
	if err != nil {
		response.Fail(ctx, err)
		return
	}

	response.Success(ctx, data)
}
