package auth

import (
	"context"
	"time"

	"ssh-manager-panel/internal/models"
)

// Repository is the persistence boundary auth.Service depends on. Keeping it
// as an interface (rather than a concrete *pgxpool.Pool) is what lets the
// service's business logic (lockout, failed-attempt counting) be unit
// tested with a fake, without a real Postgres instance.
type Repository interface {
	GetAdminByUsername(ctx context.Context, username string) (*models.Admin, error)
	RecordFailedLogin(ctx context.Context, adminID string, lockedUntil *time.Time) error
	ResetFailedLogins(ctx context.Context, adminID string) error
	InsertAuditLog(ctx context.Context, actorID *string, action, target string, detail map[string]any, ip string) error
}
