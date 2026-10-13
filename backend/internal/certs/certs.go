// Package certs manages TLS certificate metadata for the panel's
// subdomain(s). It validates domain names and exposes where issued
// certificate files live, but it never shells out to certbot itself - the
// actual privileged ACME issuance/renewal is delegated to an OSCertIssuer
// (backed by internal/sshmgr, which goes through the same narrowly-scoped
// root helper as account management) so this package stays free of any
// direct OS command execution.
package certs

import (
	"context"
	"fmt"
	"regexp"
)

var domainRe = regexp.MustCompile(`^(?i)[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?(\.[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?)+$`)

func ValidateDomain(d string) error {
	if len(d) > 253 || !domainRe.MatchString(d) {
		return fmt.Errorf("invalid domain name %q", d)
	}
	return nil
}

// OSCertIssuer is the privileged operation boundary, implemented by
// *sshmgr.Manager in production and by a fake in tests - the same pattern
// handlers.AccountService uses for sshmgr.Manager's account operations.
type OSCertIssuer interface {
	IssueCertificate(ctx context.Context, domain, email string) error
	RenewCertificates(ctx context.Context) error
}

type Manager struct {
	os         OSCertIssuer
	storageDir string // where the API process reads published cert/key files from, e.g. /etc/sshpanel/certs
}

func NewManager(os OSCertIssuer, storageDir string) *Manager {
	return &Manager{os: os, storageDir: storageDir}
}

// Issue requests (or renews, since certbot is idempotent for a domain it
// already manages) a certificate using the HTTP-01 webroot challenge. This
// requires the subdomain to already point at this server (A/AAAA record)
// and port 80 to be reachable - the panel does not and cannot alter DNS
// records itself, since that is outside any single server's control; the
// admin is expected to have created the subdomain's DNS record already.
func (m *Manager) Issue(ctx context.Context, domain, email string) error {
	if err := ValidateDomain(domain); err != nil {
		return err
	}
	return m.os.IssueCertificate(ctx, domain, email)
}

// RenewAll triggers certbot's own renewal logic for all managed certs (it
// only actually renews ones within its configured renewal window, typically
// the last 30 days of validity) - intended to be called from a daily
// systemd timer, documented in deploy/systemd.
func (m *Manager) RenewAll(ctx context.Context) error {
	return m.os.RenewCertificates(ctx)
}

// CertPath and KeyPath point at this deployment's *published* copy of the
// certificate (see sshpanel-helper's publishCert), not certbot's own
// /etc/letsencrypt/live, which is root-only and unreadable by the
// unprivileged API process that needs these files to serve the WS/TLS
// bridge.
func (m *Manager) CertPath(domain string) string {
	return m.storageDir + "/" + domain + "/fullchain.pem"
}

func (m *Manager) KeyPath(domain string) string {
	return m.storageDir + "/" + domain + "/privkey.pem"
}
