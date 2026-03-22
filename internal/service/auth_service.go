package service

import (
	"context"
	"errors"
	config "training_with_ai/configs"
	"training_with_ai/internal/model/dto"
	"training_with_ai/internal/model/entity"
	"training_with_ai/internal/pkg/auth"
	"training_with_ai/internal/pkg/constants"
	"training_with_ai/internal/repository"

	"gorm.io/gorm"
)

type AuthService interface {
	Login(ctx context.Context, req dto.AuthLoginReq) (int, string, error)
	Register(ctx context.Context, req dto.AuthRegisterReq) error
	Logout(ctx context.Context, userID int64, jti string) error
}

type authService struct {
	// 注入刚才定义的 Repository 接口
	repo repository.AuthRepository
}

func NewAuthService(repo repository.AuthRepository) AuthService {
	return &authService{
		repo: repo,
	}
}

func (s *authService) Login(ctx context.Context, req dto.AuthLoginReq) (int, string, error) {
	userEntity, err := s.repo.GetUserByUsername(ctx, req.Username)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return 0, "", constants.ErrUserNotFound
		}
		return 0, "", err
	}

	// 进行密码验证等逻辑
	userPassword := userEntity.PasswordHash

	isValid, err := auth.VerifyPassword(req.Password, userPassword)
	if err != nil {
		return 0, "", err
	}
	if !isValid {
		return 0, "", constants.ErrPasswordInvalid
	}

	token, err := auth.GenerateToken(userEntity.ID, userEntity.Username, userEntity.Role)
	if err != nil {
		return 0, "", err
	}

	if req.RememberMe == false {
		return -1, token, nil
	}
	cfg, err := config.LoadConfig()
	return cfg.JWT.ExpireHours, token, nil
}

func (s *authService) Register(ctx context.Context, req dto.AuthRegisterReq) error {
	// 检查用户名是否已存在

	exists, err := s.repo.CheckUserExistsByUsername(ctx, req.Username)
	if err != nil {
		return err
	}
	if exists {
		return constants.ErrUserAlreadyExists
	}

	// 如果不存在，生成密码哈希值
	hashedPassword, err := auth.GeneratePasswordHash(req.Password)
	if err != nil {
		return err
	}
	//创建用户记录并保存到数据库

	user := &entity.User{
		Username:     req.Username,
		PasswordHash: hashedPassword,
	}

	err = s.repo.CreateUser(ctx, user)
	if err != nil {
		return err
	}

	return nil
}

func (s *authService) Logout(ctx context.Context, userID int64, jti string) error {
	err := s.repo.Logout(ctx, userID, jti)
	if err != nil {
		return err
	}
	return nil
}
