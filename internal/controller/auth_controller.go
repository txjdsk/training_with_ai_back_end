package controller

import (
	"net/http"
	"training_with_ai/internal/model/dto"
	"training_with_ai/internal/pkg/constants"
	"training_with_ai/internal/pkg/response"
	"training_with_ai/internal/service"

	"github.com/gin-gonic/gin"
)

// 定义 Controller 结构体
type AuthController struct {
	// 注入 Service 接口
	svc service.AuthService
}

// 构造函数
func NewAuthController(svc service.AuthService) *AuthController {
	return &AuthController{
		svc: svc,
	}
}

//handler方法

func (c *AuthController) Login(ctx *gin.Context) {
	// 1. 解析请求参数
	var req dto.AuthLoginReq
	if err := ctx.ShouldBindJSON(&req); err != nil {
		response.Fail(ctx, constants.ErrParamInvalid)
		return
	}

	// 2. 调用 Service 进行登录逻辑处理
	cookieMaxAge, token, err := c.svc.Login(ctx.Request.Context(), req)
	if err != nil {
		response.Fail(ctx, err)
		return
	}
	if cookieMaxAge > 0 {
		cookieMaxAge = cookieMaxAge * 3600
	} else if cookieMaxAge < 0 {
		cookieMaxAge = 0
	}

	ctx.SetCookie(
		"token",
		token,
		cookieMaxAge, // -1=会话Cookie，72*3600=持久72小时
		"/",          // 全站有效
		"",           // 域名
		true,         // HTTPS下Secure=true（你已做HTTPS）
		true,         // HttpOnly防XSS（你已做）
	)
	ctx.SetSameSite(http.SameSiteLaxMode) // 防CSRF
	response.Success(ctx, nil)
}

func (c *AuthController) Register(ctx *gin.Context) {
	// 1. 解析请求参数
	var req dto.AuthRegisterReq
	if err := ctx.ShouldBindJSON(&req); err != nil {
		response.Fail(ctx, constants.ErrParamInvalid)
		return
	}

	// 2. 调用 Service 进行注册逻辑处理
	err := c.svc.Register(ctx.Request.Context(), req)
	if err != nil {
		response.Fail(ctx, err)
		return
	}

	response.Success(ctx, nil)
}

func (c *AuthController) Logout(ctx *gin.Context) {
	jti, exists := ctx.Get("jti")
	if !exists {
		response.Fail(ctx, constants.ErrTokenInvalid)
		return
	}

	// 2. 调用 Service 进行登出逻辑处理
	err := c.svc.Logout(ctx.Request.Context(), jti.(string))
	if err != nil {
		response.Fail(ctx, err)
		return
	}

	response.Success(ctx, nil)
}
