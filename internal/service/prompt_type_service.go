package service

import (
	"context"
	"errors"
	"strings"
	"training_with_ai/internal/model/dto"
	"training_with_ai/internal/model/entity"
	"training_with_ai/internal/pkg/constants"
	"training_with_ai/internal/repository"

	"gorm.io/gorm"
)

type PromptTypeService interface {
	GetList(ctx context.Context) ([]dto.PromptCategoryResp, error)
	Create(ctx context.Context, req dto.PromptCategoryReq) (*dto.PromptCategoryResp, error)
	Update(ctx context.Context, id uint, req dto.PromptCategoryReq) (*dto.PromptCategoryResp, error)
	Delete(ctx context.Context, id uint) error
}
type promptTypeService struct {
	// 注入刚才定义的 Repository 接口
	repo repository.PromptTypeRepository
}

func NewPromptTypeService(repo repository.PromptTypeRepository) PromptTypeService {
	return &promptTypeService{
		repo: repo,
	}
}

func (s *promptTypeService) GetList(ctx context.Context) ([]dto.PromptCategoryResp, error) {
	categories, err := s.repo.List(ctx)
	if err != nil {
		return nil, err
	}

	resp := make([]dto.PromptCategoryResp, 0, len(categories))
	for _, category := range categories {
		resp = append(resp, dto.PromptCategoryResp{
			ID:   category.ID,
			Name: category.Name,
		})
	}
	return resp, nil
}

func (s *promptTypeService) Create(ctx context.Context, req dto.PromptCategoryReq) (*dto.PromptCategoryResp, error) {
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return nil, constants.ErrParamInvalid
	}
	if req.ID > 0 && req.ID < 7 {
		return nil, constants.ErrParamInvalid
	}

	if _, err := s.repo.GetByName(ctx, name); err == nil {
		return nil, constants.ErrPromptTypeExists
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	if req.ID > 0 {
		if _, err := s.repo.GetByID(ctx, req.ID); err == nil {
			return nil, constants.ErrPromptTypeExists
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, err
		}
	}

	category := &entity.PromptCategory{ID: req.ID, Name: name}
	if err := s.repo.Create(ctx, category); err != nil {
		return nil, err
	}

	return &dto.PromptCategoryResp{ID: category.ID, Name: category.Name}, nil
}

func (s *promptTypeService) Update(ctx context.Context, id uint, req dto.PromptCategoryReq) (*dto.PromptCategoryResp, error) {
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return nil, constants.ErrParamInvalid
	}
	if id <= 2 || id == 6 {
		return nil, constants.ErrNoPermission
	}

	category, err := s.repo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, constants.ErrPromptTypeNotFound
		}
		return nil, err
	}

	if existing, err := s.repo.GetByName(ctx, name); err == nil {
		if existing.ID != id {
			return nil, constants.ErrPromptTypeExists
		}
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	category.Name = name
	if err := s.repo.Update(ctx, category); err != nil {
		return nil, err
	}

	return &dto.PromptCategoryResp{ID: category.ID, Name: category.Name}, nil
}

func (s *promptTypeService) Delete(ctx context.Context, id uint) error {
	if id <= 2 || id == 6 {
		return constants.ErrNoPermission
	}
	if _, err := s.repo.GetByID(ctx, id); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return constants.ErrPromptTypeNotFound
		}
		return err
	}

	count, err := s.repo.CountPromptsByCategory(ctx, id)
	if err != nil {
		return err
	}
	if count > 0 {
		return constants.ErrPromptTypeInUse
	}

	return s.repo.Delete(ctx, id)
}
