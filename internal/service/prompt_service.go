package service

import (
	"context"
	"errors"
	"training_with_ai/internal/model/dto"
	"training_with_ai/internal/model/entity"
	"training_with_ai/internal/pkg/constants"
	"training_with_ai/internal/pkg/llm"
	"training_with_ai/internal/repository"

	config "training_with_ai/configs"

	"gorm.io/gorm"
)

type PromptService interface {
	GetPromptList(ctx context.Context, req dto.PromptListReq) ([]dto.PromptListItem, error)
	GetPromptDetail(ctx context.Context, id int64, isAdmin bool) (interface{}, error)
	CreatePrompt(ctx context.Context, req dto.PromptReq) (*dto.AdminPromptResp, error)
	UpdatePrompt(ctx context.Context, id int64, req dto.PromptReq) (*dto.AdminPromptResp, error)
	DeletePrompt(ctx context.Context, id int64) error
	OptimizePrompt(ctx context.Context, id int64, req dto.PromptOptimizeReq) (*dto.PromptOptimizeResp, error)
}

type promptService struct {
	// 注入刚才定义的 Repository 接口
	repo repository.PromptRepository
}

func NewPromptService(repo repository.PromptRepository) PromptService {
	return &promptService{
		repo: repo,
	}
}

func (s *promptService) GetPromptList(ctx context.Context, req dto.PromptListReq) ([]dto.PromptListItem, error) {
	// 1. 调用 Repository 获取数据
	prompts, err := s.repo.GetPromptList(ctx, req.Type, req.Search)
	if err != nil {
		return nil, err
	}
	// 2. 将数据转换为 DTO 列表
	var resp []dto.PromptListItem
	for _, prompt := range prompts {
		resp = append(resp, dto.PromptListItem{
			ID:   uint64(prompt.ID),
			Note: derefString(prompt.Note),
			Type: prompt.CategoryID,
		})
	}
	return resp, nil
}

func (s *promptService) GetPromptDetail(ctx context.Context, id int64, isAdmin bool) (interface{}, error) {
	prompt, err := s.repo.GetPromptByID(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, constants.ErrPromptNotFound
		}
		return nil, err
	}

	if isAdmin || prompt.CategoryID >= 6 {
		return dto.AdminPromptResp{
			ID:         uint64(prompt.ID),
			CategoryID: prompt.CategoryID,
			Content:    prompt.Content,
			Note:       derefString(prompt.Note),
			CreatedAt:  prompt.CreatedAt,
		}, nil
	}

	return dto.UserPromptResp{
		ID:         uint64(prompt.ID),
		CategoryID: prompt.CategoryID,
		Note:       derefString(prompt.Note),
	}, nil
}

func (s *promptService) CreatePrompt(ctx context.Context, req dto.PromptReq) (*dto.AdminPromptResp, error) {
	if req.CategoryID > 2 {
		categoryExists, err := s.repo.CheckCategoryExists(ctx, req.CategoryID)
		if err != nil {
			return nil, err
		}
		if !categoryExists {
			return nil, constants.ErrPromptTypeNotFound
		}
	}

	prompt := &entity.Prompt{
		CategoryID: req.CategoryID,
		Content:    req.Content,
		Note:       stringPtrOrNil(req.Note),
	}
	if err := s.repo.CreatePrompt(ctx, prompt); err != nil {
		return nil, err
	}

	return &dto.AdminPromptResp{
		ID:         uint64(prompt.ID),
		CategoryID: prompt.CategoryID,
		Content:    prompt.Content,
		Note:       derefString(prompt.Note),
		CreatedAt:  prompt.CreatedAt,
	}, nil
}

func (s *promptService) UpdatePrompt(ctx context.Context, id int64, req dto.PromptReq) (*dto.AdminPromptResp, error) {
	if req.CategoryID > 2 {
		categoryExists, err := s.repo.CheckCategoryExists(ctx, req.CategoryID)
		if err != nil {
			return nil, err
		}
		if !categoryExists {
			return nil, constants.ErrPromptTypeNotFound
		}
	}

	prompt, err := s.repo.GetPromptByID(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, constants.ErrPromptNotFound
		}
		return nil, err
	}

	prompt.CategoryID = req.CategoryID
	prompt.Content = req.Content
	prompt.Note = stringPtrOrNil(req.Note)

	if err := s.repo.UpdatePrompt(ctx, prompt); err != nil {
		return nil, err
	}

	return &dto.AdminPromptResp{
		ID:         uint64(prompt.ID),
		CategoryID: prompt.CategoryID,
		Content:    prompt.Content,
		Note:       derefString(prompt.Note),
		CreatedAt:  prompt.CreatedAt,
	}, nil
}

func (s *promptService) DeletePrompt(ctx context.Context, id int64) error {
	_, err := s.repo.GetPromptByID(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return constants.ErrPromptNotFound
		}
		return err
	}
	return s.repo.DeletePrompt(ctx, id)
}

func (s *promptService) OptimizePrompt(ctx context.Context, id int64, req dto.PromptOptimizeReq) (*dto.PromptOptimizeResp, error) {
	prompt, err := s.repo.GetPromptByID(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, constants.ErrPromptNotFound
		}
		return nil, err
	}

	cfg, err := config.LoadConfig()
	if err != nil {
		return nil, constants.ErrLLMConfigMissing
	}

	client, err := llm.NewClient(cfg.LLM)
	if err != nil {
		return nil, constants.ErrLLMConfigMissing
	}

	optimized, err := client.OptimizePrompt(ctx, prompt.Content, req.Requirement)
	if err != nil {
		return nil, constants.ErrLLMRequestFailed
	}

	return &dto.PromptOptimizeResp{
		PromptID:         uint64(prompt.ID),
		OriginalContent:  prompt.Content,
		OptimizedContent: optimized,
	}, nil
}

func stringPtrOrNil(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}

func derefString(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}
