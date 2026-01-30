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
)

// AuthService 认证服务
type AuthService struct {
	db              *gorm.DB
	logger          *slog.Logger
	adminUserRepo   *repository.AdminUserRepository
	accessTokenRepo *repository.AccessTokenRepository
	jwtService      *JWTService
}

// NewAuthService 创建认证服务
func NewAuthService(
	db *gorm.DB,
	logger *slog.Logger,
	adminUserRepo *repository.AdminUserRepository,
	accessTokenRepo *repository.AccessTokenRepository,
	jwtService *JWTService,
) *AuthService {
	if jwtService == nil {
		jwtService = NewJWTService()
	}
	return &AuthService{
		db:              db,
		logger:          logger,
		adminUserRepo:   adminUserRepo,
		accessTokenRepo: accessTokenRepo,
		jwtService:      jwtService,
	}
}

// LoginRequest 登录请求
type LoginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

// LoginResponse 登录响应
type LoginResponse struct {
	AccessToken  string           `json:"access_token"`
	RefreshToken string           `json:"refresh_token"`
	User         *AdminUserDetail `json:"user"`
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
			s.logger.Warn("尝试登录不存在的用户", slog.String("username", req.Username))
			return nil, ErrInvalidCredentials
		}
		s.logger.Error("未找到用户", slog.String("username", req.Username), slog.String("error", err.Error()))
		return nil, fmt.Errorf("failed to find user: %w", err)
	}

	// 防御性检查：确保 user 不为 nil
	if user == nil {
		s.logger.Warn("用户查询返回空结果", slog.String("username", req.Username))
		return nil, ErrInvalidCredentials
	}

	// 检查用户状态
	if !user.IsActive() {
		s.logger.Warn("用户被禁用", slog.String("username", req.Username), slog.String("status", user.Status))
		return nil, ErrUserDisabled
	}

	// 验证密码
	if !user.CheckPassword(req.Password) {
		s.logger.Warn("登录密码不正确", slog.String("username", req.Username))
		return nil, ErrInvalidCredentials
	}

	// 生成访问令牌和刷新令牌
	accessToken, err := s.jwtService.GenerateAccessToken(user.ID, user.Username)
	if err != nil {
		s.logger.Error("生成访问令牌失败", slog.Uint64("user_id", uint64(user.ID)), slog.String("error", err.Error()))
		return nil, fmt.Errorf("failed to generate access token: %w", err)
	}

	refreshToken, err := s.jwtService.GenerateRefreshToken(user.ID, user.Username)
	if err != nil {
		s.logger.Error("生成刷新令牌失败", slog.Uint64("user_id", uint64(user.ID)), slog.String("error", err.Error()))
		return nil, fmt.Errorf("failed to generate refresh token: %w", err)
	}

	// 更新最后登录时间
	now := time.Now()
	user.LastLoginAt = &now
	if err := s.adminUserRepo.Update(ctx, user); err != nil {
		s.logger.Error("更新最近登录时间异常", slog.Uint64("user_id", uint64(user.ID)), slog.String("error", err.Error()))
		// 不影响登录流程，继续
	}

	s.logger.Info("用户登录成功", slog.Uint64("user_id", uint64(user.ID)), slog.String("username", user.Username))

	return &LoginResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		User: &AdminUserDetail{
			ID:          user.ID,
			Username:    user.Username,
			Status:      user.Status,
			LastLoginAt: user.LastLoginAt,
			CreatedAt:   user.CreatedAt,
		},
	}, nil
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

	s.logger.Debug("密钥已生成",
		slog.Uint64("token_id", uint64(accessToken.ID)),
	)

	return token, nil
}

// ChangePasswordRequest 修改密码请求
type ChangePasswordRequest struct {
	OldPassword string `json:"old_password" binding:"required"`
	NewPassword string `json:"new_password" binding:"required,min=6"`
}

// ChangePassword 修改密码
func (s *AuthService) ChangePassword(ctx context.Context, userID uint, req *ChangePasswordRequest) error {
	// 查询用户
	user, err := s.adminUserRepo.FindByID(ctx, userID)
	if err != nil {
		s.logger.Error("failed to find user",
			slog.Uint64("user_id", uint64(userID)),
			slog.String("error", err.Error()),
		)
		return fmt.Errorf("failed to find user: %w", err)
	}

	// 防御性检查：确保 user 不为 nil
	if user == nil {
		s.logger.Error("用户查询返回空结果",
			slog.Uint64("user_id", uint64(userID)),
		)
		return errors.New("user not found")
	}

	// 验证旧密码
	if !user.CheckPassword(req.OldPassword) {
		s.logger.Warn("invalid old password",
			slog.Uint64("user_id", uint64(userID)),
		)
		return errors.New("invalid old password")
	}

	// 设置新密码
	if err := user.SetPassword(req.NewPassword); err != nil {
		s.logger.Error("failed to hash new password",
			slog.Uint64("user_id", uint64(userID)),
			slog.String("error", err.Error()),
		)
		return fmt.Errorf("failed to hash password: %w", err)
	}

	// 更新用户
	if err := s.adminUserRepo.Update(ctx, user); err != nil {
		s.logger.Error("failed to update user password",
			slog.Uint64("user_id", uint64(userID)),
			slog.String("error", err.Error()),
		)
		return fmt.Errorf("failed to update password: %w", err)
	}

	s.logger.Info("密码修改成功",
		slog.Uint64("user_id", uint64(userID)),
	)

	return nil
}

// RefreshTokenRequest 刷新令牌请求
type RefreshTokenRequest struct {
	RefreshToken string `json:"refresh_token" binding:"required"`
}

// RefreshTokenResponse 刷新令牌响应
type RefreshTokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
}

// RefreshToken 刷新访问令牌
func (s *AuthService) RefreshToken(ctx context.Context, req *RefreshTokenRequest) (*RefreshTokenResponse, error) {
	// 验证刷新令牌
	claims, err := s.jwtService.ValidateToken(req.RefreshToken)
	if err != nil {
		s.logger.Warn("刷新令牌验证失败", slog.String("error", err.Error()))
		return nil, errors.New("invalid refresh token")
	}

	// 检查是否为刷新令牌
	if claims.Type != "refresh" {
		s.logger.Warn("令牌类型错误", slog.String("type", claims.Type))
		return nil, errors.New("invalid token type")
	}

	// 查询用户
	user, err := s.adminUserRepo.FindByID(ctx, claims.UserID)
	if err != nil {
		s.logger.Error("未找到用户", slog.Uint64("user_id", uint64(claims.UserID)), slog.String("error", err.Error()))
		return nil, fmt.Errorf("failed to find user: %w", err)
	}

	// 防御性检查：确保 user 不为 nil
	if user == nil {
		s.logger.Warn("用户不存在", slog.Uint64("user_id", uint64(claims.UserID)))
		return nil, errors.New("user not found")
	}

	// 检查用户状态
	if !user.IsActive() {
		s.logger.Warn("用户被禁用", slog.Uint64("user_id", uint64(claims.UserID)))
		return nil, ErrUserDisabled
	}

	// 生成新的访问令牌和刷新令牌
	accessToken, err := s.jwtService.GenerateAccessToken(user.ID, user.Username)
	if err != nil {
		s.logger.Error("生成访问令牌失败", slog.Uint64("user_id", uint64(user.ID)), slog.String("error", err.Error()))
		return nil, fmt.Errorf("failed to generate access token: %w", err)
	}

	newRefreshToken, err := s.jwtService.GenerateRefreshToken(user.ID, user.Username)
	if err != nil {
		s.logger.Error("生成刷新令牌失败", slog.Uint64("user_id", uint64(user.ID)), slog.String("error", err.Error()))
		return nil, fmt.Errorf("failed to generate refresh token: %w", err)
	}

	s.logger.Info("令牌刷新成功", slog.Uint64("user_id", uint64(user.ID)), slog.String("username", user.Username))

	return &RefreshTokenResponse{
		AccessToken:  accessToken,
		RefreshToken: newRefreshToken,
	}, nil
}
