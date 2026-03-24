package repository

import (
	"context"
	"database/sql"
	"training_with_ai/internal/model/entity"
	database "training_with_ai/internal/pkg/DB"

	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

// 1. 定义接口
type AuthRepository interface {
	GetUserByUsername(ctx context.Context, username string) (*entity.User, error)
	CreateUser(ctx context.Context, user *entity.User) error
	CheckUserExistsByUsername(ctx context.Context, username string) (bool, error)
	Logout(ctx context.Context, jti string, expireHours int) error
}

// 2. 定义私有结构体实现接口
type authRepository struct {
	db  *gorm.DB
	rdb *redis.Client
}

// 3. 构造函数：注入 DB 和 Redis 客户端，返回接口类型
func NewAuthRepository(db *gorm.DB, rdb *redis.Client) AuthRepository {
	return &authRepository{
		db:  db,
		rdb: rdb,
	}
}

func (r *authRepository) GetUserByUsername(ctx context.Context, username string) (*entity.User, error) {
	var user entity.User
	err := r.db.WithContext(ctx).Where("username = ?", username).First(&user).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *authRepository) CreateUser(ctx context.Context, user *entity.User) error {
	return r.db.WithContext(ctx).Create(user).Error
}

func (r *authRepository) CheckUserExistsByUsername(ctx context.Context, username string) (bool, error) {
	var exists bool
	err := r.db.WithContext(ctx).Model(&entity.User{}).
		Where("username = ?", username).
		Select("1").
		Limit(1).
		Row().
		Scan(&exists)
	if err != nil {
		if err == sql.ErrNoRows {
			return false, nil
		}
		return false, err
	}
	return exists, nil
}

func (r *authRepository) Logout(ctx context.Context, jti string, expireHours int) error {
	// 将 jti 加入 Redis 黑名单，过期时间和 JWT 一致
	err := database.AddToBlacklist(ctx, jti, expireHours, r.rdb)
	if err != nil {
		return err
	}
	return nil
}
