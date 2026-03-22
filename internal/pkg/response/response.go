package response

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"training_with_ai/internal/pkg/constants"
	"training_with_ai/internal/pkg/logger" // 你自己封装的日志库
)

type APIResponse struct {
	Code int         `json:"code"` // 业务码
	Msg  string      `json:"msg"`  // 提示信息
	Data interface{} `json:"data"` // 业务数据
}

// Success 成功返回
func Success(c *gin.Context, data interface{}) {
	c.JSON(http.StatusOK, APIResponse{
		Code: constants.Success.BizCode,
		Msg:  constants.Success.Message,
		Data: data,
	})
}

// Fail 失败返回
func Fail(c *gin.Context, err error) {
	var appErr *constants.AppError

	// 1. 判断是否是我们自定义的业务错误
	if errors.As(err, &appErr) {
		// 是预知的业务错误，直接返回给前端
		c.JSON(appErr.HTTPCode, APIResponse{
			Code: appErr.BizCode,
			Msg:  appErr.Message,
			Data: nil,
		})
		return
	}

	// 2. 如果是未知的系统底层错误 (如数据库断开、空指针)
	// 对内：记录真实的底层报错到日志文件
	logger.Error("系统未知异常", zap.Error(err), zap.String("path", c.Request.URL.Path))

	// 对外：统一伪装成 500，绝不泄露包含 SQL 语句的报错信息
	c.JSON(constants.ErrServerInternal.HTTPCode, APIResponse{
		Code: constants.ErrServerInternal.BizCode,
		Msg:  constants.ErrServerInternal.Message,
		Data: nil,
	})
}
