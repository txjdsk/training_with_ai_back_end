package service

import (
	"context"
	"errors"
	"strings"
	"training_with_ai/internal/model/dto"
	"training_with_ai/internal/model/entity"
	"training_with_ai/internal/pkg/auth"
	"training_with_ai/internal/pkg/constants"
	"training_with_ai/internal/pkg/timeutil"
	"training_with_ai/internal/repository"

	"gorm.io/gorm"
)

type UserService interface {
	GetProfile(ctx context.Context, userID int64) (*dto.UserResp, error)
	UpdateProfile(ctx context.Context, userID int64, req dto.UpdateProfileReq) (*dto.UserResp, error)
	AdminList(ctx context.Context, req dto.AdminUserFilterReq) (*dto.PageResp, error)
	AdminCreate(ctx context.Context, req dto.AdminCreateUserReq) (*dto.UserResp, error)
	AdminUpdate(ctx context.Context, id int64, req dto.AdminUpdateUserReq) (*dto.UserResp, error)
	AdminDelete(ctx context.Context, id int64, operatorID int64) error
}
type userService struct {
	// 注入刚才定义的 Repository 接口
	repo repository.UserRepository
}

func NewUserService(repo repository.UserRepository) UserService {
	return &userService{
		repo: repo,
	}
}

func (s *userService) GetProfile(ctx context.Context, userID int64) (*dto.UserResp, error) {
	user, err := s.repo.GetByID(ctx, userID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, constants.ErrUserNotFound
		}
		return nil, err
	}

	return toUserResp(user), nil
}

func (s *userService) UpdateProfile(ctx context.Context, userID int64, req dto.UpdateProfileReq) (*dto.UserResp, error) {
	if strings.TrimSpace(req.Username) == "" && req.NewPassword == "" {
		return nil, constants.ErrParamInvalid
	}

	user, err := s.repo.GetByID(ctx, userID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, constants.ErrUserNotFound
		}
		return nil, err
	}

	if strings.TrimSpace(req.Username) != "" {
		exists, err := s.repo.CheckUsernameExistsExcludeID(ctx, req.Username, userID)
		if err != nil {
			return nil, err
		}
		if exists {
			return nil, constants.ErrUserAlreadyExists
		}
		user.Username = req.Username
	}

	if req.NewPassword != "" {
		if req.OldPassword == "" {
			return nil, constants.ErrParamInvalid
		}
		ok, err := auth.VerifyPassword(req.OldPassword, user.PasswordHash)
		if err != nil {
			return nil, err
		}
		if !ok {
			return nil, constants.ErrPasswordInvalid
		}
		hashed, err := auth.GeneratePasswordHash(req.NewPassword)
		if err != nil {
			return nil, err
		}
		user.PasswordHash = hashed
	}

	if err := s.repo.Update(ctx, user); err != nil {
		return nil, err
	}

	return toUserResp(user), nil
}

func (s *userService) AdminList(ctx context.Context, req dto.AdminUserFilterReq) (*dto.PageResp, error) {
	startTime, err := timeutil.ParseQueryTime(req.StartTime, false)
	if err != nil {
		return nil, constants.ErrParamInvalid
	}
	endTime, err := timeutil.ParseQueryTime(req.EndTime, true)
	if err != nil {
		return nil, constants.ErrParamInvalid
	}

	users, total, err := s.repo.List(ctx, req.Username, req.Role, startTime, endTime, req.Page, req.Size)
	if err != nil {
		return nil, err
	}

	respList := make([]dto.UserResp, 0, len(users))
	for _, user := range users {
		userCopy := user
		respList = append(respList, *toUserResp(&userCopy))
	}

	return &dto.PageResp{
		Total: total,
		List:  respList,
	}, nil
}

func (s *userService) AdminCreate(ctx context.Context, req dto.AdminCreateUserReq) (*dto.UserResp, error) {
	username := strings.TrimSpace(req.Username)
	if username == "" {
		return nil, constants.ErrParamInvalid
	}

	role := req.Role
	if role == "" {
		role = "user"
	}

	exists, err := s.repo.CheckUsernameExists(ctx, username)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, constants.ErrUserAlreadyExists
	}

	hashed, err := auth.GeneratePasswordHash(req.Password)
	if err != nil {
		return nil, err
	}

	user := &entity.User{
		Username:     username,
		PasswordHash: hashed,
		Role:         role,
	}
	if err := s.repo.Create(ctx, user); err != nil {
		return nil, err
	}

	return toUserResp(user), nil
}

func (s *userService) AdminUpdate(ctx context.Context, id int64, req dto.AdminUpdateUserReq) (*dto.UserResp, error) {
	user, err := s.repo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, constants.ErrUserNotFound
		}
		return nil, err
	}

	if user.Role == "root" && req.Role != "" && req.Role != "root" {
		return nil, constants.ErrNoPermission
	}

	if strings.TrimSpace(req.Username) != "" {
		exists, err := s.repo.CheckUsernameExistsExcludeID(ctx, req.Username, id)
		if err != nil {
			return nil, err
		}
		if exists {
			return nil, constants.ErrUserAlreadyExists
		}
		user.Username = req.Username
	}

	if req.Password != "" {
		hashed, err := auth.GeneratePasswordHash(req.Password)
		if err != nil {
			return nil, err
		}
		user.PasswordHash = hashed
	}

	if req.Role != "" {
		user.Role = req.Role
	}

	if err := s.repo.Update(ctx, user); err != nil {
		return nil, err
	}

	return toUserResp(user), nil
}

func (s *userService) AdminDelete(ctx context.Context, id int64, operatorID int64) error {
	user, err := s.repo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return constants.ErrUserNotFound
		}
		return err
	}
	if user.Role == "root" {
		return constants.ErrNoPermission
	}
	if user.ID == operatorID {
		return constants.ErrNoPermission
	}
	return s.repo.Delete(ctx, id)
}

func toUserResp(user *entity.User) *dto.UserResp {
	return &dto.UserResp{
		ID:        uint64(user.ID),
		Username:  user.Username,
		Role:      user.Role,
		CreatedAt: user.CreatedAt,
	}
}
