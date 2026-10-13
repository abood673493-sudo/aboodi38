package sshmgr

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// Manager is the only part of the backend allowed to touch real OS accounts.
// It talks to the privileged helper binary through `sudo`, passing a fixed
// argv (never a shell string), so there is no way for a request body to
// inject an arbitrary command. The sudoers rule for sshHelperPath must pin
// the exact binary path and allow it to run as root with NOPASSWD, and
// nothing else.
type Manager struct {
	sudoHelperPath string
	sshGroup       string
	serviceUser    string // the unprivileged account the API process itself runs as
	runTimeout     time.Duration
}

func NewManager(sudoHelperPath, sshGroup, serviceUser string) *Manager {
	return &Manager{sudoHelperPath: sudoHelperPath, sshGroup: sshGroup, serviceUser: serviceUser, runTimeout: 10 * time.Second}
}

func (m *Manager) run(ctx context.Context, args ...string) (string, error) {
	return m.runWithTimeout(ctx, m.runTimeout, args...)
}

func (m *Manager) runWithTimeout(ctx context.Context, timeout time.Duration, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// argv is built as a Go slice throughout - exec.CommandContext never
	// passes through /bin/sh, so shell metacharacters in any value (quotes,
	// `;`, `$()`, backticks) are inert.
	fullArgs := append([]string{m.sudoHelperPath}, args...)
	cmd := exec.CommandContext(ctx, "sudo", fullArgs...)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("helper command failed: %w (stderr: %s)", err, strings.TrimSpace(stderr.String()))
	}
	return strings.TrimSpace(stdout.String()), nil
}

// CreateAccount provisions a real Linux user: no login shell beyond SSH
// tunneling, added to the restricted sshpanel group that sshd_config scopes
// down (see deploy/sshd/99-sshpanel.conf), home directory disabled, and an
// immediate password set via chpasswd (never passed as a bare argv to
// `passwd`, which would leak it through /proc/<pid>/cmdline).
func (m *Manager) CreateAccount(ctx context.Context, username, password string) error {
	if err := ValidateUsername(username); err != nil {
		return err
	}
	if password == "" {
		return fmt.Errorf("password must not be empty")
	}
	_, err := m.run(ctx, "create", username, m.sshGroup, password)
	return err
}

func (m *Manager) SetPassword(ctx context.Context, username, newPassword string) error {
	if err := ValidateUsername(username); err != nil {
		return err
	}
	if newPassword == "" {
		return fmt.Errorf("password must not be empty")
	}
	_, err := m.run(ctx, "setpassword", username, newPassword)
	return err
}

func (m *Manager) RenameAccount(ctx context.Context, oldUsername, newUsername string) error {
	if err := ValidateUsername(oldUsername); err != nil {
		return err
	}
	if err := ValidateUsername(newUsername); err != nil {
		return err
	}
	_, err := m.run(ctx, "rename", oldUsername, newUsername)
	return err
}

// DisableAccount locks the password (no login via password/kbd-interactive)
// and expires the account immediately (chage -E 1970-01-01), which also
// blocks key-based login once sshd's AccountExpired check runs - it does not
// delete anything so it is fully reversible via EnableAccount.
func (m *Manager) DisableAccount(ctx context.Context, username string) error {
	if err := ValidateUsername(username); err != nil {
		return err
	}
	_, err := m.run(ctx, "disable", username)
	return err
}

func (m *Manager) EnableAccount(ctx context.Context, username string) error {
	if err := ValidateUsername(username); err != nil {
		return err
	}
	_, err := m.run(ctx, "enable", username)
	return err
}

// DeleteAccount removes the Linux user and its home directory. Irreversible.
func (m *Manager) DeleteAccount(ctx context.Context, username string) error {
	if err := ValidateUsername(username); err != nil {
		return err
	}
	_, err := m.run(ctx, "delete", username)
	return err
}

// SetExpiration sets (or clears, if expiresAt is nil) the account's OS-level
// expiration date via `chage -E`, which is enforced by the OS/sshd itself -
// not only by the panel's own cron reaper - so an expired account cannot log
// in even if the panel's background job hasn't run yet.
func (m *Manager) SetExpiration(ctx context.Context, username string, expiresAt *time.Time) error {
	if err := ValidateUsername(username); err != nil {
		return err
	}
	val := "-1" // chage -E -1 clears expiration
	if expiresAt != nil {
		val = expiresAt.UTC().Format("2006-01-02")
	}
	_, err := m.run(ctx, "setexpiry", username, val)
	return err
}

// IPAccounting returns the real cumulative ingress/egress byte counters for
// an account, sourced from systemd's per-user-slice IPAccounting feature
// (`systemctl show user-<uid>.slice`). This requires the deploy-time
// `user-.slice.d/override.conf` drop-in (see deploy/systemd) to have
// enabled IPAccounting=yes for user slices; if that isn't present the
// helper returns zeros rather than a fabricated figure, and the caller
// should treat zero-with-no-samples as "not available" rather than "no
// traffic".
func (m *Manager) IPAccounting(ctx context.Context, uid int) (ingressBytes, egressBytes uint64, err error) {
	out, err := m.run(ctx, "ipacct", strconv.Itoa(uid))
	if err != nil {
		return 0, 0, err
	}
	parts := strings.Fields(out)
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("unexpected ipacct output: %q", out)
	}
	in, err1 := strconv.ParseUint(parts[0], 10, 64)
	out2, err2 := strconv.ParseUint(parts[1], 10, 64)
	if err1 != nil || err2 != nil {
		return 0, 0, fmt.Errorf("unexpected ipacct values: %q", out)
	}
	return in, out2, nil
}

// certRunTimeout is longer than the default OS-account timeout because
// certbot talks to a real ACME server over the network (HTTP-01 challenge
// validation, rate-limit checks) rather than doing a local syscall.
const certRunTimeout = 2 * time.Minute

// IssueCertificate requests (or idempotently re-requests) a certificate for
// domain via the privileged helper, which validates the domain again itself,
// runs certbot with a fixed argv, and republishes the result to a location
// this (unprivileged) process can read. email may be empty for
// --register-unsafely-without-email.
func (m *Manager) IssueCertificate(ctx context.Context, domain, email string) error {
	if err := ValidateDomainName(domain); err != nil {
		return err
	}
	emailArg := email
	if emailArg == "" {
		emailArg = "-"
	}
	_, err := m.runWithTimeout(ctx, certRunTimeout, "cert-issue", domain, emailArg, m.serviceUser)
	return err
}

// RenewCertificates triggers certbot's own renewal pass for every domain it
// manages (only ones within its renewal window actually renew) and
// republishes whatever changed.
func (m *Manager) RenewCertificates(ctx context.Context) error {
	_, err := m.runWithTimeout(ctx, certRunTimeout, "cert-renew", m.serviceUser)
	return err
}

// LookupUID resolves the numeric UID for an account that was just created,
// which the stats poller needs to attribute real iptables/nftables
// per-UID byte counters back to the right account.
func (m *Manager) LookupUID(ctx context.Context, username string) (int, error) {
	if err := ValidateUsername(username); err != nil {
		return 0, err
	}
	out, err := m.run(ctx, "uid", username)
	if err != nil {
		return 0, err
	}
	uid, err := strconv.Atoi(out)
	if err != nil {
		return 0, fmt.Errorf("unexpected uid output %q: %w", out, err)
	}
	return uid, nil
}
