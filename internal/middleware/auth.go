package middleware

import (
	"net/http"
	"strings"
	database "training_with_ai/internal/pkg/DB"
	"training_with_ai/internal/pkg/auth"
	"training_with_ai/internal/pkg/constants"
	"training_with_ai/internal/pkg/response"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
)

// JWTAuth 校验 Token，并将 UserID 和 Role 塞入 Gin 的 Context 中
func JWTAuth(rdb *redis.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		// 1. 获取 Token (从 Header 的 Authorization 字段)
		// 2. 验证 Token (利用 pkg/utils 里的 JWT 工具)
		// 3. 提取用户信息并 set 到 context
		// c.Set("user_id", claims.UserID)
		// c.Set("role", claims.Role)
		// c.Next()
		// 1. 从请求头获取 Token（格式：Bearer {token}）
		authHeader := c.Request.Header.Get("Authorization")
		if authHeader == "" {
			response.Fail(c, constants.ErrTokenMissing)
			return
		}

		// 2. 解析 Bearer Token
		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || parts[0] != "Bearer" {
			response.Fail(c, constants.ErrTokenInvalid)
			return
		}
		tokenStr := parts[1]

		// 3. 解析 Token（验证签名 + 过期时间）
		claims, err := auth.ParseToken(tokenStr)
		if err != nil {
			response.Fail(c, constants.ErrTokenInvalid)
			return
		}

		// 4. 检查 jti 是否在黑名单（登出后的 Token）
		blacklisted, err := database.IsInBlacklist(c, claims.Jti, rdb)
		if err != nil {
			// Redis 异常时，兜底返回服务器错误
			response.Fail(c, constants.ErrServerInternal)
			return
		}
		if blacklisted {
			response.Fail(c, constants.ErrTokenBlacklisted)
			return
		}

		// 5. 验证通过：将用户信息存入 Context（供后续接口使用）
		c.Set("user_id", claims.UserID)
		c.Set("username", claims.Username)
		c.Set("jti", claims.Jti)
		c.Set("role", claims.Role)

		// 6. 继续执行后续接口逻辑
		c.Next()
	}
}

// RequireRole 基于角色的权限校验拦截 (也可以替换为更高级的 Casbin)
func RequireRole(requiredRole string) gin.HandlerFunc {
	return func(c *gin.Context) {
		role, exists := c.Get("role")
		if !exists || role.(string) != requiredRole {
			c.JSON(http.StatusForbidden, gin.H{"code": 403, "msg": "权限不足，需要管理员权限"})
			c.Abort()
			return
		}
		c.Next()
	}
}
