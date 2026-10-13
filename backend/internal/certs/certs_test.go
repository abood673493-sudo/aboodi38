package certs

import (
	"context"
	"errors"
	"testing"
)

func TestValidateDomain_AcceptsValidDomains(t *testing.T) {
	valid := []string{
		"ssh.example.com",
		"example.com",
		"a.b.c.example.co.uk",
		"ssh-01.example.com",
	}
	for _, d := range valid {
		if err := ValidateDomain(d); err != nil {
			t.Errorf("expected %q to be a valid domain, got error: %v", d, err)
		}
	}
}

func TestValidateDomain_RejectsInvalidDomains(t *testing.T) {
	invalid := []string{
		"",
		"localhost", // no dot - not a usable ACME/HTTP-01 domain
		"-leading-hyphen.example.com",
		"example.com; rm -rf /",     // shell metacharacters
		"example.com/../etc/passwd", // path traversal attempt
		"exa mple.com",              // space
		"*.example.com",             // wildcard not supported by HTTP-01
		"toolonglabelaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa.example.com",
	}
	for _, d := range invalid {
		if err := ValidateDomain(d); err == nil {
			t.Errorf("expected %q to be rejected as an invalid/unsafe domain", d)
		}
	}
}

func TestManager_CertAndKeyPaths(t *testing.T) {
	m := NewManager(&fakeIssuer{}, "/etc/sshpanel/certs")
	if got, want := m.CertPath("ssh.example.com"), "/etc/sshpanel/certs/ssh.example.com/fullchain.pem"; got != want {
		t.Errorf("CertPath = %q, want %q", got, want)
	}
	if got, want := m.KeyPath("ssh.example.com"), "/etc/sshpanel/certs/ssh.example.com/privkey.pem"; got != want {
		t.Errorf("KeyPath = %q, want %q", got, want)
	}
}

// fakeIssuer stands in for *sshmgr.Manager so Manager's own logic (domain
// validation happens before any privileged call is attempted) can be tested
// without touching a real system.
type fakeIssuer struct {
	issuedDomain, issuedEmail string
	issueErr                  error
	renewCalled               bool
	renewErr                  error
}

func (f *fakeIssuer) IssueCertificate(_ context.Context, domain, email string) error {
	f.issuedDomain, f.issuedEmail = domain, email
	return f.issueErr
}

func (f *fakeIssuer) RenewCertificates(_ context.Context) error {
	f.renewCalled = true
	return f.renewErr
}

func TestManager_Issue_RejectsInvalidDomainBeforeCallingOS(t *testing.T) {
	issuer := &fakeIssuer{}
	m := NewManager(issuer, "/etc/sshpanel/certs")

	err := m.Issue(context.Background(), "not a domain; rm -rf /", "admin@example.com")
	if err == nil {
		t.Fatal("expected invalid domain to be rejected")
	}
	if issuer.issuedDomain != "" {
		t.Fatal("the privileged issuer must never be called for an invalid domain")
	}
}

func TestManager_Issue_DelegatesToOSForValidDomain(t *testing.T) {
	issuer := &fakeIssuer{}
	m := NewManager(issuer, "/etc/sshpanel/certs")

	if err := m.Issue(context.Background(), "ssh.example.com", "admin@example.com"); err != nil {
		t.Fatalf("expected success, got %v", err)
	}
	if issuer.issuedDomain != "ssh.example.com" || issuer.issuedEmail != "admin@example.com" {
		t.Fatalf("expected the OS issuer to be called with the validated domain/email, got %q/%q", issuer.issuedDomain, issuer.issuedEmail)
	}
}

func TestManager_Issue_PropagatesOSFailure(t *testing.T) {
	issuer := &fakeIssuer{issueErr: errors.New("certbot: challenge failed")}
	m := NewManager(issuer, "/etc/sshpanel/certs")

	err := m.Issue(context.Background(), "ssh.example.com", "")
	if err == nil {
		t.Fatal("expected the OS issuer's failure to propagate")
	}
}

func TestManager_RenewAll_DelegatesToOS(t *testing.T) {
	issuer := &fakeIssuer{}
	m := NewManager(issuer, "/etc/sshpanel/certs")

	if err := m.RenewAll(context.Background()); err != nil {
		t.Fatalf("expected success, got %v", err)
	}
	if !issuer.renewCalled {
		t.Fatal("expected RenewCertificates to have been called")
	}
}
