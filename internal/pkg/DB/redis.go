package database

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
)

// AddToBlacklist 将 jti 加入黑名单（登出时调用）
func AddToBlacklist(ctx context.Context, jti string, expireHours int, client *redis.Client) error {
	// 黑名单 Key：jwt:blacklist:{jti}
	key := "jwt:blacklist:" + jti
	// 设置值为 1，过期时间和 Token 一致（72小时）
	return client.SetEx(ctx, key, 1, time.Duration(expireHours)*time.Hour).Err()
}

// IsInBlacklist 检查 jti 是否在黑名单中（鉴权时调用）
func IsInBlacklist(ctx context.Context, jti string, client *redis.Client) (bool, error) {
	key := "jwt:blacklist:" + jti
	exists, err := client.Exists(ctx, key).Result()
	if err != nil {
		return false, err
	}
	// exists == 1 表示存在（在黑名单中）
	return exists == 1, nil
}
