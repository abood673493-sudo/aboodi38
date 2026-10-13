package stats

import (
	"os/exec"
	"strings"
)

// ActiveSSHSessionCounts counts real logged-in SSH sessions per Linux
// username by inspecting the privilege-separated sshd worker processes
// (`sshd: <user>@pts/N` or `sshd: <user> [priv]`), which sshd itself names
// after a successful authentication. This reflects actual open sessions -
// it is not derived from anything the panel itself tracks, so a session
// opened directly at the OS level (not through the bridge) is still counted.
func ActiveSSHSessionCounts() (map[string]int, error) {
	out, err := exec.Command("ps", "-eo", "user=,args=").Output()
	if err != nil {
		return nil, err
	}
	return parsePSOutputForSSHSessions(string(out)), nil
}

// parsePSOutputForSSHSessions is the pure parsing half of
// ActiveSSHSessionCounts, split out so the counting logic (which lines count,
// which are skipped) can be unit tested without a real process table.
func parsePSOutputForSSHSessions(psOutput string) map[string]int {
	counts := map[string]int{}
	for _, line := range strings.Split(psOutput, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		fields := strings.SplitN(line, " ", 2)
		if len(fields) != 2 {
			continue
		}
		procUser, args := fields[0], fields[1]
		args = strings.TrimSpace(args)
		if !strings.HasPrefix(args, "sshd:") {
			continue
		}
		// sshd's privilege-separation model spawns a root-owned monitor
		// process ("sshd: alice [priv]") AND a per-session child that drops
		// to the authenticated user's UID ("sshd: alice@pts/0" or
		// "sshd: alice@notty" for a non-interactive tunnel). Only the
		// latter is counted, so one established session isn't counted
		// twice via its own privileged parent.
		if procUser == "root" {
			continue
		}
		counts[procUser]++
	}
	return counts
}
