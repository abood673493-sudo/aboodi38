package auth

import (
	"context"
	"errors"
	"time"
)

var (
	ErrInvalidCredentials = errors.New("invalid username or password")
	ErrAccountLocked      = errors.New("account temporarily locked due to repeated failed logins")
	ErrAccountInactive    = errors.New("account is disabled")
)

type Service struct {
	repo            Repository
	tokens          *TokenManager
	maxFailedLogins int
	lockoutDuration time.Duration
	now             func() time.Time // overridable for tests
}

func NewService(repo Repository, tokens *TokenManager, maxFailedLogins int, lockoutDuration time.Duration) *Service {
	return &Service{
		repo:            repo,
		tokens:          tokens,
		maxFailedLogins: maxFailedLogins,
		lockoutDuration: lockoutDuration,
		now:             time.Now,
	}
}

type LoginResult struct {
	AccessToken  string
	RefreshToken string
	AdminID      string
	Username     string
	Role         string
}

// Login verifies credentials, enforces account lockout after repeated
// failures, and on success issues a fresh token pair. The same generic
// ErrInvalidCredentials is returned whether the username doesn't exist or
// the password is wrong, so the API never reveals which admins exist.
func (s *Service) Login(ctx context.Context, username, password, clientIP string) (*LoginResult, error) {
	admin, err := s.repo.GetAdminByUsername(ctx, username)
	if err != nil {
		return nil, ErrInvalidCredentials
	}
	if admin == nil {
		// still run a hash comparison against a dummy value so the response
		// time doesn't leak whether the username exists.
		CheckPassword("$2a$10$7EqJtq98hPqEX7fNZaFWoOhi5L1N3Qm3eJDHFM7hUu8zq6h3k2f0u", password)
		return nil, ErrInvalidCredentials
	}

	if admin.LockedUntil != nil && admin.LockedUntil.After(s.now()) {
		return nil, ErrAccountLocked
	}
	if !admin.IsActive {
		return nil, ErrAccountInactive
	}

	if !CheckPassword(admin.PasswordHash, password) {
		failed := admin.FailedLogins + 1
		var lockedUntil *time.Time
		if failed >= s.maxFailedLogins {
			t := s.now().Add(s.lockoutDuration)
			lockedUntil = &t
		}
		_ = s.repo.RecordFailedLogin(ctx, admin.ID, lockedUntil)
		_ = s.repo.InsertAuditLog(ctx, &admin.ID, "login_failed", admin.Username, map[string]any{"failed_logins": failed}, clientIP)
		if lockedUntil != nil {
			return nil, ErrAccountLocked
		}
		return nil, ErrInvalidCredentials
	}

	_ = s.repo.ResetFailedLogins(ctx, admin.ID)
	_ = s.repo.InsertAuditLog(ctx, &admin.ID, "login_success", admin.Username, nil, clientIP)

	access, err := s.tokens.IssueAccessToken(admin.ID, admin.Username, string(admin.Role))
	if err != nil {
		return nil, err
	}
	refresh, err := s.tokens.IssueRefreshToken(admin.ID, admin.Username, string(admin.Role))
	if err != nil {
		return nil, err
	}

	return &LoginResult{
		AccessToken:  access,
		RefreshToken: refresh,
		AdminID:      admin.ID,
		Username:     admin.Username,
		Role:         string(admin.Role),
	}, nil
}

// Refresh exchanges a valid refresh token for a new access token.
func (s *Service) Refresh(refreshToken string) (string, error) {
	claims, err := s.tokens.Parse(refreshToken)
	if err != nil {
		return "", ErrInvalidToken
	}
	if claims.TokenUse != "refresh" {
		return "", ErrInvalidToken
	}
	return s.tokens.IssueAccessToken(claims.AdminID, claims.Username, claims.Role)
}
