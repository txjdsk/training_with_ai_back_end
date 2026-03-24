package repository

import (
	"context"
	"training_with_ai/internal/model/entity"

	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

// 1. 定义接口
type PromptRepository interface {
	GetPromptList(ctx context.Context, promptType *uint, search string) ([]entity.Prompt, error)
	GetPromptByID(ctx context.Context, id int64) (*entity.Prompt, error)
	GetPromptsByIDs(ctx context.Context, ids []uint64) ([]entity.Prompt, error)
	GetPromptsByCategory(ctx context.Context, categoryID uint) ([]entity.Prompt, error)
	CreatePrompt(ctx context.Context, prompt *entity.Prompt) error
	UpdatePrompt(ctx context.Context, prompt *entity.Prompt) error
	DeletePrompt(ctx context.Context, id int64) error
	CheckCategoryExists(ctx context.Context, categoryID uint) (bool, error)
}

// 2. 定义私有结构体实现接口
type promptRepository struct {
	db  *gorm.DB
	rdb *redis.Client
}

// 3. 构造函数：注入 DB 和 Redis 客户端，返回接口类型
func NewPromptRepository(db *gorm.DB, rdb *redis.Client) PromptRepository {
	return &promptRepository{
		db:  db,
		rdb: rdb,
	}
}

func (r *promptRepository) GetPromptList(ctx context.Context, promptType *uint, search string) ([]entity.Prompt, error) {
	// 1. 构建查询条件
	query := r.db.WithContext(ctx).Model(&entity.Prompt{})
	if promptType != nil {
		query = query.Where("category_id = ?", *promptType)
	}
	if search != "" {
		likeSearch := "%" + search + "%"
		query = query.Where("note LIKE ?", likeSearch)
	}
	// 2. 执行查询
	var prompts []entity.Prompt
	if err := query.Find(&prompts).Error; err != nil {
		return nil, err
	}
	return prompts, nil
}

func (r *promptRepository) GetPromptByID(ctx context.Context, id int64) (*entity.Prompt, error) {
	var prompt entity.Prompt
	if err := r.db.WithContext(ctx).First(&prompt, id).Error; err != nil {
		return nil, err
	}
	return &prompt, nil
}

func (r *promptRepository) GetPromptsByIDs(ctx context.Context, ids []uint64) ([]entity.Prompt, error) {
	if len(ids) == 0 {
		return []entity.Prompt{}, nil
	}
	var prompts []entity.Prompt
	if err := r.db.WithContext(ctx).Where("id IN ?", ids).Find(&prompts).Error; err != nil {
		return nil, err
	}
	return prompts, nil
}

func (r *promptRepository) GetPromptsByCategory(ctx context.Context, categoryID uint) ([]entity.Prompt, error) {
	var prompts []entity.Prompt
	if err := r.db.WithContext(ctx).Where("category_id = ?", categoryID).Find(&prompts).Error; err != nil {
		return nil, err
	}
	return prompts, nil
}

func (r *promptRepository) CreatePrompt(ctx context.Context, prompt *entity.Prompt) error {
	return r.db.WithContext(ctx).Create(prompt).Error
}

func (r *promptRepository) UpdatePrompt(ctx context.Context, prompt *entity.Prompt) error {
	return r.db.WithContext(ctx).Save(prompt).Error
}

func (r *promptRepository) DeletePrompt(ctx context.Context, id int64) error {
	return r.db.WithContext(ctx).Delete(&entity.Prompt{}, id).Error
}

func (r *promptRepository) CheckCategoryExists(ctx context.Context, categoryID uint) (bool, error) {
	var exists bool
	err := r.db.WithContext(ctx).Model(&entity.PromptCategory{}).
		Where("id = ?", categoryID).
		Select("1").
		Limit(1).
		Row().
		Scan(&exists)
	if err != nil {
		return false, err
	}
	return exists, nil
}
