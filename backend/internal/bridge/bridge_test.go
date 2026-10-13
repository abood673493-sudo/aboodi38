package bridge

import "testing"

func TestHostAllowed_NoConfiguredHostAllowsAnything(t *testing.T) {
	if !hostAllowed("", "anything.example.com") {
		t.Error("an empty configured host should disable the check")
	}
	if !hostAllowed("", "") {
		t.Error("an empty configured host should disable the check even for an empty request host")
	}
}

func TestHostAllowed_MatchesConfiguredDomain(t *testing.T) {
	if !hostAllowed("ssh.example.com", "ssh.example.com") {
		t.Error("expected an exact host match to be allowed")
	}
}

func TestHostAllowed_MatchIsCaseInsensitiveAndIgnoresPort(t *testing.T) {
	if !hostAllowed("ssh.example.com", "SSH.Example.com:8443") {
		t.Error("expected host matching to be case-insensitive and ignore the port")
	}
}

func TestHostAllowed_RejectsUnrelatedHost(t *testing.T) {
	if hostAllowed("ssh.example.com", "attacker.example.net") {
		t.Error("expected a request for an unrelated host to be rejected once a domain is configured")
	}
}

func TestHostAllowed_RejectsEmptyRequestHostWhenConfigured(t *testing.T) {
	if hostAllowed("ssh.example.com", "") {
		t.Error("expected a request with no Host to be rejected once a domain is configured")
	}
}

func TestNormalizeHost(t *testing.T) {
	cases := map[string]string{
		"Ssh.Example.com":      "ssh.example.com",
		"ssh.example.com:8443": "ssh.example.com",
		"  ssh.example.com  ":  "ssh.example.com",
	}
	for in, want := range cases {
		if got := normalizeHost(in); got != want {
			t.Errorf("normalizeHost(%q) = %q, want %q", in, got, want)
		}
	}
}
