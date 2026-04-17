package admin

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/yangshoulai/hydra/internal/repository"
)

var (
	// ErrInvalidCredentials 无效的凭证
	ErrInvalidCredentials = errors.New("invalid username or password")
	// ErrInvalidOldPassword 原密码错误
	ErrInvalidOldPassword = errors.New("invalid old password")
)

// AuthService 认证服务
type AuthService struct {
	logger        *slog.Logger
	adminUserRepo *repository.AdminUserRepository
	jwtService    *JWTService
}

// NewAuthService 创建认证服务
func NewAuthService(
	logger *slog.Logger,
	adminUserRepo *repository.AdminUserRepository,
	jwtService *JWTService,
) *AuthService {
	return &AuthService{
		logger:        logger,
		adminUserRepo: adminUserRepo,
		jwtService:    jwtService,
	}
}

// LoginRequest 登录请求
type LoginRequest struct {
	Username   string `json:"username" binding:"required"`
	Password   string `json:"password" binding:"required"`
	RememberMe bool   `json:"remember_me"`
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
	LastLoginAt *time.Time `json:"last_login_at"`
	CreatedAt   time.Time  `json:"created_at"`
}

// Login 管理员登录
func (s *AuthService) Login(ctx context.Context, req *LoginRequest) (*LoginResponse, error) {
	// 查询用户
	user, err := s.adminUserRepo.FindByUsername(ctx, req.Username)
	if err != nil {
		s.logger.Error("未找到用户", slog.String("username", req.Username), slog.String("error", err.Error()))
		return nil, fmt.Errorf("failed to find user: %w", err)
	}

	// 仓储未命中用户
	if user == nil {
		s.logger.Warn("用户查询返回空结果", slog.String("username", req.Username))
		return nil, ErrInvalidCredentials
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

	refreshToken, err := s.jwtService.GenerateRefreshToken(user.ID, user.Username, req.RememberMe)
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
			LastLoginAt: user.LastLoginAt,
			CreatedAt:   user.CreatedAt,
		},
	}, nil
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

	// 仓储未命中用户
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
		return ErrInvalidOldPassword
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

	// 仓储未命中用户
	if user == nil {
		s.logger.Warn("用户不存在", slog.Uint64("user_id", uint64(claims.UserID)))
		return nil, errors.New("user not found")
	}

	// 生成新的访问令牌和刷新令牌
	accessToken, err := s.jwtService.GenerateAccessToken(user.ID, user.Username)
	if err != nil {
		s.logger.Error("生成访问令牌失败", slog.Uint64("user_id", uint64(user.ID)), slog.String("error", err.Error()))
		return nil, fmt.Errorf("failed to generate access token: %w", err)
	}

	remainingTTL := RefreshTokenDefaultTTL
	if claims.ExpiresAt != nil {
		remainingTTL = time.Until(claims.ExpiresAt.Time)
	}
	if remainingTTL <= 0 {
		s.logger.Warn("刷新令牌已过期",
			slog.Uint64("user_id", uint64(user.ID)),
		)
		return nil, errors.New("refresh token expired")
	}
	if remainingTTL > RefreshTokenRememberTTL {
		remainingTTL = RefreshTokenRememberTTL
	}

	newRefreshToken, err := s.jwtService.GenerateRefreshTokenWithTTL(user.ID, user.Username, remainingTTL)
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
