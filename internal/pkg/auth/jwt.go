package auth

import (
	"errors"
	"time"
	config "training_with_ai/configs"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"

	"training_with_ai/internal/pkg/constants"
)

// JWTConfig JWT 配置
type JWTConfig struct {
	SecretKey       string // JWT 签名密钥（生产环境放配置/环境变量）
	ExpireHours     int    // Token 过期时间（72小时）
	BlacklistPrefix string // Redis 黑名单 key 前缀
}

func loadJWTConfig() JWTConfig {
	const defaultExpireHours = 72

	cfg, err := config.LoadConfig()
	if err != nil || cfg == nil {
		return JWTConfig{
			SecretKey:       "change-me-in-config",
			ExpireHours:     defaultExpireHours,
			BlacklistPrefix: "jwt:blacklist:",
		}
	}

	secretKey := cfg.JWT.SecretKey
	if secretKey == "" {
		secretKey = "change-me-in-config"
	}

	expireHours := cfg.JWT.ExpireHours
	if expireHours <= 0 {
		expireHours = defaultExpireHours
	}

	return JWTConfig{
		SecretKey:       secretKey, // 生产环境替换为随机字符串（至少32位）
		ExpireHours:     expireHours,
		BlacklistPrefix: "jwt:blacklist:",
	}
}

var JWTConf = loadJWTConfig()

// CustomClaims 自定义 JWT 载荷（包含 jti + 核心用户信息）
type CustomClaims struct {
	jwt.RegisteredClaims        // 内置标准 Claims（包含 exp 过期时间等）
	Jti                  string `json:"jti"`      // JWT ID（UUID），用于黑名单
	UserID               int64  `json:"user_id"`  // 用户ID（业务字段）
	Username             string `json:"username"` // 用户名（可选）
	Role                 string `json:"role"`     // 角色（如 admin/user）
}

// GenerateToken 生成 JWT Token（登录时调用）
func GenerateToken(userID int64, username string, role string) (string, error) {
	// 1. 生成唯一 jti（UUID v4）
	jti := uuid.New().String()

	// 2. 定义过期时间
	expireTime := time.Now().Add(time.Duration(JWTConf.ExpireHours) * time.Hour)

	// 3. 构造自定义 Claims
	claims := CustomClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expireTime), // 过期时间
			IssuedAt:  jwt.NewNumericDate(time.Now()), // 签发时间
			Issuer:    "training_with_ai",             // 签发者（可选）
		},
		Jti:      jti,
		UserID:   userID,
		Username: username,
		Role:     role,
	}

	// 4. 签名生成 Token
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signedToken, err := token.SignedString([]byte(JWTConf.SecretKey))
	if err != nil {
		return "", errors.New("生成Token失败：" + err.Error())
	}

	return signedToken, nil
}

// ParseToken 解析 Token（鉴权时调用）
func ParseToken(tokenStr string) (*CustomClaims, error) {
	// 1. 解析 Token（验证签名 + 过期时间）
	token, err := jwt.ParseWithClaims(
		tokenStr,
		&CustomClaims{},
		func(token *jwt.Token) (interface{}, error) {
			// 验证签名算法
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, constants.ErrTokenInvalid // 自定义错误：Token无效
			}
			return []byte(JWTConf.SecretKey), nil
		},
	)
	if err != nil {
		return nil, constants.ErrTokenInvalid
	}

	// 2. 校验 Token 有效性
	if claims, ok := token.Claims.(*CustomClaims); ok && token.Valid {
		return claims, nil
	}

	return nil, constants.ErrTokenInvalid
}
