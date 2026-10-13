package handlers

import (
	"context"
	"encoding/json"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"ssh-manager-panel/internal/models"
)

// AccountRepository is the persistence boundary for SSH account records.
// Kept as an interface so AccountService's business rules (status
// transitions, validation ordering) can be unit tested with a fake.
type AccountRepository interface {
	Insert(ctx context.Context, a *models.SSHAccount) error
	GetByUsername(ctx context.Context, username string) (*models.SSHAccount, error)
	GetByID(ctx context.Context, id string) (*models.SSHAccount, error)
	List(ctx context.Context) ([]*models.SSHAccount, error)
	UpdateUsername(ctx context.Context, id, newUsername string) error
	UpdateExpiryAndLimits(ctx context.Context, id string, expiresAt *time.Time, maxConn *int, note *string) error
	UpdateStatus(ctx context.Context, id string, status models.AccountStatus) error
	SetLinuxUID(ctx context.Context, id string, uid int) error
	SoftDelete(ctx context.Context, id string) error
	ListExpiredButActive(ctx context.Context, asOf time.Time) ([]*models.SSHAccount, error)
	InsertAuditLog(ctx context.Context, actorID *string, action, target string, detail map[string]any, ip string) error
}

type PGAccountRepository struct {
	pool *pgxpool.Pool
}

func NewPGAccountRepository(pool *pgxpool.Pool) *PGAccountRepository {
	return &PGAccountRepository{pool: pool}
}

const accountCols = `id, username, linux_uid, status, expires_at, max_connections, note, created_by, created_at, updated_at`

func scanAccount(row pgx.Row) (*models.SSHAccount, error) {
	var a models.SSHAccount
	err := row.Scan(&a.ID, &a.Username, &a.LinuxUID, &a.Status, &a.ExpiresAt, &a.MaxConnections, &a.Note, &a.CreatedBy, &a.CreatedAt, &a.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &a, nil
}

func (r *PGAccountRepository) Insert(ctx context.Context, a *models.SSHAccount) error {
	return r.pool.QueryRow(ctx, `
		INSERT INTO ssh_accounts (username, status, expires_at, max_connections, note, created_by)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, created_at, updated_at`,
		a.Username, a.Status, a.ExpiresAt, a.MaxConnections, a.Note, a.CreatedBy,
	).Scan(&a.ID, &a.CreatedAt, &a.UpdatedAt)
}

func (r *PGAccountRepository) GetByUsername(ctx context.Context, username string) (*models.SSHAccount, error) {
	row := r.pool.QueryRow(ctx, `SELECT `+accountCols+` FROM ssh_accounts WHERE username = $1 AND status != 'deleted'`, username)
	a, err := scanAccount(row)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	return a, err
}

func (r *PGAccountRepository) GetByID(ctx context.Context, id string) (*models.SSHAccount, error) {
	row := r.pool.QueryRow(ctx, `SELECT `+accountCols+` FROM ssh_accounts WHERE id = $1`, id)
	a, err := scanAccount(row)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	return a, err
}

func (r *PGAccountRepository) List(ctx context.Context) ([]*models.SSHAccount, error) {
	rows, err := r.pool.Query(ctx, `SELECT `+accountCols+` FROM ssh_accounts WHERE status != 'deleted' ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []*models.SSHAccount
	for rows.Next() {
		a, err := scanAccount(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

func (r *PGAccountRepository) UpdateUsername(ctx context.Context, id, newUsername string) error {
	_, err := r.pool.Exec(ctx, `UPDATE ssh_accounts SET username = $2, updated_at = now() WHERE id = $1`, id, newUsername)
	return err
}

func (r *PGAccountRepository) UpdateExpiryAndLimits(ctx context.Context, id string, expiresAt *time.Time, maxConn *int, note *string) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE ssh_accounts SET
			expires_at = COALESCE($2, expires_at),
			max_connections = COALESCE($3, max_connections),
			note = COALESCE($4, note),
			updated_at = now()
		WHERE id = $1`, id, expiresAt, maxConn, note)
	return err
}

func (r *PGAccountRepository) UpdateStatus(ctx context.Context, id string, status models.AccountStatus) error {
	_, err := r.pool.Exec(ctx, `UPDATE ssh_accounts SET status = $2, updated_at = now() WHERE id = $1`, id, status)
	return err
}

func (r *PGAccountRepository) SetLinuxUID(ctx context.Context, id string, uid int) error {
	_, err := r.pool.Exec(ctx, `UPDATE ssh_accounts SET linux_uid = $2, updated_at = now() WHERE id = $1`, id, uid)
	return err
}

func (r *PGAccountRepository) SoftDelete(ctx context.Context, id string) error {
	_, err := r.pool.Exec(ctx, `UPDATE ssh_accounts SET status = 'deleted', deleted_at = now(), updated_at = now() WHERE id = $1`, id)
	return err
}

func (r *PGAccountRepository) ListExpiredButActive(ctx context.Context, asOf time.Time) ([]*models.SSHAccount, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT `+accountCols+` FROM ssh_accounts
		WHERE status = 'active' AND expires_at IS NOT NULL AND expires_at <= $1`, asOf)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []*models.SSHAccount
	for rows.Next() {
		a, err := scanAccount(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

func (r *PGAccountRepository) InsertAuditLog(ctx context.Context, actorID *string, action, target string, detail map[string]any, ip string) error {
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
