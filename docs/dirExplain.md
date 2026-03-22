training_with_ai/
├── cmd/
│   └── server/
│       └── main.go           # 【入口】项目启动文件，负责初始化配置、数据库、路由并启动服务
├── configs/                  # 【配置】加载配置文件的代码
├── internal/                 # 【私有代码】项目的核心业务代码，外部项目不可导入
│   ├── config/               # 配置加载逻辑（读取 configs/ 下的文件映射到结构体）
│   ├── controller/           # 【控制层】处理 HTTPS请求，参数校验，调用 Service，返回 JSON
│   ├── service/              # 【业务逻辑层】核心业务逻辑，事务处理，调用 Repository
│   ├── repository/           # 【数据访问层】封装 GORM 操作，负责数据库 CRUD
│   ├── model/                # 【模型层】
│   │   ├── entity/           # 数据库实体 (GORM Struct, 对应数据库表)
│   │   └── dto/              # 数据传输对象 (前端请求参数 Request / 响应数据 Response)
│   ├── router/               # 路由定义，将 URL 映射到 Controller
│   ├── middleware/           # Gin 中间件 (JWT认证, CORS, 日志, 鉴权, Panic恢复)
│   ├── pkg/                  # 内部通用工具包 (不想对外暴露的工具)
│   │   ├── auth/             # 生成令牌，解析令牌，核对密码，密码加盐等业务
│   │   ├── calc/             # 评分相关打分算法函数
│   │   ├── DB/               # 数据库初始化与redis相关调用业务函数
│   │   ├── response/         # 统一返回格式
│   │   ├── logger/           # 日志库封装 (Zap/Logrus)
│   │   └──...（其他工具）
│   └── mocks/                # 【测试】mockery 生成的 Mock 文件，用于 Service/Repo 单元测试
├── pkg/                      # 【公共库】可以被外部项目使用的通用工具 (如加密工具、时间处理)
│   ├── utils/
│   └── constants/            # 全局常量 (错误码定义等)
├── test/                     # 【集成测试】针对 API 接口的端对端测试代码
├── docs/                     # 文档
├── scripts/                  # 构建、安装、数据库迁移等脚本
├── .github/
│    └── workflows/            # 【CI/CD】GitHub Actions 配置文件
└── .env
