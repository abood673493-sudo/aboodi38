// Package models holds the plain data types shared across the backend.
package models

import "time"

type AdminRole string

const (
	RoleAdmin    AdminRole = "admin"
	RoleOperator AdminRole = "operator"
	RoleViewer   AdminRole = "viewer"
)

type Admin struct {
	ID           string     `json:"id"`
	Username     string     `json:"username"`
	PasswordHash string     `json:"-"`
	Role         AdminRole  `json:"role"`
	IsActive     bool       `json:"is_active"`
	FailedLogins int        `json:"-"`
	LockedUntil  *time.Time `json:"-"`
	CreatedAt    time.Time  `json:"created_at"`
}

type AccountStatus string

const (
	StatusActive   AccountStatus = "active"
	StatusDisabled AccountStatus = "disabled"
	StatusExpired  AccountStatus = "expired"
	StatusDeleted  AccountStatus = "deleted"
)

type SSHAccount struct {
	ID             string        `json:"id"`
	Username       string        `json:"username"`
	LinuxUID       *int          `json:"linux_uid,omitempty"`
	Status         AccountStatus `json:"status"`
	ExpiresAt      *time.Time    `json:"expires_at,omitempty"`
	MaxConnections int           `json:"max_connections"`
	Note           string        `json:"note,omitempty"`
	CreatedBy      *string       `json:"created_by,omitempty"`
	CreatedAt      time.Time     `json:"created_at"`
	UpdatedAt      time.Time     `json:"updated_at"`

	// Populated by joins/aggregation, not stored directly on this table.
	ActiveConnections int   `json:"active_connections"`
	BytesInRecent     int64 `json:"bytes_in_recent"`
	BytesOutRecent    int64 `json:"bytes_out_recent"`
}

type CreateAccountRequest struct {
	Username       string     `json:"username"`
	Password       string     `json:"password"`
	ExpiresAt      *time.Time `json:"expires_at,omitempty"`
	MaxConnections int        `json:"max_connections"`
	Note           string     `json:"note,omitempty"`
}

type UpdateAccountRequest struct {
	NewUsername    *string    `json:"new_username,omitempty"`
	NewPassword    *string    `json:"new_password,omitempty"`
	ExpiresAt      *time.Time `json:"expires_at,omitempty"`
	MaxConnections *int       `json:"max_connections,omitempty"`
	Note           *string    `json:"note,omitempty"`
}

type ActiveSession struct {
	AccountID   string    `json:"account_id"`
	Username    string    `json:"username"`
	RemoteAddr  string    `json:"remote_addr"`
	ConnectedAt time.Time `json:"connected_at"`
}

type Certificate struct {
	ID        string     `json:"id"`
	Domain    string     `json:"domain"`
	Provider  string     `json:"provider"`
	Status    string     `json:"status"`
	IssuedAt  *time.Time `json:"issued_at,omitempty"`
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
	LastError string     `json:"last_error,omitempty"`
}

type ServerStatus struct {
	Hostname       string  `json:"hostname"`
	UptimeSeconds  int64   `json:"uptime_seconds"`
	LoadAvg1       float64 `json:"load_avg_1"`
	LoadAvg5       float64 `json:"load_avg_5"`
	LoadAvg15      float64 `json:"load_avg_15"`
	MemTotalBytes  uint64  `json:"mem_total_bytes"`
	MemUsedBytes   uint64  `json:"mem_used_bytes"`
	DiskTotalBytes uint64  `json:"disk_total_bytes"`
	DiskUsedBytes  uint64  `json:"disk_used_bytes"`
	TotalAccounts  int     `json:"total_accounts"`
	ActiveAccounts int     `json:"active_accounts"`
	ActiveSessions int     `json:"active_sessions"`
}
