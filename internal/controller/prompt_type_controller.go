package controller

import (
	"strconv"
	"training_with_ai/internal/model/dto"
	"training_with_ai/internal/pkg/constants"
	"training_with_ai/internal/pkg/response"
	"training_with_ai/internal/service"

	"github.com/gin-gonic/gin"
)

// 1. 定义 Controller 结构体
type PromptTypeController struct {
	// 注入 Service 接口
	svc service.PromptTypeService
}

func NewPromptTypeController(svc service.PromptTypeService) *PromptTypeController {
	return &PromptTypeController{
		svc: svc,
	}
}

func (c *PromptTypeController) GetList(ctx *gin.Context) {
	resp, err := c.svc.GetList(ctx.Request.Context())
	if err != nil {
		response.Fail(ctx, err)
		return
	}
	response.Success(ctx, resp)
}

func (c *PromptTypeController) Create(ctx *gin.Context) {
	var req dto.PromptCategoryReq
	if err := ctx.ShouldBindJSON(&req); err != nil {
		response.Fail(ctx, constants.ErrParamInvalid)
		return
	}

	resp, err := c.svc.Create(ctx.Request.Context(), req)
	if err != nil {
		response.Fail(ctx, err)
		return
	}
	response.Success(ctx, resp)
}

func (c *PromptTypeController) Update(ctx *gin.Context) {
	id, err := strconv.ParseUint(ctx.Param("id"), 10, 64)
	if err != nil {
		response.Fail(ctx, constants.ErrParamInvalid)
		return
	}

	var req dto.PromptCategoryReq
	if err := ctx.ShouldBindJSON(&req); err != nil {
		response.Fail(ctx, constants.ErrParamInvalid)
		return
	}

	resp, err := c.svc.Update(ctx.Request.Context(), uint(id), req)
	if err != nil {
		response.Fail(ctx, err)
		return
	}
	response.Success(ctx, resp)
}

func (c *PromptTypeController) Delete(ctx *gin.Context) {
	id, err := strconv.ParseUint(ctx.Param("id"), 10, 64)
	if err != nil {
		response.Fail(ctx, constants.ErrParamInvalid)
		return
	}

	if err := c.svc.Delete(ctx.Request.Context(), uint(id)); err != nil {
		response.Fail(ctx, err)
		return
	}
	response.Success(ctx, nil)
}
