package router

import (
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"

	"training_with_ai/internal/controller"
	"training_with_ai/internal/middleware"
)

// RouterControllers 存放所有需要注入的 Controller
type RouterControllers struct {
	Auth       *controller.AuthController
	Prompt     *controller.PromptController
	PromptType *controller.PromptTypeController
	Session    *controller.SessionController
	User       *controller.UserController
}

// SetupRouter 初始化路由配置
func SetupRouter(r *gin.Engine, ctrls *RouterControllers, rdb *redis.Client) {
	// 1. 全局中间件
	r.Use(middleware.PanicRecovery())  // Panic 恢复
	r.Use(middleware.CORSMiddleware()) // 跨域处理

	v1 := r.Group("/api/v1")

	// ==========================================
	// A. 认证中心 (Auth) - 公开接口
	// ==========================================
	authGroup := v1.Group("/auth")
	{
		authGroup.POST("/login", ctrls.Auth.Login)
		authGroup.POST("/register", ctrls.Auth.Register)
		// 登出需要用户先登录并携带 Token
		authGroup.POST("/logout", middleware.JWTAuth(rdb), ctrls.Auth.Logout)
	}

	// ==========================================
	// 需要普通用户登录认证的路由组
	// ==========================================
	authRequired := v1.Group("")
	authRequired.Use(middleware.JWTAuth(rdb)) // 挂载 JWT 认证中间件
	{
		// --------------------------------------
		// B. 提示词 (Prompts)
		// --------------------------------------
		prompts := authRequired.Group("/prompts")
		{
			// 普通用户可读
			prompts.GET("", ctrls.Prompt.GetList)
			prompts.GET("/:id", ctrls.Prompt.GetDetail)

			// 管理员专属操作
			adminPrompts := prompts.Group("")
			adminPrompts.Use(middleware.RequireRole("admin")) // Casbin鉴权或简单角色校验
			{
				adminPrompts.POST("", ctrls.Prompt.Create)
				adminPrompts.PUT("/:id", ctrls.Prompt.Update)
				adminPrompts.DELETE("/:id", ctrls.Prompt.Delete)
				adminPrompts.POST("/:id/optimize", ctrls.Prompt.Optimize) // AI辅助优化
			}
		}

		// --------------------------------------
		// C. 提示词类别 (Prompt Types)
		// --------------------------------------
		promptTypes := authRequired.Group("/prompt-types")
		{
			promptTypes.GET("", ctrls.PromptType.GetList)

			adminTypes := promptTypes.Group("")
			adminTypes.Use(middleware.RequireRole("admin"))
			{
				adminTypes.POST("", ctrls.PromptType.Create)
				adminTypes.PUT("/:id", ctrls.PromptType.Update)
				adminTypes.DELETE("/:id", ctrls.PromptType.Delete)
			}
		}

		// --------------------------------------
		// D. 训练会话 (Sessions) - 核心
		// --------------------------------------
		sessions := authRequired.Group("/sessions")
		{
			sessions.POST("", ctrls.Session.Create)
			sessions.GET("/:id/stream", ctrls.Session.Stream)                                                           // SSE连接
			sessions.POST("/:id/chat", middleware.RateLimitByTokenSession(rdb, 10, 10*time.Second), ctrls.Session.Chat) // 发送消息
			sessions.POST("/:id/terminate", ctrls.Session.Terminate)                                                    // 主动停止
			sessions.GET("/:id", ctrls.Session.GetDetail)                                                               // 会话详情
			sessions.GET("", ctrls.Session.GetList)                                                                     // 历史列表

			// 软删除记录 (Controller层需二次校验：只能删自己的，或Admin可删所有人)
			sessions.DELETE("/:id", ctrls.Session.Delete)
		}

		// --------------------------------------
		// E. 用户管理 (Users)
		// --------------------------------------
		users := authRequired.Group("/users")
		{
			// 普通用户自己的 Profile 操作
			users.GET("/profile", ctrls.User.GetProfile)
			users.PUT("/profile", ctrls.User.UpdateProfile)

			// 管理员专属操作
			adminUsers := users.Group("")
			adminUsers.Use(middleware.RequireRole("admin"))
			{
				adminUsers.GET("", ctrls.User.GetList)
				adminUsers.POST("", ctrls.User.Create)
				adminUsers.PUT("/:id", ctrls.User.Update)
				adminUsers.DELETE("/:id", ctrls.User.Delete)
			}
		}
	}
}
