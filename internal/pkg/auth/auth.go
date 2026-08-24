// Copyright 2026 zhouhouping. All Rights Reserved.

package auth

import (
	stderrors "errors"
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/golang-jwt/jwt/v5"
	"erp-system/internal/pkg/errors"
)

// Claims 是自有的 JWT 声明，包含租户、角色、权限以及一次性 jti。
// P0 整改：之前只有 user_id/username，无法支撑权限校验与登出黑名单。
type Claims struct {
	UserID   int64    `json:"user_id"`
	Username string   `json:"username"`
	TenantID int64    `json:"tenant_id"`
	Roles    []string `json:"roles,omitempty"`
	Perms    []string `json:"perms,omitempty"`
	JTI      string   `json:"jti"`
	jwt.RegisteredClaims
}

// GenerateToken 按完整 Claims 生成签名 token。
// 调用方应在登录/刷新时把 roles/perms/tenant/jti 全部装入。
func GenerateToken(secret string, claims Claims, expire time.Duration) (string, error) {
	now := time.Now()
	if claims.IssuedAt == nil {
		claims.IssuedAt = jwt.NewNumericDate(now)
	}
	if claims.NotBefore == nil {
		claims.NotBefore = jwt.NewNumericDate(now)
	}
	if claims.ExpiresAt == nil {
		claims.ExpiresAt = jwt.NewNumericDate(now.Add(expire))
	}
	if claims.Issuer == "" {
		claims.Issuer = "erp-system"
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(secret))
}

// ParseToken 校验签名、算法、过期时间，返回 Claims。
func ParseToken(secret, tokenString string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New(10003, 401, "token签名无效")
		}
		return []byte(secret), nil
	})
	if err != nil {
		// 区分过期与其他无效场景，便于前端决定是否刷新
		if stderrors.Is(err, jwt.ErrTokenExpired) {
			return nil, errors.New(10002, 401, "token已过期")
		}
		return nil, errors.New(10003, 401, "token无效")
	}
	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, errors.New(10003, 401, "token无效")
	}
	return claims, nil
}

func ValidatePassword(hashed, plain string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hashed), []byte(plain)) == nil
}

// HashPassword 使用 cost=12 生成 bcrypt 哈希，高于默认的 10，抵抗 GPU 暴力破解。
func HashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), 12)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}
