package main

import (
	"log"
	config "training_with_ai/configs"
	controller "training_with_ai/internal/controller"
	"training_with_ai/internal/middleware"
	database "training_with_ai/internal/pkg/DB"
	"training_with_ai/internal/pkg/logger"
	repository "training_with_ai/internal/repository"
	router "training_with_ai/internal/router"
	service "training_with_ai/internal/service"

	"github.com/gin-gonic/gin"
)

func main() {
	// 加载配置
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatal("Can not load config file:", err)
		panic(err)
	}

	logger.InitLogger() // 初始化日志系统
	defer logger.Sync() // 确保日志缓冲区的日志被写入磁盘

	//初始化 Gin 引擎
	r := gin.New()
	r.Use(middleware.RequestLogger())

	// 初始化数据库和 Redis
	db, err := database.InitDB(cfg.DB.DSN)
	if err != nil {
		panic(err)
	}
	rdb, err := database.InitRedis(cfg.Redis.Addr)
	if err != nil {
		panic(err)
	}

	//初始化
	//初始化 Repository
	sessionRepo := repository.NewSessionRepository(db, rdb)
	promptRepo := repository.NewPromptRepository(db, rdb)
	promptTypeRepo := repository.NewPromptTypeRepository(db, rdb)
	userRepo := repository.NewUserRepository(db, rdb)
	authRepo := repository.NewAuthRepository(db, rdb)
	// 初始化 Service
	sessionService := service.NewSessionService(sessionRepo, promptRepo)
	promptService := service.NewPromptService(promptRepo)
	promptTypeService := service.NewPromptTypeService(promptTypeRepo)
	userService := service.NewUserService(userRepo)
	authService := service.NewAuthService(authRepo)

	// 初始化 Controller
	ctrls := &router.RouterControllers{
		//User: controller.NewUserController(db, rdb),
		Session:    controller.NewSessionController(sessionService),
		Prompt:     controller.NewPromptController(promptService),
		PromptType: controller.NewPromptTypeController(promptTypeService),
		User:       controller.NewUserController(userService),
		Auth:       controller.NewAuthController(authService),
	}

	// 设置路由
	router.SetupRouter(r, ctrls, rdb)

	// 启动服务器
	log.Printf("server is running at port%s", cfg.App.Port)
	if cfg.TLS.CertFile != "" && cfg.TLS.KeyFile != "" {
		if err := r.RunTLS(":"+cfg.App.Port, cfg.TLS.CertFile, cfg.TLS.KeyFile); err != nil {
			log.Fatalf("Failed to run TLS server: %v", err)
		}
		return
	}
	if err := r.Run(":" + cfg.App.Port); err != nil {
		log.Fatalf("Failed to run server: %v", err)
	}
}
