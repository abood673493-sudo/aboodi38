// Package config loads runtime configuration strictly from the environment
// (optionally via a .env file for local dev). No secret ever has a built-in
// default - a missing required value fails startup instead of silently
// falling back to something insecure.
package config

import (
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	// HTTP
	ListenAddr string

	// Database
	DatabaseURL string

	// Auth
	JWTSecret       string
	AccessTokenTTL  time.Duration
	RefreshTokenTTL time.Duration
	MaxFailedLogins int
	LockoutDuration time.Duration

	// SSH account management
	SSHGroup       string // Linux group every panel-managed account belongs to
	SudoHelperPath string // path to the privileged helper binary (see internal/sshmgr)
	SSHDPort       int    // local sshd TCP port the bridge forwards to
	ServiceUser    string // the unprivileged OS account this API process itself runs as

	// Stats polling
	StatsPollInterval time.Duration

	// Certificates - certbot's own path and the ACME webroot are fixed
	// inside sshpanel-helper itself (not configurable here) so a compromised
	// API process can't redirect them; CertStorageDir is only where this
	// process reads its own already-published, readable copy from (see
	// sshpanel-helper's publishCert).
	CertStorageDir string

	// WebSocket/TLS bridge
	BridgeListenAddr string
	BridgeTLSCert    string
	BridgeTLSKey     string
	BridgeDomain     string // the subdomain clients connect to, e.g. "ssh.example.com"; empty disables the Host check

	// Frontend origin allowed to call the API (CORS). Empty means same-origin
	// deployments only (nginx serving both) - see deploy/nginx.
	FrontendOrigin string
}

func mustEnv(key string) string {
	v := os.Getenv(key)
	if v == "" {
		fmt.Fprintf(os.Stderr, "fatal: required environment variable %s is not set\n", key)
		os.Exit(1)
	}
	return v
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func envIntOr(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}

func envDurationOr(key string, def time.Duration) time.Duration {
	if v := os.Getenv(key); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return def
}

// Load reads `.env` if present (dev convenience only - never required in
// production, where real env vars / systemd EnvironmentFile are used) and
// returns a validated Config, exiting the process if a required value is
// missing.
func Load() *Config {
	_ = godotenv.Load()

	cfg := &Config{
		ListenAddr:        envOr("LISTEN_ADDR", ":8080"),
		DatabaseURL:       mustEnv("DATABASE_URL"),
		JWTSecret:         mustEnv("JWT_SECRET"),
		AccessTokenTTL:    envDurationOr("ACCESS_TOKEN_TTL", 15*time.Minute),
		RefreshTokenTTL:   envDurationOr("REFRESH_TOKEN_TTL", 7*24*time.Hour),
		MaxFailedLogins:   envIntOr("MAX_FAILED_LOGINS", 5),
		LockoutDuration:   envDurationOr("LOCKOUT_DURATION", 15*time.Minute),
		SSHGroup:          envOr("SSH_MANAGED_GROUP", "sshpanel"),
		SudoHelperPath:    envOr("SUDO_HELPER_PATH", "/usr/local/bin/sshpanel-helper"),
		SSHDPort:          envIntOr("SSHD_PORT", 22),
		ServiceUser:       envOr("SERVICE_USER", "sshpanel-api"),
		StatsPollInterval: envDurationOr("STATS_POLL_INTERVAL", 30*time.Second),
		CertStorageDir:    envOr("CERT_STORAGE_DIR", "/etc/sshpanel/certs"),
		BridgeListenAddr:  envOr("BRIDGE_LISTEN_ADDR", ":8443"),
		BridgeTLSCert:     os.Getenv("BRIDGE_TLS_CERT"),
		BridgeTLSKey:      os.Getenv("BRIDGE_TLS_KEY"),
		BridgeDomain:      os.Getenv("BRIDGE_DOMAIN"),
		FrontendOrigin:    os.Getenv("FRONTEND_ORIGIN"),
	}

	if len(cfg.JWTSecret) < 32 {
		fmt.Fprintln(os.Stderr, "fatal: JWT_SECRET must be at least 32 characters")
		os.Exit(1)
	}

	return cfg
}
