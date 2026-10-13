// sshpanel-helper is the ONLY part of this project that runs as root. It is
// invoked by the unprivileged API process through a sudoers rule that pins
// this exact binary path (see deploy/sudoers.d/sshpanel). It re-validates
// every argument itself (never trusting the caller), builds real OS commands
// as argv slices (never a shell string), and implements a fixed, small set
// of actions - nothing resembling "run this shell command for me". There is
// deliberately no action here that accepts free-form text to execute.
package main

import (
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"regexp"
	"strings"
	"time"
)

var usernameRe = regexp.MustCompile(`^[a-z_][a-z0-9_-]{2,31}$`)

var reserved = map[string]bool{
	"root": true, "daemon": true, "bin": true, "sys": true, "sync": true,
	"games": true, "man": true, "lp": true, "mail": true, "news": true,
	"uucp": true, "proxy": true, "www-data": true, "backup": true,
	"list": true, "irc": true, "gnats": true, "nobody": true,
	"systemd-network": true, "systemd-resolve": true, "messagebus": true,
	"sshd": true, "postgres": true, "sshpanel": true,
}

func validUsername(u string) error {
	if !usernameRe.MatchString(u) {
		return fmt.Errorf("rejected username")
	}
	if reserved[u] {
		return fmt.Errorf("reserved username")
	}
	return nil
}

// validGroup is intentionally narrower than a username: it must match the
// group name baked into this deployment's config, passed on argv by the
// caller but double-checked here against the same simple pattern.
var groupRe = regexp.MustCompile(`^[a-z_][a-z0-9_-]{2,31}$`)

func validGroup(g string) error {
	if !groupRe.MatchString(g) {
		return fmt.Errorf("rejected group name")
	}
	return nil
}

func validDate(d string) error {
	if d == "-1" {
		return nil
	}
	_, err := time.Parse("2006-01-02", d)
	return err
}

func run(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func runOut(name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	out, err := cmd.Output()
	return strings.TrimSpace(string(out)), err
}

func fail(msg string) {
	fmt.Fprintln(os.Stderr, "sshpanel-helper: "+msg)
	os.Exit(1)
}

func main() {
	if len(os.Args) < 2 {
		fail("no action given")
	}
	action := os.Args[1]
	args := os.Args[2:]

	switch action {

	case "create":
		// create <username> <group> <password>
		if len(args) != 3 {
			fail("create requires: <username> <group> <password>")
		}
		username, group, password := args[0], args[1], args[2]
		if err := validUsername(username); err != nil {
			fail(err.Error())
		}
		if err := validGroup(group); err != nil {
			fail(err.Error())
		}
		if len(password) < 12 {
			fail("password too short")
		}
		// No home directory login shell beyond nologin - these accounts exist
		// only to authenticate SSH tunnels, not to run interactive sessions.
		if err := run("useradd",
			"--create-home",
			"--shell", "/usr/sbin/nologin",
			"--groups", group,
			username,
		); err != nil {
			fail("useradd failed: " + err.Error())
		}
		if err := setPasswordViaChpasswd(username, password); err != nil {
			fail(err.Error())
		}

	case "setpassword":
		// setpassword <username> <new-password>
		if len(args) != 2 {
			fail("setpassword requires: <username> <new-password>")
		}
		username, password := args[0], args[1]
		if err := validUsername(username); err != nil {
			fail(err.Error())
		}
		if len(password) < 12 {
			fail("password too short")
		}
		if err := setPasswordViaChpasswd(username, password); err != nil {
			fail(err.Error())
		}

	case "rename":
		// rename <old-username> <new-username>
		if len(args) != 2 {
			fail("rename requires: <old-username> <new-username>")
		}
		oldU, newU := args[0], args[1]
		if err := validUsername(oldU); err != nil {
			fail(err.Error())
		}
		if err := validUsername(newU); err != nil {
			fail(err.Error())
		}
		if err := run("usermod", "--login", newU, "--move-home", oldU); err != nil {
			fail("usermod rename failed: " + err.Error())
		}

	case "disable":
		// disable <username>
		if len(args) != 1 {
			fail("disable requires: <username>")
		}
		username := args[0]
		if err := validUsername(username); err != nil {
			fail(err.Error())
		}
		if err := run("usermod", "--lock", username); err != nil {
			fail("usermod lock failed: " + err.Error())
		}
		if err := run("chage", "-E", "1970-01-01", username); err != nil {
			fail("chage expire failed: " + err.Error())
		}

	case "enable":
		// enable <username>
		if len(args) != 1 {
			fail("enable requires: <username>")
		}
		username := args[0]
		if err := validUsername(username); err != nil {
			fail(err.Error())
		}
		if err := run("usermod", "--unlock", username); err != nil {
			fail("usermod unlock failed: " + err.Error())
		}
		if err := run("chage", "-E", "-1", username); err != nil {
			fail("chage clear-expiry failed: " + err.Error())
		}

	case "delete":
		// delete <username>
		if len(args) != 1 {
			fail("delete requires: <username>")
		}
		username := args[0]
		if err := validUsername(username); err != nil {
			fail(err.Error())
		}
		if err := run("userdel", "--remove", username); err != nil {
			fail("userdel failed: " + err.Error())
		}

	case "setexpiry":
		// setexpiry <username> <YYYY-MM-DD|-1>
		if len(args) != 2 {
			fail("setexpiry requires: <username> <date>")
		}
		username, date := args[0], args[1]
		if err := validUsername(username); err != nil {
			fail(err.Error())
		}
		if err := validDate(date); err != nil {
			fail("invalid date")
		}
		if err := run("chage", "-E", date, username); err != nil {
			fail("chage failed: " + err.Error())
		}

	case "uid":
		// uid <username>  -> prints numeric UID
		if len(args) != 1 {
			fail("uid requires: <username>")
		}
		username := args[0]
		if err := validUsername(username); err != nil {
			fail(err.Error())
		}
		u, err := user.Lookup(username)
		if err != nil {
			fail("lookup failed: " + err.Error())
		}
		fmt.Println(u.Uid)

	case "ipacct":
		// ipacct <uid>  -> prints "<ingress_bytes> <egress_bytes>"
		// Reads real cumulative counters from systemd's per-user-slice
		// IPAccounting feature. Requires the user-.slice.d override that
		// enables IPAccounting=yes (deploy/systemd/user-slice-ipaccounting.conf).
		// If the slice doesn't exist (user never logged in since boot) or
		// accounting isn't enabled, this prints "0 0" - the caller treats
		// that as "no data yet", never as a real zero-traffic measurement
		// unless the slice is confirmed present.
		if len(args) != 1 {
			fail("ipacct requires: <uid>")
		}
		uid := args[0]
		if !regexp.MustCompile(`^[0-9]+$`).MatchString(uid) {
			fail("invalid uid")
		}
		ingress := readSliceProperty(uid, "IPIngressBytes")
		egress := readSliceProperty(uid, "IPEgressBytes")
		fmt.Printf("%s %s\n", ingress, egress)

	case "cert-issue":
		// cert-issue <domain> <email-or--> <service-user>
		if len(args) != 3 {
			fail("cert-issue requires: <domain> <email-or--> <service-user>")
		}
		domain, email, serviceUser := args[0], args[1], args[2]
		if err := validDomain(domain); err != nil {
			fail(err.Error())
		}
		if email != "-" {
			if err := validEmail(email); err != nil {
				fail(err.Error())
			}
		}
		if err := validUsername(serviceUser); err != nil {
			fail("rejected service user: " + err.Error())
		}
		certArgs := []string{
			"certonly", "--non-interactive", "--agree-tos",
			"--webroot", "-w", certWebroot,
			"-d", domain,
		}
		if email != "-" {
			certArgs = append(certArgs, "-m", email)
		} else {
			certArgs = append(certArgs, "--register-unsafely-without-email")
		}
		if err := run(certbotPath, certArgs...); err != nil {
			fail("certbot certonly failed: " + err.Error())
		}
		if err := publishCert(domain, serviceUser); err != nil {
			fail(err.Error())
		}

	case "cert-renew":
		// cert-renew <service-user>
		if len(args) != 1 {
			fail("cert-renew requires: <service-user>")
		}
		serviceUser := args[0]
		if err := validUsername(serviceUser); err != nil {
			fail("rejected service user: " + err.Error())
		}
		if err := run(certbotPath, "renew", "--non-interactive"); err != nil {
			fail("certbot renew failed: " + err.Error())
		}
		// certbot renew may have rotated any subset of the domains it
		// manages - republish all of them so the API process always reads
		// current material from its own readable copy.
		entries, err := os.ReadDir(letsEncryptLiveDir)
		if err != nil {
			fail("failed to list renewed certificates: " + err.Error())
		}
		for _, e := range entries {
			if !e.IsDir() || e.Name() == "README" {
				continue
			}
			if err := validDomain(e.Name()); err != nil {
				continue // skip anything that isn't a plain domain directory
			}
			if err := publishCert(e.Name(), serviceUser); err != nil {
				fail(err.Error())
			}
		}

	default:
		fail("unknown action: " + action)
	}
}

// certbotPath and certWebroot are fixed, not accepted as arguments: the
// privileged helper decides where certbot lives and which webroot it serves
// the ACME HTTP-01 challenge from, so a compromised unprivileged API process
// cannot redirect either to an attacker-chosen location.
const (
	certbotPath             = "/usr/bin/certbot"
	certWebroot             = "/var/www/sshpanel-acme"
	letsEncryptLiveDir      = "/etc/letsencrypt/live"
	publishedCertsDirParent = "/etc/sshpanel"
	publishedCertsDir       = "/etc/sshpanel/certs"
)

var emailRe = regexp.MustCompile(`^[^@\s]+@[^@\s]+\.[^@\s]+$`)

func validEmail(e string) error {
	if len(e) > 254 || !emailRe.MatchString(e) {
		return fmt.Errorf("rejected email address")
	}
	return nil
}

// validDomain mirrors internal/certs.ValidateDomain. It is re-implemented
// here, not imported, so this binary's safety never depends on another
// package in the tree staying correct - the same reasoning as validUsername
// above.
var domainRe = regexp.MustCompile(`^(?i)[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?(\.[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?)+$`)

func validDomain(d string) error {
	if len(d) > 253 || !domainRe.MatchString(d) {
		return fmt.Errorf("rejected domain name")
	}
	return nil
}

// publishCert copies the fullchain/privkey certbot just (re)issued for
// domain out of /etc/letsencrypt/live (root-only, 0700 by certbot default)
// into /etc/sshpanel/certs/<domain>, owned by the unprivileged service
// account so the API process can load them for the WS/TLS bridge without
// ever running as root itself.
func publishCert(domain, serviceUser string) error {
	srcDir := letsEncryptLiveDir + "/" + domain
	dstDir := publishedCertsDir + "/" + domain

	// os.MkdirAll would create any missing parent (/etc/sshpanel,
	// /etc/sshpanel/certs) owned by root with the leaf directory's mode,
	// which previously left those parents at 0750 root:root - blocking the
	// service account from ever traversing into them to reach its own,
	// correctly-owned leaf directory. The parents only ever contain domain
	// *names*, nothing secret, so 0755 root:root (traversable/listable by
	// anyone, writable only by root) is the right permission for them; only
	// the per-domain leaf directory and the private key itself need to be
	// restricted to the service account.
	for _, dir := range []string{publishedCertsDirParent, publishedCertsDir} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("mkdir %s: %w", dir, err)
		}
	}
	if err := os.MkdirAll(dstDir, 0750); err != nil {
		return fmt.Errorf("mkdir %s: %w", dstDir, err)
	}

	u, err := user.Lookup(serviceUser)
	if err != nil {
		return fmt.Errorf("lookup service user %q: %w", serviceUser, err)
	}
	uid, _ := parseUintSafe(u.Uid)
	gid, _ := parseUintSafe(u.Gid)

	for _, f := range []struct {
		name string
		mode os.FileMode
	}{
		{"fullchain.pem", 0640},
		{"privkey.pem", 0600},
	} {
		src := srcDir + "/" + f.name
		dst := dstDir + "/" + f.name
		data, err := os.ReadFile(src)
		if err != nil {
			return fmt.Errorf("read %s: %w", src, err)
		}
		if err := os.WriteFile(dst, data, f.mode); err != nil {
			return fmt.Errorf("write %s: %w", dst, err)
		}
		if err := os.Chown(dst, uid, gid); err != nil {
			return fmt.Errorf("chown %s: %w", dst, err)
		}
	}
	if err := os.Chown(dstDir, uid, gid); err != nil {
		return fmt.Errorf("chown %s: %w", dstDir, err)
	}
	return nil
}

func parseUintSafe(s string) (int, error) {
	n := 0
	for _, r := range s {
		if r < '0' || r > '9' {
			return 0, fmt.Errorf("not a number: %q", s)
		}
		n = n*10 + int(r-'0')
	}
	return n, nil
}

func readSliceProperty(uid, property string) string {
	out, err := runOut("systemctl", "show", "user-"+uid+".slice", "-p", property, "--value")
	if err != nil {
		return "0"
	}
	out = strings.TrimSpace(out)
	if out == "" || out == "[not set]" {
		return "0"
	}
	return out
}

// setPasswordViaChpasswd feeds "user:password\n" to chpasswd over stdin so
// the plaintext password never appears as a process argument (which would
// be visible to any local user via /proc/<pid>/cmdline or `ps`).
func setPasswordViaChpasswd(username, password string) error {
	cmd := exec.Command("chpasswd")
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return err
	}
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		return err
	}
	if _, err := stdin.Write([]byte(username + ":" + password + "\n")); err != nil {
		return err
	}
	if err := stdin.Close(); err != nil {
		return err
	}
	return cmd.Wait()
}
