// Package sshmgr creates/edits/removes real Linux accounts that are allowed
// to authenticate over SSH. It never builds a shell string: every external
// command is invoked with a fixed argv, and every value that reaches it is
// validated against a strict allow-list pattern first. The actual
// privileged operations are not performed by the API process itself (which
// should run as an unprivileged user) but delegated to a tiny, separately
// reviewed helper binary invoked through a narrow sudoers rule - see
// cmd/sshpanel-helper and deploy/sudoers.d/sshpanel.
package sshmgr

import (
	"fmt"
	"regexp"
)

// usernameRe mirrors useradd's own default constraint (POSIX portable
// username) plus a length cap, and matches the CHECK constraint in
// migrations/0001_init.sql so the DB and the OS can never disagree about
// what a valid username looks like.
var usernameRe = regexp.MustCompile(`^[a-z_][a-z0-9_-]{2,31}$`)

func ValidateUsername(u string) error {
	if !usernameRe.MatchString(u) {
		return fmt.Errorf("invalid username %q: must match %s", u, usernameRe.String())
	}
	for _, reserved := range reservedUsernames {
		if u == reserved {
			return fmt.Errorf("username %q is reserved", u)
		}
	}
	return nil
}

// reservedUsernames blocks anything that would collide with a system
// account or be confusing/dangerous to create via the panel.
var reservedUsernames = []string{
	"root", "daemon", "bin", "sys", "sync", "games", "man", "lp", "mail",
	"news", "uucp", "proxy", "www-data", "backup", "list", "irc", "gnats",
	"nobody", "systemd-network", "systemd-resolve", "messagebus", "sshd",
	"postgres", "sshpanel",
}

// domainRe mirrors internal/certs.ValidateDomain's pattern. It is duplicated
// (not imported) for the same reason sshpanel-helper keeps its own copy:
// this package's safety guarantees must not depend on another package in
// the tree staying correct.
var domainRe = regexp.MustCompile(`^(?i)[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?(\.[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?)+$`)

// ValidateDomainName is the sshmgr-layer check applied before a domain
// reaches the privileged helper for certificate issuance/renewal.
func ValidateDomainName(d string) error {
	if len(d) > 253 || !domainRe.MatchString(d) {
		return fmt.Errorf("invalid domain name %q", d)
	}
	return nil
}
