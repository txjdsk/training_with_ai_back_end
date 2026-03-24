package middleware

import (
	"strconv"
	"strings"

	"training_with_ai/internal/pkg/logger"

	"github.com/gin-gonic/gin"
)

// CORSConfig CORS 配置项（可根据环境调整）
type CORSConfig struct {
	AllowOrigins     []string // 允许的跨域源（生产环境建议指定具体域名，如 ["https://your-front.com"]）
	AllowMethods     []string // 允许的请求方法
	AllowHeaders     []string // 允许的请求头
	AllowCredentials bool     // 是否允许携带凭证（cookie、token 等）
	MaxAge           int      // 预检请求（OPTIONS）的缓存时间（秒），减少 OPTIONS 请求次数
	ExposeHeaders    []string // 允许前端访问的响应头
}

// 默认 CORS 配置（开发环境可用，生产环境建议自定义）
func defaultCORSConfig() CORSConfig {
	return CORSConfig{
		AllowOrigins:     []string{"*"}, // 开发环境允许所有源，生产环境替换为具体域名
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "PATCH", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Authorization", "X-Requested-With"},
		AllowCredentials: true,                                  // 允许携带凭证（比如 JWT token、cookie）
		MaxAge:           86400,                                 // 预检请求缓存 24 小时
		ExposeHeaders:    []string{"Content-Length", "X-Token"}, // 前端可获取的自定义响应头
	}
}

// CORSMiddleware Gin CORS 中间件（支持自定义配置，不传则用默认）
func CORSMiddleware(config ...CORSConfig) gin.HandlerFunc {
	// 处理配置：不传则用默认
	cfg := defaultCORSConfig()
	if len(config) > 0 {
		cfg = config[0]
	}

	return func(c *gin.Context) {
		// 1. 获取请求的 Origin（前端域名）
		origin := c.Request.Header.Get("Origin")
		// 2. 记录跨域请求日志（结合你封装的 logger）
		logger.Infow(
			"cors request",
			"origin", origin,
			"method", c.Request.Method,
			"path", c.Request.URL.Path,
		)

		// 3. 处理允许的源：配置 "*" 时优先回显 Origin（避免带凭证时浏览器拦截）
		allowOrigin := ""
		if len(cfg.AllowOrigins) > 0 {
			allowAll := false
			for _, o := range cfg.AllowOrigins {
				if o == "*" {
					allowAll = true
					break
				}
			}
			if allowAll {
				if cfg.AllowCredentials && origin != "" {
					allowOrigin = origin
				} else {
					allowOrigin = "*"
				}
			} else {
				// 生产环境：匹配请求的 Origin 是否在允许列表中
				for _, o := range cfg.AllowOrigins {
					if o == origin {
						allowOrigin = origin
						break
					}
				}
			}
		}

		// 4. 设置核心跨域响应头
		if allowOrigin != "" {
			c.Header("Access-Control-Allow-Origin", allowOrigin)
		}
		// 允许的请求方法（拼接为字符串）
		c.Header("Access-Control-Allow-Methods", strings.Join(cfg.AllowMethods, ","))
		// 允许的请求头
		c.Header("Access-Control-Allow-Headers", strings.Join(cfg.AllowHeaders, ","))
		// 允许携带凭证（注意：Allow-Origin 为 "*" 时，这个值不能为 true，浏览器会报错）
		if cfg.AllowCredentials {
			c.Header("Access-Control-Allow-Credentials", "true")
		}
		// 预检请求缓存时间
		if cfg.MaxAge > 0 {
			c.Header("Access-Control-Max-Age", strconv.Itoa(cfg.MaxAge))
		}
		// 暴露的响应头（前端可通过 JS 获取）
		if len(cfg.ExposeHeaders) > 0 {
			c.Header("Access-Control-Expose-Headers", strings.Join(cfg.ExposeHeaders, ","))
		}

		// 5. 处理预检请求（OPTIONS）：直接返回 204，不执行后续逻辑
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		// 6. 非 OPTIONS 请求，继续执行后续中间件/处理函数
		c.Next()
	}
}
