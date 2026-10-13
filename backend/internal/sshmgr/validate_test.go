package sshmgr

import "testing"

func TestValidateUsername(t *testing.T) {
	valid := []string{"alice", "bob_2026", "user-01", "a23"}
	for _, u := range valid {
		if err := ValidateUsername(u); err != nil {
			t.Errorf("expected %q to be valid, got error: %v", u, err)
		}
	}

	invalid := []string{
		"root",             // reserved
		"Alice",            // uppercase not allowed
		"a",                // too short
		"user;rm -rf /",    // shell metacharacters
		"user name",        // space
		"../../etc/passwd", // path traversal attempt
		"user$(whoami)",    // command substitution attempt
		"",
		"this-username-is-way-too-long-to-be-a-valid-linux-account-name",
	}
	for _, u := range invalid {
		if err := ValidateUsername(u); err == nil {
			t.Errorf("expected %q to be rejected, got nil error", u)
		}
	}
}
