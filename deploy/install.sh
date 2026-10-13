#!/usr/bin/env bash
#
# SSH Manager Panel - Ubuntu installer.
#
# What this does, in order: installs OS packages (PostgreSQL, nginx,
# certbot), installs Go/Node if missing or too old, creates the restricted
# service account, builds the backend and frontend, installs the privileged
# helper + its sudoers rule + the sshd hardening drop-in, sets up the
# database, writes the environment file, installs the systemd units, and
# creates the first admin.
#
# It does NOT issue a TLS certificate or start the WebSocket/TLS bridge -
# that needs your domain's DNS to already point at this server first (see
# the "Next steps" printed at the end).
#
# Review this script before running it, as with any script that configures
# system accounts, sudo rules and services. Run as root:
#   sudo bash install.sh
#
# Safe to re-run: every step either checks first or is naturally idempotent
# (useradd -f-style guards, CREATE ... IF NOT EXISTS, etc.).

set -euo pipefail

# --- resolve paths relative to this script, not the caller's cwd ---
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
BACKEND_DIR="$REPO_ROOT/backend"
FRONTEND_DIR="$REPO_ROOT/frontend"

INSTALL_PREFIX="/opt/sshpanel"
CONFIG_DIR="/etc/sshpanel"
SERVICE_USER="sshpanel-api"
SSH_GROUP="sshpanel"
ACME_WEBROOT="/var/www/sshpanel-acme"

log()  { echo -e "\n\033[1;32m==>\033[0m $*"; }
warn() { echo -e "\033[1;33m!!\033[0m $*" >&2; }
die()  { echo -e "\033[1;31merror:\033[0m $*" >&2; exit 1; }

[[ $EUID -eq 0 ]] || die "run as root (sudo bash install.sh)"
command -v apt-get >/dev/null 2>&1 || die "this installer is for Ubuntu/Debian (apt-get not found)"

# ---------------------------------------------------------------------------
log "Installing OS packages (PostgreSQL, nginx, certbot, build tools)"
# ---------------------------------------------------------------------------
export DEBIAN_FRONTEND=noninteractive
apt-get update -y
apt-get install -y --no-install-recommends \
    postgresql postgresql-contrib \
    nginx \
    certbot \
    curl ca-certificates gnupg build-essential git jq

systemctl enable --now postgresql

# ---------------------------------------------------------------------------
log "Checking Go toolchain (need >= 1.22)"
# ---------------------------------------------------------------------------
GO_MIN_MINOR=22
need_go_install=true
if command -v go >/dev/null 2>&1; then
    go_minor="$(go env GOVERSION 2>/dev/null | sed -E 's/go1\.([0-9]+).*/\1/')"
    if [[ "${go_minor:-0}" =~ ^[0-9]+$ ]] && (( go_minor >= GO_MIN_MINOR )); then
        need_go_install=false
    fi
fi
if $need_go_install; then
    GO_VERSION="1.22.6"
    ARCH="$(dpkg --print-architecture)"
    log "Installing Go $GO_VERSION ($ARCH) to /usr/local/go"
    curl -fsSL "https://go.dev/dl/go${GO_VERSION}.linux-${ARCH}.tar.gz" -o /tmp/go.tar.gz
    rm -rf /usr/local/go
    tar -C /usr/local -xzf /tmp/go.tar.gz
    rm -f /tmp/go.tar.gz
    ln -sf /usr/local/go/bin/go /usr/local/bin/go
    ln -sf /usr/local/go/bin/gofmt /usr/local/bin/gofmt
fi
log "Using $(go version)"

# ---------------------------------------------------------------------------
log "Checking Node.js toolchain (need >= 20)"
# ---------------------------------------------------------------------------
NODE_MIN_MAJOR=20
need_node_install=true
if command -v node >/dev/null 2>&1; then
    node_major="$(node -v | sed -E 's/^v([0-9]+).*/\1/')"
    if [[ "${node_major:-0}" =~ ^[0-9]+$ ]] && (( node_major >= NODE_MIN_MAJOR )); then
        need_node_install=false
    fi
fi
if $need_node_install; then
    log "Installing Node.js 20.x via NodeSource"
    curl -fsSL https://deb.nodesource.com/setup_20.x | bash -
    apt-get install -y nodejs
fi
log "Using node $(node -v), npm $(npm -v)"

# ---------------------------------------------------------------------------
log "Creating service account and group"
# ---------------------------------------------------------------------------
groupadd -f "$SSH_GROUP"
id -u "$SERVICE_USER" >/dev/null 2>&1 || \
    useradd --system --no-create-home --shell /usr/sbin/nologin "$SERVICE_USER"

# ---------------------------------------------------------------------------
log "Creating directories"
# ---------------------------------------------------------------------------
install -d -m 0755 "$INSTALL_PREFIX/bin" "$INSTALL_PREFIX/web" "$CONFIG_DIR"
install -d -m 0755 -o www-data -g www-data "$ACME_WEBROOT"

# ---------------------------------------------------------------------------
log "Building backend"
# ---------------------------------------------------------------------------
export PATH="/usr/local/go/bin:$PATH"
pushd "$BACKEND_DIR" >/dev/null
go build -o "$INSTALL_PREFIX/bin/server" ./cmd/server
go build -o "$INSTALL_PREFIX/bin/bootstrap-admin" ./cmd/bootstrap-admin
go build -o /usr/local/bin/sshpanel-helper ./cmd/sshpanel-helper
popd >/dev/null
chown root:root /usr/local/bin/sshpanel-helper
chmod 0755 /usr/local/bin/sshpanel-helper
chown -R "$SERVICE_USER:$SERVICE_USER" "$INSTALL_PREFIX/bin"

# ---------------------------------------------------------------------------
log "Building frontend"
# ---------------------------------------------------------------------------
pushd "$FRONTEND_DIR" >/dev/null
npm ci
npm run build
popd >/dev/null
rm -rf "${INSTALL_PREFIX:?}/web"/*
cp -r "$FRONTEND_DIR/dist/." "$INSTALL_PREFIX/web/"
chown -R root:root "$INSTALL_PREFIX/web"

# ---------------------------------------------------------------------------
log "Installing sudoers rule, sshd hardening, systemd units"
# ---------------------------------------------------------------------------
install -m 0440 "$SCRIPT_DIR/sudoers.d/sshpanel" /etc/sudoers.d/sshpanel
visudo -c -f /etc/sudoers.d/sshpanel || die "sudoers rule failed validation - not installed"

install -m 0644 "$SCRIPT_DIR/sshd/99-sshpanel.conf" /etc/ssh/sshd_config.d/99-sshpanel.conf
sshd -t && systemctl reload ssh || warn "sshd config check failed - review /etc/ssh/sshd_config.d/99-sshpanel.conf by hand"

install -m 0644 "$SCRIPT_DIR/systemd/sshpanel-api.service" /etc/systemd/system/sshpanel-api.service
install -m 0644 "$SCRIPT_DIR/systemd/sshpanel-cert-renew.service" /etc/systemd/system/sshpanel-cert-renew.service
install -m 0644 "$SCRIPT_DIR/systemd/sshpanel-cert-renew.timer" /etc/systemd/system/sshpanel-cert-renew.timer
install -d -m 0755 /etc/systemd/system/user-.slice.d
install -m 0644 "$SCRIPT_DIR/systemd/user-slice-ipaccounting.conf" /etc/systemd/system/user-.slice.d/override.conf
systemctl daemon-reload

# ---------------------------------------------------------------------------
log "Setting up PostgreSQL database"
# ---------------------------------------------------------------------------
DB_NAME="sshpanel"
DB_USER="sshpanel"
if [[ -f "$CONFIG_DIR/db_password" ]]; then
    DB_PASSWORD="$(cat "$CONFIG_DIR/db_password")"
else
    DB_PASSWORD="$(openssl rand -hex 24)"
    install -m 0600 /dev/stdin "$CONFIG_DIR/db_password" <<<"$DB_PASSWORD"
fi

su postgres -c "psql -tc \"SELECT 1 FROM pg_roles WHERE rolname='${DB_USER}'\"" | grep -q 1 || \
    su postgres -c "psql -c \"CREATE ROLE ${DB_USER} LOGIN PASSWORD '${DB_PASSWORD}';\""
su postgres -c "psql -tc \"SELECT 1 FROM pg_database WHERE datname='${DB_NAME}'\"" | grep -q 1 || \
    su postgres -c "psql -c \"CREATE DATABASE ${DB_NAME} OWNER ${DB_USER};\""
su postgres -c "psql -d ${DB_NAME} -c \"GRANT ALL ON SCHEMA public TO ${DB_USER};\""

psql "postgres://${DB_USER}:${DB_PASSWORD}@127.0.0.1:5432/${DB_NAME}" \
    -f "$BACKEND_DIR/migrations/0001_init.sql"

# ---------------------------------------------------------------------------
log "Writing /etc/sshpanel/sshpanel.env"
# ---------------------------------------------------------------------------
if [[ -f "$CONFIG_DIR/sshpanel.env" ]]; then
    warn "$CONFIG_DIR/sshpanel.env already exists - leaving it untouched"
else
    JWT_SECRET="$(openssl rand -base64 48 | tr -d '\n')"
    cat > "$CONFIG_DIR/sshpanel.env" <<EOF
LISTEN_ADDR=:8080
DATABASE_URL=postgres://${DB_USER}:${DB_PASSWORD}@127.0.0.1:5432/${DB_NAME}
JWT_SECRET=${JWT_SECRET}
ACCESS_TOKEN_TTL=15m
REFRESH_TOKEN_TTL=168h
MAX_FAILED_LOGINS=5
LOCKOUT_DURATION=15m
SSH_MANAGED_GROUP=${SSH_GROUP}
SUDO_HELPER_PATH=/usr/local/bin/sshpanel-helper
SSHD_PORT=22
SERVICE_USER=${SERVICE_USER}
STATS_POLL_INTERVAL=30s
CERT_STORAGE_DIR=/etc/sshpanel/certs
BRIDGE_LISTEN_ADDR=:8443
BRIDGE_TLS_CERT=
BRIDGE_TLS_KEY=
BRIDGE_DOMAIN=
FRONTEND_ORIGIN=
EOF
fi
chown root:"$SERVICE_USER" "$CONFIG_DIR/sshpanel.env"
chmod 0640 "$CONFIG_DIR/sshpanel.env"

# ---------------------------------------------------------------------------
log "Creating the first admin account"
# ---------------------------------------------------------------------------
if [[ "${SKIP_BOOTSTRAP_ADMIN:-}" != "1" ]]; then
    echo "You will now be prompted for the first panel administrator's username and password."
    sudo -u "$SERVICE_USER" env DATABASE_URL="postgres://${DB_USER}:${DB_PASSWORD}@127.0.0.1:5432/${DB_NAME}" \
        "$INSTALL_PREFIX/bin/bootstrap-admin"
else
    warn "SKIP_BOOTSTRAP_ADMIN=1 set - run $INSTALL_PREFIX/bin/bootstrap-admin by hand later"
fi

# ---------------------------------------------------------------------------
log "Enabling services"
# ---------------------------------------------------------------------------
systemctl enable --now sshpanel-api
systemctl enable --now sshpanel-cert-renew.timer

# nginx: only install the site config if the admin has already edited the
# placeholder domain, so we never silently serve on a domain nobody owns.
if grep -q "ssh.example.com" "$SCRIPT_DIR/nginx/sshpanel.conf"; then
    warn "deploy/nginx/sshpanel.conf still has the placeholder domain ssh.example.com - edit it, then:"
    warn "  cp $SCRIPT_DIR/nginx/sshpanel.conf /etc/nginx/sites-available/sshpanel.conf"
    warn "  ln -s /etc/nginx/sites-available/sshpanel.conf /etc/nginx/sites-enabled/"
    warn "  nginx -t && systemctl reload nginx"
else
    install -m 0644 "$SCRIPT_DIR/nginx/sshpanel.conf" /etc/nginx/sites-available/sshpanel.conf
    ln -sf /etc/nginx/sites-available/sshpanel.conf /etc/nginx/sites-enabled/sshpanel.conf
    nginx -t && systemctl reload nginx
fi

cat <<'EOF'

============================================================================
Install steps done. Next steps (these need your real domain, so they are
not automated):

  1. Point an A/AAAA record for your chosen subdomain at this server's IP.
  2. Edit deploy/nginx/sshpanel.conf, replacing ssh.example.com with that
     domain, then install it (see the warning above if this wasn't done
     automatically) and `systemctl reload nginx`.
  3. Edit /etc/sshpanel/sshpanel.env: set FRONTEND_ORIGIN to
     https://<your-domain> and BRIDGE_DOMAIN to <your-domain>.
  4. Log into the panel and issue a certificate for that domain
     (Certificates page, or POST /api/certificates), OR run once by hand:
       sudo -u sshpanel-api sudo -n /usr/local/bin/sshpanel-helper \
         cert-issue <your-domain> <your-email> sshpanel-api
  5. Set BRIDGE_TLS_CERT/BRIDGE_TLS_KEY in /etc/sshpanel/sshpanel.env to
       /etc/sshpanel/certs/<your-domain>/fullchain.pem
       /etc/sshpanel/certs/<your-domain>/privkey.pem
     then: systemctl restart sshpanel-api

The database password was generated and stored at
/etc/sshpanel/db_password (root-readable only).
============================================================================
EOF
