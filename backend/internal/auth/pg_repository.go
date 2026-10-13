package auth

import (
	"context"
	"encoding/json"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"ssh-manager-panel/internal/models"
)

// PGRepository is the real Postgres-backed Repository used in production.
type PGRepository struct {
	pool *pgxpool.Pool
}

func NewPGRepository(pool *pgxpool.Pool) *PGRepository {
	return &PGRepository{pool: pool}
}

func (r *PGRepository) GetAdminByUsername(ctx context.Context, username string) (*models.Admin, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT id, username, password_hash, role, is_active, failed_logins, locked_until, created_at
		FROM admins WHERE username = $1`, username)

	var a models.Admin
	err := row.Scan(&a.ID, &a.Username, &a.PasswordHash, &a.Role, &a.IsActive, &a.FailedLogins, &a.LockedUntil, &a.CreatedAt)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &a, nil
}

func (r *PGRepository) RecordFailedLogin(ctx context.Context, adminID string, lockedUntil *time.Time) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE admins SET failed_logins = failed_logins + 1, locked_until = $2, updated_at = now()
		WHERE id = $1`, adminID, lockedUntil)
	return err
}

func (r *PGRepository) ResetFailedLogins(ctx context.Context, adminID string) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE admins SET failed_logins = 0, locked_until = NULL, updated_at = now()
		WHERE id = $1`, adminID)
	return err
}

func (r *PGRepository) InsertAuditLog(ctx context.Context, actorID *string, action, target string, detail map[string]any, ip string) error {
	var detailJSON []byte
	if detail != nil {
		detailJSON, _ = json.Marshal(detail)
	}
	_, err := r.pool.Exec(ctx, `
		INSERT INTO audit_log (actor_admin_id, action, target, detail, ip_addr)
		VALUES ($1, $2, $3, $4, NULLIF($5, '')::inet)`,
		actorID, action, target, detailJSON, ip)
	return err
}
