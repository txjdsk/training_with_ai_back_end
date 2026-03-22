package middleware

import (
	"time"
	"training_with_ai/internal/pkg/logger"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

func RequestLogger() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path

		c.Next() // 等待后续业务执行完毕

		cost := time.Since(start)

		// 打印类似：[INFO] /api/v1/users 200 15ms
		logger.Info("HTTP Request",
			zap.Int("status", c.Writer.Status()),
			zap.String("method", c.Request.Method),
			zap.String("path", path),
			zap.Duration("cost", cost),
			zap.String("ip", c.ClientIP()),
		)
	}
}
