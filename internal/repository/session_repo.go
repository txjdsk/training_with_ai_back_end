package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"

	"training_with_ai/internal/model/entity" // 你的数据库实体存放路径
)

// 1. 定义接口 (方便后续使用 mockery 生成 Mock 文件进行测试)
type SessionRepository interface {
	// Redis 相关方法
	SetSessionCache(ctx context.Context, sessionID string, data []byte, ttl time.Duration) error
	GetSessionCache(ctx context.Context, sessionID string) ([]byte, error)
	UpdateSessionTTL(ctx context.Context, sessionID string, ttl time.Duration) error
	DeleteSessionCache(ctx context.Context, sessionID string) error

	// PostgreSQL 相关方法
	CreateRecord(ctx context.Context, record *entity.TrainingRecord) error
	GetRecordByID(ctx context.Context, id int64) (*entity.TrainingRecord, error)
	ListRecords(ctx context.Context, userID *int64, filterUsername string, minScore *float64, maxScore *float64, promptID *uint64, page int, size int) ([]entity.TrainingRecord, int64, error)
	DeleteRecord(ctx context.Context, id int64) error
}

// 2. 定义私有结构体实现接口
type sessionRepository struct {
	db  *gorm.DB
	rdb *redis.Client
}

// 3. 构造函数：注入 DB 和 Redis 客户端，返回接口类型
func NewSessionRepository(db *gorm.DB, rdb *redis.Client) SessionRepository {
	return &sessionRepository{
		db:  db,
		rdb: rdb,
	}
}

// ---------------- 以下为接口方法的具体实现示例 ----------------
// TODO: 这里的实现只是示例，实际项目中可能需要更复杂的逻辑和错误处理
func (r *sessionRepository) SetSessionCache(ctx context.Context, sessionID string, data []byte, ttl time.Duration) error {
	return r.rdb.Set(ctx, "session:"+sessionID, data, ttl).Err()
}

func (r *sessionRepository) GetSessionCache(ctx context.Context, sessionID string) ([]byte, error) {
	return r.rdb.Get(ctx, "session:"+sessionID).Bytes()
}

func (r *sessionRepository) UpdateSessionTTL(ctx context.Context, sessionID string, ttl time.Duration) error {
	return r.rdb.Expire(ctx, "session:"+sessionID, ttl).Err()
}

func (r *sessionRepository) DeleteSessionCache(ctx context.Context, sessionID string) error {
	return r.rdb.Del(ctx, "session:"+sessionID).Err()
}

func (r *sessionRepository) CreateRecord(ctx context.Context, record *entity.TrainingRecord) error {
	return r.db.WithContext(ctx).Create(record).Error
}

func (r *sessionRepository) GetRecordByID(ctx context.Context, id int64) (*entity.TrainingRecord, error) {
	var record entity.TrainingRecord
	if err := r.db.WithContext(ctx).Preload("User").First(&record, id).Error; err != nil {
		return nil, err
	}
	return &record, nil
}

func (r *sessionRepository) ListRecords(ctx context.Context, userID *int64, filterUsername string, minScore *float64, maxScore *float64, promptID *uint64, page int, size int) ([]entity.TrainingRecord, int64, error) {
	query := r.db.WithContext(ctx).Model(&entity.TrainingRecord{}).Preload("User")
	if userID != nil {
		query = query.Where("user_id = ?", *userID)
	}
	if filterUsername != "" {
		query = query.Joins("User").Where("users.username ILIKE ?", "%"+filterUsername+"%")
	}
	if minScore != nil {
		query = query.Where("score >= ?", *minScore)
	}
	if maxScore != nil {
		query = query.Where("score <= ?", *maxScore)
	}
	if promptID != nil {
		query = query.Where("used_prompt_ids @> ?", "["+fmt.Sprint(*promptID)+"]")
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

	var records []entity.TrainingRecord
	if err := query.Order("finished_at desc").Limit(size).Offset(offset).Find(&records).Error; err != nil {
		return nil, 0, err
	}

	return records, total, nil
}

func (r *sessionRepository) DeleteRecord(ctx context.Context, id int64) error {
	return r.db.WithContext(ctx).Delete(&entity.TrainingRecord{}, id).Error
}
