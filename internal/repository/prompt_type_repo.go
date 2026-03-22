package repository

import (
	"context"
	"training_with_ai/internal/model/entity"

	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

// 1. 定义接口
type PromptTypeRepository interface {
	List(ctx context.Context) ([]entity.PromptCategory, error)
	GetByID(ctx context.Context, id uint) (*entity.PromptCategory, error)
	GetByName(ctx context.Context, name string) (*entity.PromptCategory, error)
	Create(ctx context.Context, category *entity.PromptCategory) error
	Update(ctx context.Context, category *entity.PromptCategory) error
	Delete(ctx context.Context, id uint) error
	CountPromptsByCategory(ctx context.Context, categoryID uint) (int64, error)
}

// 2. 定义私有结构体实现接口
type promptTypeRepository struct {
	db  *gorm.DB
	rdb *redis.Client
}

// 3. 构造函数：注入 DB 和 Redis 客户端，返回接口类型
func NewPromptTypeRepository(db *gorm.DB, rdb *redis.Client) PromptTypeRepository {
	return &promptTypeRepository{
		db:  db,
		rdb: rdb,
	}
}

func (r *promptTypeRepository) List(ctx context.Context) ([]entity.PromptCategory, error) {
	var categories []entity.PromptCategory
	if err := r.db.WithContext(ctx).Order("id asc").Find(&categories).Error; err != nil {
		return nil, err
	}
	return categories, nil
}

func (r *promptTypeRepository) GetByID(ctx context.Context, id uint) (*entity.PromptCategory, error) {
	var category entity.PromptCategory
	if err := r.db.WithContext(ctx).First(&category, id).Error; err != nil {
		return nil, err
	}
	return &category, nil
}

func (r *promptTypeRepository) GetByName(ctx context.Context, name string) (*entity.PromptCategory, error) {
	var category entity.PromptCategory
	if err := r.db.WithContext(ctx).Where("name = ?", name).First(&category).Error; err != nil {
		return nil, err
	}
	return &category, nil
}

func (r *promptTypeRepository) Create(ctx context.Context, category *entity.PromptCategory) error {
	return r.db.WithContext(ctx).Create(category).Error
}

func (r *promptTypeRepository) Update(ctx context.Context, category *entity.PromptCategory) error {
	return r.db.WithContext(ctx).Save(category).Error
}

func (r *promptTypeRepository) Delete(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).Delete(&entity.PromptCategory{}, id).Error
}

func (r *promptTypeRepository) CountPromptsByCategory(ctx context.Context, categoryID uint) (int64, error) {
	var count int64
	if err := r.db.WithContext(ctx).Model(&entity.Prompt{}).
		Where("category_id = ?", categoryID).
		Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}
