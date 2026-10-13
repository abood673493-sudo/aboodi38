package stats

import "testing"

func TestParsePSOutputForSSHSessions(t *testing.T) {
	// A realistic `ps -eo user=,args=` snippet: a privileged sshd listener,
	// one privsep monitor per connection (still root), and the post-auth
	// child for each session (dropped to the real user). Only the latter
	// should be counted, and two sessions for the same user should sum.
	psOutput := `root       sshd: /usr/sbin/sshd -D [listener] 0 of 10-100 startups
root       sshd: alice [priv]
alice      sshd: alice@pts/0
root       sshd: alice [priv]
alice      sshd: alice@notty
root       sshd: bob [priv]
bob        sshd: bob@pts/1
www-data   nginx: worker process
postgres   postgres: sshpanel sshpanel 127.0.0.1(53412) idle
`

	counts := parsePSOutputForSSHSessions(psOutput)

	if counts["alice"] != 2 {
		t.Errorf("expected alice to have 2 active sessions, got %d", counts["alice"])
	}
	if counts["bob"] != 1 {
		t.Errorf("expected bob to have 1 active session, got %d", counts["bob"])
	}
	if _, ok := counts["root"]; ok {
		t.Error("root (privileged sshd monitor processes) must never be counted as a session")
	}
	if _, ok := counts["www-data"]; ok {
		t.Error("non-sshd processes must not be counted")
	}
	if len(counts) != 2 {
		t.Errorf("expected exactly 2 distinct users with sessions, got %d: %v", len(counts), counts)
	}
}

func TestParsePSOutputForSSHSessions_Empty(t *testing.T) {
	counts := parsePSOutputForSSHSessions("")
	if len(counts) != 0 {
		t.Errorf("expected no sessions from empty ps output, got %v", counts)
	}
}

func TestParsePSOutputForSSHSessions_NoSSHDProcesses(t *testing.T) {
	psOutput := `root     init\nwww-data nginx: master process\n`
	counts := parsePSOutputForSSHSessions(psOutput)
	if len(counts) != 0 {
		t.Errorf("expected no sessions when no sshd processes present, got %v", counts)
	}
}
