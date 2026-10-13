-- SSH Manager Panel - initial schema
-- Run with: psql -U sshpanel -d sshpanel -f 0001_init.sql

CREATE EXTENSION IF NOT EXISTS pgcrypto;

-- Panel administrators (people who log into the web UI)
CREATE TABLE IF NOT EXISTS admins (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    username        TEXT UNIQUE NOT NULL,
    password_hash   TEXT NOT NULL,
    role            TEXT NOT NULL DEFAULT 'admin' CHECK (role IN ('admin', 'operator', 'viewer')),
    is_active       BOOLEAN NOT NULL DEFAULT TRUE,
    failed_logins   INTEGER NOT NULL DEFAULT 0,
    locked_until    TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- SSH accounts that exist as real Linux system accounts on the host.
-- `linux_uid` is only populated once the OS user has actually been created.
CREATE TABLE IF NOT EXISTS ssh_accounts (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    username        TEXT UNIQUE NOT NULL CHECK (username ~ '^[a-z_][a-z0-9_-]{2,31}$'),
    linux_uid       INTEGER,
    status          TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'disabled', 'expired', 'deleted')),
    expires_at      TIMESTAMPTZ,
    max_connections INTEGER NOT NULL DEFAULT 2,
    note            TEXT,
    created_by      UUID REFERENCES admins(id),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at      TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_ssh_accounts_status ON ssh_accounts(status);
CREATE INDEX IF NOT EXISTS idx_ssh_accounts_expires_at ON ssh_accounts(expires_at);

-- A snapshot of currently-open sessions, refreshed periodically from `ss`/utmp
-- by the stats poller. Not a log of history - history lives in connection_log.
CREATE TABLE IF NOT EXISTS active_sessions (
    id              BIGSERIAL PRIMARY KEY,
    account_id      UUID NOT NULL REFERENCES ssh_accounts(id) ON DELETE CASCADE,
    remote_addr     INET,
    local_port      INTEGER,
    pid             INTEGER,
    connected_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_seen_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (account_id, remote_addr, pid)
);

-- Append-only record of session start/end, kept for history and billing.
CREATE TABLE IF NOT EXISTS connection_log (
    id              BIGSERIAL PRIMARY KEY,
    account_id      UUID NOT NULL REFERENCES ssh_accounts(id) ON DELETE CASCADE,
    remote_addr     INET,
    started_at      TIMESTAMPTZ NOT NULL,
    ended_at        TIMESTAMPTZ,
    bytes_in        BIGINT NOT NULL DEFAULT 0,
    bytes_out       BIGINT NOT NULL DEFAULT 0
);

CREATE INDEX IF NOT EXISTS idx_connection_log_account ON connection_log(account_id, started_at DESC);

-- Periodic real counters (from nftables/iptables per-uid accounting) so
-- "usage per account" reflects actual traffic rather than estimates.
CREATE TABLE IF NOT EXISTS bandwidth_samples (
    id              BIGSERIAL PRIMARY KEY,
    account_id      UUID NOT NULL REFERENCES ssh_accounts(id) ON DELETE CASCADE,
    sampled_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    bytes_in_total  BIGINT NOT NULL,   -- cumulative counter value at sample time
    bytes_out_total BIGINT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_bandwidth_samples_account_time ON bandwidth_samples(account_id, sampled_at DESC);

-- TLS certificates issued for the panel's subdomain(s), tracked for renewal.
CREATE TABLE IF NOT EXISTS certificates (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    domain          TEXT UNIQUE NOT NULL,
    provider        TEXT NOT NULL DEFAULT 'letsencrypt',
    cert_path       TEXT NOT NULL,
    key_path        TEXT NOT NULL,
    issued_at       TIMESTAMPTZ,
    expires_at      TIMESTAMPTZ,
    status          TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'active', 'expiring', 'expired', 'failed')),
    last_error      TEXT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Every privileged action taken through the panel (account create/edit/delete,
-- cert issue, login) - required because the backend executes real OS commands.
CREATE TABLE IF NOT EXISTS audit_log (
    id              BIGSERIAL PRIMARY KEY,
    actor_admin_id  UUID REFERENCES admins(id),
    action          TEXT NOT NULL,
    target          TEXT,
    detail          JSONB,
    ip_addr         INET,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_audit_log_created_at ON audit_log(created_at DESC);
