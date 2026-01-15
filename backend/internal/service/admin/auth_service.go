package admin

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/yangshoulai/hydra/internal/models"
	"github.com/yangshoulai/hydra/internal/repository"
	"gorm.io/gorm"
)

var (
	// ErrInvalidCredentials 无效的凭证
	ErrInvalidCredentials = errors.New("invalid username or password")
	// ErrUserDisabled 用户已禁用
	ErrUserDisabled = errors.New("user is disabled")
	// ErrInvalidToken 无效的令牌
	ErrInvalidToken = errors.New("invalid or expired token")
)

// AuthService 认证服务
type AuthService struct {
	db              *gorm.DB
	logger          *slog.Logger
	adminUserRepo   *repository.AdminUserRepository
	accessTokenRepo *repository.AccessTokenRepository
}

// NewAuthService 创建认证服务
func NewAuthService(
	db *gorm.DB,
	logger *slog.Logger,
	adminUserRepo *repository.AdminUserRepository,
	accessTokenRepo *repository.AccessTokenRepository,
) *AuthService {
	return &AuthService{
		db:              db,
		logger:          logger,
		adminUserRepo:   adminUserRepo,
		accessTokenRepo: accessTokenRepo,
	}
}

// LoginRequest 登录请求
type LoginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

// LoginResponse 登录响应
type LoginResponse struct {
	Token string           `json:"token"`
	User  *AdminUserDetail `json:"user"`
}

// AdminUserDetail 管理员用户详情
type AdminUserDetail struct {
	ID          uint       `json:"id"`
	Username    string     `json:"username"`
	Status      string     `json:"status"`
	LastLoginAt *time.Time `json:"last_login_at"`
	CreatedAt   time.Time  `json:"created_at"`
}

// Login 管理员登录
func (s *AuthService) Login(ctx context.Context, req *LoginRequest) (*LoginResponse, error) {
	// 查询用户
	user, err := s.adminUserRepo.FindByUsername(ctx, req.Username)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			s.logger.Warn("login attempt with non-existent username",
				slog.String("username", req.Username),
			)
			return nil, ErrInvalidCredentials
		}
		s.logger.Error("failed to find user",
			slog.String("username", req.Username),
			slog.String("error", err.Error()),
		)
		return nil, fmt.Errorf("failed to find user: %w", err)
	}

	// 检查用户状态
	if !user.IsActive() {
		s.logger.Warn("login attempt for disabled user",
			slog.String("username", req.Username),
			slog.String("status", user.Status),
		)
		return nil, ErrUserDisabled
	}

	// 验证密码
	if !user.CheckPassword(req.Password) {
		s.logger.Warn("login attempt with invalid password",
			slog.String("username", req.Username),
		)
		return nil, ErrInvalidCredentials
	}

	// 生成访问令牌
	token, err := s.generateAccessToken(ctx)
	if err != nil {
		s.logger.Error("failed to generate access token",
			slog.Uint64("user_id", uint64(user.ID)),
			slog.String("error", err.Error()),
		)
		return nil, fmt.Errorf("failed to generate token: %w", err)
	}

	// 更新最后登录时间
	now := time.Now()
	user.LastLoginAt = &now
	if err := s.adminUserRepo.Update(ctx, user); err != nil {
		s.logger.Error("failed to update last login time",
			slog.Uint64("user_id", uint64(user.ID)),
			slog.String("error", err.Error()),
		)
		// 不影响登录流程，继续
	}

	s.logger.Info("user logged in successfully",
		slog.Uint64("user_id", uint64(user.ID)),
		slog.String("username", user.Username),
	)

	return &LoginResponse{
		Token: token,
		User: &AdminUserDetail{
			ID:          user.ID,
			Username:    user.Username,
			Status:      user.Status,
			LastLoginAt: user.LastLoginAt,
			CreatedAt:   user.CreatedAt,
		},
	}, nil
}

// Logout 管理员登出
func (s *AuthService) Logout(ctx context.Context, token string) error {
	// 对令牌进行哈希
	tokenHash := models.HashToken(token)

	// 查找令牌
	accessToken, err := s.accessTokenRepo.FindByTokenHash(ctx, tokenHash)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			// 令牌不存在,直接返回成功
			return nil
		}
		s.logger.Error("failed to find access token",
			slog.String("error", err.Error()),
		)
		return fmt.Errorf("failed to find token: %w", err)
	}

	// 禁用令牌
	accessToken.Status = "disabled"
	if err := s.accessTokenRepo.Update(ctx, accessToken); err != nil {
		s.logger.Error("failed to disable access token",
			slog.Uint64("token_id", uint64(accessToken.ID)),
			slog.String("error", err.Error()),
		)
		return fmt.Errorf("failed to disable token: %w", err)
	}

	s.logger.Info("user logged out successfully",
		slog.Uint64("token_id", uint64(accessToken.ID)),
	)

	return nil
}

// ValidateToken 验证访问令牌
func (s *AuthService) ValidateToken(ctx context.Context, token string) (*models.AccessToken, error) {
	// 对令牌进行哈希
	tokenHash := models.HashToken(token)

	// 查找令牌
	accessToken, err := s.accessTokenRepo.FindByTokenHash(ctx, tokenHash)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrInvalidToken
		}
		s.logger.Error("failed to find access token",
			slog.String("error", err.Error()),
		)
		return nil, fmt.Errorf("failed to find token: %w", err)
	}

	// 检查令牌状态
	if !accessToken.IsActive() {
		return nil, ErrInvalidToken
	}

	// 更新最后使用时间
	now := time.Now()
	accessToken.LastUsedAt = &now
	if err := s.accessTokenRepo.Update(ctx, accessToken); err != nil {
		s.logger.Error("failed to update token last used time",
			slog.Uint64("token_id", uint64(accessToken.ID)),
			slog.String("error", err.Error()),
		)
		// 不影响验证流程
	}

	return accessToken, nil
}

// generateAccessToken 生成随机访问令牌
func (s *AuthService) generateAccessToken(ctx context.Context) (string, error) {
	// 生成 32 字节随机数据
	randomBytes := make([]byte, 32)
	if _, err := rand.Read(randomBytes); err != nil {
		return "", fmt.Errorf("failed to generate random bytes: %w", err)
	}

	// Base64 编码
	token := base64.URLEncoding.EncodeToString(randomBytes)

	// 计算哈希
	tokenHash := models.HashToken(token)

	// 生成脱敏预览
	tokenPreview := models.MaskToken(token)

	// 创建令牌记录
	accessToken := &models.AccessToken{
		TokenHash:    tokenHash,
		TokenPreview: tokenPreview,
		Status:       "active",
		Name:         "Admin login token",
	}

	if err := s.accessTokenRepo.Create(ctx, accessToken); err != nil {
		s.logger.Error("failed to create access token record",
			slog.String("error", err.Error()),
		)
		return "", fmt.Errorf("failed to create token record: %w", err)
	}

	s.logger.Debug("access token generated",
		slog.Uint64("token_id", uint64(accessToken.ID)),
	)

	return token, nil
}

// GetCurrentUser 获取当前登录用户信息
// 通过令牌获取用户信息(管理后台只有一个管理员,所以简化处理)
func (s *AuthService) GetCurrentUser(ctx context.Context, token string) (*AdminUserDetail, error) {
	// 验证令牌
	_, err := s.ValidateToken(ctx, token)
	if err != nil {
		return nil, err
	}

	// 查询管理员用户(假设只有一个管理员)
	// 在实际应用中,应该将 AdminUserID 关联到 AccessToken
	users, err := s.adminUserRepo.List(ctx)
	if err != nil {
		s.logger.Error("failed to find admin users",
			slog.String("error", err.Error()),
		)
		return nil, fmt.Errorf("failed to find users: %w", err)
	}

	if len(users) == 0 {
		return nil, errors.New("no admin user found")
	}

	// 返回第一个激活的管理员
	for _, user := range users {
		if user.IsActive() {
			return &AdminUserDetail{
				ID:          user.ID,
				Username:    user.Username,
				Status:      user.Status,
				LastLoginAt: user.LastLoginAt,
				CreatedAt:   user.CreatedAt,
			}, nil
		}
	}

	return nil, errors.New("no active admin user found")
}
