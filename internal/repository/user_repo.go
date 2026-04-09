package repository

import (
	"context"
	"database/sql"
	"time"
	"training_with_ai/internal/model/entity"

	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

// 1. 定义接口
type UserRepository interface {
	GetByID(ctx context.Context, id int64) (*entity.User, error)
	GetByUsername(ctx context.Context, username string) (*entity.User, error)
	CheckUsernameExists(ctx context.Context, username string) (bool, error)
	CheckUsernameExistsExcludeID(ctx context.Context, username string, excludeID int64) (bool, error)
	Create(ctx context.Context, user *entity.User) error
	Update(ctx context.Context, user *entity.User) error
	Delete(ctx context.Context, id int64) error
	List(ctx context.Context, username string, role string, startTime *time.Time, endTime *time.Time, page int, size int) ([]entity.User, int64, error)
}

// 2. 定义私有结构体实现接口
type userRepository struct {
	db  *gorm.DB
	rdb *redis.Client
}

// 3. 构造函数：注入 DB 和 Redis 客户端，返回接口类型
func NewUserRepository(db *gorm.DB, rdb *redis.Client) UserRepository {
	return &userRepository{
		db:  db,
		rdb: rdb,
	}
}

func (r *userRepository) GetByID(ctx context.Context, id int64) (*entity.User, error) {
	var user entity.User
	if err := r.db.WithContext(ctx).First(&user, id).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *userRepository) GetByUsername(ctx context.Context, username string) (*entity.User, error) {
	var user entity.User
	if err := r.db.WithContext(ctx).Where("username = ?", username).First(&user).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *userRepository) CheckUsernameExists(ctx context.Context, username string) (bool, error) {
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

func (r *userRepository) CheckUsernameExistsExcludeID(ctx context.Context, username string, excludeID int64) (bool, error) {
	var exists bool
	err := r.db.WithContext(ctx).Model(&entity.User{}).
		Where("username = ? AND id <> ?", username, excludeID).
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

func (r *userRepository) Create(ctx context.Context, user *entity.User) error {
	return r.db.WithContext(ctx).Create(user).Error
}

func (r *userRepository) Update(ctx context.Context, user *entity.User) error {
	return r.db.WithContext(ctx).Save(user).Error
}

func (r *userRepository) Delete(ctx context.Context, id int64) error {
	return r.db.WithContext(ctx).Delete(&entity.User{}, id).Error
}

func (r *userRepository) List(ctx context.Context, username string, role string, startTime *time.Time, endTime *time.Time, page int, size int) ([]entity.User, int64, error) {
	query := r.db.WithContext(ctx).Model(&entity.User{})
	if username != "" {
		query = query.Where("username ILIKE ?", "%"+username+"%")
	}
	if role != "" {
		query = query.Where("role = ?", role)
	}
	if startTime != nil {
		query = query.Where("created_at >= ?", *startTime)
	}
	if endTime != nil {
		query = query.Where("created_at <= ?", *endTime)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	if page <= 0 {
		page = 1
	}
	if size <= 0 {
		size = 10
	}
	offset := (page - 1) * size

	var users []entity.User
	if err := query.Order("created_at desc").Limit(size).Offset(offset).Find(&users).Error; err != nil {
		return nil, 0, err
	}

	return users, total, nil
}
