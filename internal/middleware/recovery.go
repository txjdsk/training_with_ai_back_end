package middleware

import (
	"fmt"
	"net/http"
	"runtime/debug"
	"training_with_ai/internal/pkg/constants"
	"training_with_ai/internal/pkg/logger"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

func PanicRecovery() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if err := recover(); err != nil {
				method := c.Request.Method
				path := c.Request.URL.Path
				ip := c.ClientIP()
				panicMsg := panicToString(err)
				stack := string(debug.Stack())

				// 捕获到宕机！记录致命错误和堆栈
				logger.Error("Gin Panic Recovered",
					zap.String("method", method),
					zap.String("path", path),
					zap.String("ip", ip),
					zap.String("panic_type", fmt.Sprintf("%T", err)),
					zap.String("panic", panicMsg),
					zap.String("stack", stack),
				)

				// 如果响应已写入，避免重复写 Header
				if c.Writer.Written() {
					c.Abort()
					return
				}

				isDev := gin.Mode() != gin.ReleaseMode
				if isDev {
					c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
						"code": constants.ErrServerInternal.BizCode,
						"msg":  panicMsg,
						"data": gin.H{"stack": stack},
					})
					return
				}

				c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
					"code": constants.ErrServerInternal.BizCode,
					"msg":  constants.ErrServerInternal.Message,
					"data": nil,
				})
			}
		}()
		c.Next()
	}
}

func panicToString(p interface{}) string {
	switch v := p.(type) {
	case error:
		return v.Error()
	case string:
		return v
	default:
		return fmt.Sprintf("%v", v)
	}
}
