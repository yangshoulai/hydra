package admin

import (
	"errors"
	"os"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type JWTService struct {
	secretKey []byte
}

type Claims struct {
	UserID   uint   `json:"user_id"`
	Username string `json:"username"`
	Type     string `json:"type"` // "access" or "refresh"
	jwt.RegisteredClaims
}

func NewJWTService() *JWTService {
	secret := os.Getenv("JWT_SECRET")
	if secret == "" {
		secret = "hydra-default-secret-change-in-production"
	}
	return &JWTService{
		secretKey: []byte(secret),
	}
}

// GenerateAccessToken 生成访问令牌（短有效期，2小时）
func (s *JWTService) GenerateAccessToken(userID uint, username string) (string, error) {
	return s.generateTokenWithType(userID, username, "access", 2*time.Hour)
}

// GenerateRefreshToken 生成刷新令牌（长有效期，7天）
func (s *JWTService) GenerateRefreshToken(userID uint, username string) (string, error) {
	return s.generateTokenWithType(userID, username, "refresh", 7*24*time.Hour)
}

// generateTokenWithType 根据类型生成不同有效期的令牌
func (s *JWTService) generateTokenWithType(userID uint, username string, tokenType string, expiration time.Duration) (string, error) {
	claims := Claims{
		UserID:   userID,
		Username: username,
		Type:     tokenType,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(expiration)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(s.secretKey)
}

// GenerateToken 生成访问令牌（兼容旧代码，默认24小时）
func (s *JWTService) GenerateToken(userID uint, username string) (string, error) {
	claims := Claims{
		UserID:   userID,
		Username: username,
		Type:     "access",
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(s.secretKey)
}

func (s *JWTService) ValidateToken(tokenString string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("invalid signing method")
		}
		return s.secretKey, nil
	})

	if err != nil {
		return nil, err
	}

	if claims, ok := token.Claims.(*Claims); ok && token.Valid {
		return claims, nil
	}

	return nil, errors.New("invalid token")
}
