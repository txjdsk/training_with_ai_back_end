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
type UserController struct {
	// 注入 Service 接口
	svc service.UserService
}

func NewUserController(svc service.UserService) *UserController {
	return &UserController{
		svc: svc,
	}
}

func (c *UserController) GetProfile(ctx *gin.Context) {
	userID, ok := ctx.Get("user_id")
	if !ok {
		response.Fail(ctx, constants.ErrTokenInvalid)
		return
	}

	resp, err := c.svc.GetProfile(ctx.Request.Context(), userID.(int64))
	if err != nil {
		response.Fail(ctx, err)
		return
	}

	response.Success(ctx, resp)
}

func (c *UserController) UpdateProfile(ctx *gin.Context) {
	userID, ok := ctx.Get("user_id")
	if !ok {
		response.Fail(ctx, constants.ErrTokenInvalid)
		return
	}

	var req dto.UpdateProfileReq
	if err := ctx.ShouldBindJSON(&req); err != nil {
		response.Fail(ctx, constants.ErrParamInvalid)
		return
	}

	resp, err := c.svc.UpdateProfile(ctx.Request.Context(), userID.(int64), req)
	if err != nil {
		response.Fail(ctx, err)
		return
	}

	response.Success(ctx, resp)
}

func (c *UserController) GetList(ctx *gin.Context) {
	var req dto.AdminUserFilterReq
	if err := ctx.ShouldBindQuery(&req); err != nil {
		response.Fail(ctx, constants.ErrParamInvalid)
		return
	}

	resp, err := c.svc.AdminList(ctx.Request.Context(), req)
	if err != nil {
		response.Fail(ctx, err)
		return
	}

	response.Success(ctx, resp)
}

func (c *UserController) Create(ctx *gin.Context) {
	var req dto.AdminCreateUserReq
	if err := ctx.ShouldBindJSON(&req); err != nil {
		response.Fail(ctx, constants.ErrParamInvalid)
		return
	}

	resp, err := c.svc.AdminCreate(ctx.Request.Context(), req)
	if err != nil {
		response.Fail(ctx, err)
		return
	}

	response.Success(ctx, resp)
}

func (c *UserController) Update(ctx *gin.Context) {
	id, err := strconv.ParseInt(ctx.Param("id"), 10, 64)
	if err != nil {
		response.Fail(ctx, constants.ErrParamInvalid)
		return
	}

	var req dto.AdminUpdateUserReq
	if err := ctx.ShouldBindJSON(&req); err != nil {
		response.Fail(ctx, constants.ErrParamInvalid)
		return
	}

	resp, err := c.svc.AdminUpdate(ctx.Request.Context(), id, req)
	if err != nil {
		response.Fail(ctx, err)
		return
	}

	response.Success(ctx, resp)
}

func (c *UserController) Delete(ctx *gin.Context) {
	operatorID, ok := ctx.Get("user_id")
	if !ok {
		response.Fail(ctx, constants.ErrTokenInvalid)
		return
	}

	id, err := strconv.ParseInt(ctx.Param("id"), 10, 64)
	if err != nil {
		response.Fail(ctx, constants.ErrParamInvalid)
		return
	}

	if err := c.svc.AdminDelete(ctx.Request.Context(), id, operatorID.(int64)); err != nil {
		response.Fail(ctx, err)
		return
	}

	response.Success(ctx, nil)
}
