package middleware

import (
	"fmt"
	"time"

	"training_with_ai/internal/pkg/constants"
	"training_with_ai/internal/pkg/response"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
)

// RateLimitByTokenSession limits requests per token+sessionid.
func RateLimitByTokenSession(rdb *redis.Client, limit int, window time.Duration) gin.HandlerFunc {
	return func(c *gin.Context) {
		jtiVal, ok := c.Get("jti")
		if !ok {
			response.Fail(c, constants.ErrTokenInvalid)
			return
		}
		sessionID := c.Param("id")
		if sessionID == "" {
			response.Fail(c, constants.ErrParamInvalid)
			return
		}

		key := fmt.Sprintf("rl:session:%v:%s", jtiVal, sessionID)
		ctx := c.Request.Context()

		count, err := rdb.Incr(ctx, key).Result()
		if err != nil {
			response.Fail(c, constants.ErrServerInternal)
			return
		}
		if count == 1 {
			_ = rdb.Expire(ctx, key, window).Err()
		}
		if count > int64(limit) {
			response.Fail(c, constants.ErrRateLimited)
			return
		}

		c.Next()
	}
}
