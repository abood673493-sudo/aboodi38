package handlers

import (
	"context"
	"errors"
	"fmt"
	"time"

	"ssh-manager-panel/internal/auth"
	"ssh-manager-panel/internal/models"
	"ssh-manager-panel/internal/sshmgr"
)

var (
	ErrUsernameTaken   = errors.New("username already exists")
	ErrAccountNotFound = errors.New("account not found")
)

// SSHManager is the subset of *sshmgr.Manager that AccountService needs.
// Declaring it here (rather than depending on the concrete type) is what
// lets service-level tests exercise real business logic - "don't call
// useradd twice", "roll back the DB row if the OS user creation fails" -
// without ever touching a real Linux system.
type SSHManager interface {
	CreateAccount(ctx context.Context, username, password string) error
	SetPassword(ctx context.Context, username, newPassword string) error
	RenameAccount(ctx context.Context, oldUsername, newUsername string) error
	DisableAccount(ctx context.Context, username string) error
	EnableAccount(ctx context.Context, username string) error
	DeleteAccount(ctx context.Context, username string) error
	SetExpiration(ctx context.Context, username string, expiresAt *time.Time) error
	LookupUID(ctx context.Context, username string) (int, error)
}

type AccountService struct {
	repo AccountRepository
	os   SSHManager
	now  func() time.Time
}

func NewAccountService(repo AccountRepository, os SSHManager) *AccountService {
	return &AccountService{repo: repo, os: os, now: time.Now}
}

// CreateAccount creates both the database record and the real Linux user,
// in that order, so a failure to create the OS user leaves no DB row
// pointing at an account that doesn't actually exist. The DB row is marked
// deleted (not left dangling) if the OS step fails after the row exists.
func (s *AccountService) CreateAccount(ctx context.Context, actorID string, req models.CreateAccountRequest) (*models.SSHAccount, error) {
	if err := sshmgr.ValidateUsername(req.Username); err != nil {
		return nil, err
	}
	if err := auth.ValidatePasswordStrength(req.Password); err != nil {
		return nil, err
	}
	if existing, err := s.repo.GetByUsername(ctx, req.Username); err != nil {
		return nil, err
	} else if existing != nil {
		return nil, ErrUsernameTaken
	}
	if req.MaxConnections <= 0 {
		req.MaxConnections = 2
	}

	account := &models.SSHAccount{
		Username:       req.Username,
		Status:         models.StatusActive,
		ExpiresAt:      req.ExpiresAt,
		MaxConnections: req.MaxConnections,
		Note:           req.Note,
		CreatedBy:      &actorID,
	}
	if err := s.repo.Insert(ctx, account); err != nil {
		return nil, fmt.Errorf("insert account record: %w", err)
	}

	if err := s.os.CreateAccount(ctx, req.Username, req.Password); err != nil {
		// Roll back: the DB row must not outlive the OS account it claims
		// to represent.
		_ = s.repo.SoftDelete(ctx, account.ID)
		return nil, fmt.Errorf("create OS account: %w", err)
	}

	if req.ExpiresAt != nil {
		if err := s.os.SetExpiration(ctx, req.Username, req.ExpiresAt); err != nil {
			return nil, fmt.Errorf("set OS expiration: %w", err)
		}
	}

	if uid, err := s.os.LookupUID(ctx, req.Username); err == nil {
		_ = s.repo.SetLinuxUID(ctx, account.ID, uid)
		account.LinuxUID = &uid
	}

	_ = s.repo.InsertAuditLog(ctx, &actorID, "account_created", req.Username, map[string]any{"expires_at": req.ExpiresAt}, "")
	return account, nil
}

func (s *AccountService) List(ctx context.Context) ([]*models.SSHAccount, error) {
	return s.repo.List(ctx)
}

func (s *AccountService) Get(ctx context.Context, id string) (*models.SSHAccount, error) {
	a, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if a == nil {
		return nil, ErrAccountNotFound
	}
	return a, nil
}

// Update applies username/password/expiry/limit changes. OS-level changes
// are applied first for username/password (the operations that must
// actually succeed on the host); DB-level fields are updated after.
func (s *AccountService) Update(ctx context.Context, actorID, id string, req models.UpdateAccountRequest) (*models.SSHAccount, error) {
	account, err := s.Get(ctx, id)
	if err != nil {
		return nil, err
	}

	targetUsername := account.Username

	if req.NewUsername != nil && *req.NewUsername != account.Username {
		if err := sshmgr.ValidateUsername(*req.NewUsername); err != nil {
			return nil, err
		}
		if existing, err := s.repo.GetByUsername(ctx, *req.NewUsername); err != nil {
			return nil, err
		} else if existing != nil {
			return nil, ErrUsernameTaken
		}
		if err := s.os.RenameAccount(ctx, account.Username, *req.NewUsername); err != nil {
			return nil, fmt.Errorf("rename OS account: %w", err)
		}
		if err := s.repo.UpdateUsername(ctx, id, *req.NewUsername); err != nil {
			return nil, err
		}
		targetUsername = *req.NewUsername
		account.Username = *req.NewUsername
	}

	if req.NewPassword != nil && *req.NewPassword != "" {
		if err := auth.ValidatePasswordStrength(*req.NewPassword); err != nil {
			return nil, err
		}
		if err := s.os.SetPassword(ctx, targetUsername, *req.NewPassword); err != nil {
			return nil, fmt.Errorf("set OS password: %w", err)
		}
		_ = s.repo.InsertAuditLog(ctx, &actorID, "password_changed", targetUsername, nil, "")
	}

	if req.ExpiresAt != nil {
		if err := s.os.SetExpiration(ctx, targetUsername, req.ExpiresAt); err != nil {
			return nil, fmt.Errorf("set OS expiration: %w", err)
		}
	}

	if err := s.repo.UpdateExpiryAndLimits(ctx, id, req.ExpiresAt, req.MaxConnections, req.Note); err != nil {
		return nil, err
	}

	_ = s.repo.InsertAuditLog(ctx, &actorID, "account_updated", targetUsername, nil, "")
	return s.Get(ctx, id)
}

func (s *AccountService) Disable(ctx context.Context, actorID, id string) error {
	account, err := s.Get(ctx, id)
	if err != nil {
		return err
	}
	if err := s.os.DisableAccount(ctx, account.Username); err != nil {
		return fmt.Errorf("disable OS account: %w", err)
	}
	if err := s.repo.UpdateStatus(ctx, id, models.StatusDisabled); err != nil {
		return err
	}
	_ = s.repo.InsertAuditLog(ctx, &actorID, "account_disabled", account.Username, nil, "")
	return nil
}

func (s *AccountService) Enable(ctx context.Context, actorID, id string) error {
	account, err := s.Get(ctx, id)
	if err != nil {
		return err
	}
	if err := s.os.EnableAccount(ctx, account.Username); err != nil {
		return fmt.Errorf("enable OS account: %w", err)
	}
	if err := s.repo.UpdateStatus(ctx, id, models.StatusActive); err != nil {
		return err
	}
	_ = s.repo.InsertAuditLog(ctx, &actorID, "account_enabled", account.Username, nil, "")
	return nil
}

// Delete removes the real Linux account and marks the DB row deleted. The
// DB row is kept (soft-deleted) rather than erased, so audit_log entries
// referencing it remain valid and the dashboard can show recent deletions.
func (s *AccountService) Delete(ctx context.Context, actorID, id string) error {
	account, err := s.Get(ctx, id)
	if err != nil {
		return err
	}
	if err := s.os.DeleteAccount(ctx, account.Username); err != nil {
		return fmt.Errorf("delete OS account: %w", err)
	}
	if err := s.repo.SoftDelete(ctx, id); err != nil {
		return err
	}
	_ = s.repo.InsertAuditLog(ctx, &actorID, "account_deleted", account.Username, nil, "")
	return nil
}

// ReapExpired finds accounts whose expiry has passed but whose DB status
// hasn't caught up yet (e.g. the panel was down when it passed) and
// disables them. The OS-level `chage -E` already blocks login independent
// of this running, so ReapExpired is about keeping the dashboard's status
// column accurate, not the actual access control.
func (s *AccountService) ReapExpired(ctx context.Context) (int, error) {
	expired, err := s.repo.ListExpiredButActive(ctx, s.now())
	if err != nil {
		return 0, err
	}
	count := 0
	for _, a := range expired {
		if err := s.os.DisableAccount(ctx, a.Username); err != nil {
			continue
		}
		if err := s.repo.UpdateStatus(ctx, a.ID, models.StatusExpired); err != nil {
			continue
		}
		_ = s.repo.InsertAuditLog(ctx, nil, "account_expired", a.Username, nil, "")
		count++
	}
	return count, nil
}
