package auth

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type Claims struct {
	AdminID  string `json:"sub"`
	Username string `json:"username"`
	Role     string `json:"role"`
	TokenUse string `json:"use"` // "access" or "refresh"
	jwt.RegisteredClaims
}

type TokenManager struct {
	secret     []byte
	accessTTL  time.Duration
	refreshTTL time.Duration
}

func NewTokenManager(secret string, accessTTL, refreshTTL time.Duration) *TokenManager {
	return &TokenManager{secret: []byte(secret), accessTTL: accessTTL, refreshTTL: refreshTTL}
}

func (m *TokenManager) issue(adminID, username, role, use string, ttl time.Duration) (string, error) {
	claims := Claims{
		AdminID:  adminID,
		Username: username,
		Role:     role,
		TokenUse: use,
		RegisteredClaims: jwt.RegisteredClaims{
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(ttl)),
		},
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return tok.SignedString(m.secret)
}

func (m *TokenManager) IssueAccessToken(adminID, username, role string) (string, error) {
	return m.issue(adminID, username, role, "access", m.accessTTL)
}

func (m *TokenManager) IssueRefreshToken(adminID, username, role string) (string, error) {
	return m.issue(adminID, username, role, "refresh", m.refreshTTL)
}

var ErrInvalidToken = errors.New("invalid or expired token")

func (m *TokenManager) Parse(tokenStr string) (*Claims, error) {
	claims := &Claims{}
	tok, err := jwt.ParseWithClaims(tokenStr, claims, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, ErrInvalidToken
		}
		return m.secret, nil
	})
	if err != nil || !tok.Valid {
		return nil, ErrInvalidToken
	}
	return claims, nil
}
