// Package stats collects real host metrics (load, memory, disk), real
// active-connection counts (parsed from `ss`), and real per-account
// bandwidth (from nftables/iptables per-UID counters). Nothing here is
// simulated or randomly generated - if a real source isn't available on the
// host, the corresponding field is left at zero/empty rather than faked.
package stats

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"

	"ssh-manager-panel/internal/models"
)

func Hostname() string {
	h, err := os.Hostname()
	if err != nil {
		return "unknown"
	}
	return h
}

// LoadAvg reads /proc/loadavg directly - this is real kernel-reported data,
// available on every Linux host without extra tooling.
func LoadAvg() (one, five, fifteen float64, err error) {
	f, err := os.Open("/proc/loadavg")
	if err != nil {
		return 0, 0, 0, err
	}
	defer f.Close()

	var line string
	scanner := bufio.NewScanner(f)
	if scanner.Scan() {
		line = scanner.Text()
	}
	fields := strings.Fields(line)
	if len(fields) < 3 {
		return 0, 0, 0, fmt.Errorf("unexpected /proc/loadavg format: %q", line)
	}
	one, _ = strconv.ParseFloat(fields[0], 64)
	five, _ = strconv.ParseFloat(fields[1], 64)
	fifteen, _ = strconv.ParseFloat(fields[2], 64)
	return one, five, fifteen, nil
}

// MemInfo reads real values from /proc/meminfo (kB entries converted to
// bytes). MemUsed is derived as Total - Available (the same definition
// `free -h` uses), not Total - Free, which would overcount usage because it
// ignores reclaimable cache.
func MemInfo() (totalBytes, usedBytes uint64, err error) {
	f, err := os.Open("/proc/meminfo")
	if err != nil {
		return 0, 0, err
	}
	defer f.Close()

	values := map[string]uint64{}
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.Fields(line)
		if len(parts) < 2 {
			continue
		}
		key := strings.TrimSuffix(parts[0], ":")
		n, convErr := strconv.ParseUint(parts[1], 10, 64)
		if convErr != nil {
			continue
		}
		values[key] = n * 1024 // kB -> bytes
	}

	total, ok := values["MemTotal"]
	if !ok {
		return 0, 0, fmt.Errorf("MemTotal not found in /proc/meminfo")
	}
	avail, ok := values["MemAvailable"]
	if !ok {
		avail = values["MemFree"]
	}
	used := uint64(0)
	if total > avail {
		used = total - avail
	}
	return total, used, nil
}

// DiskUsage reports real usage for the given path via statfs(2) - no
// estimation.
func DiskUsage(path string) (totalBytes, usedBytes uint64, err error) {
	var st syscall.Statfs_t
	if err := syscall.Statfs(path, &st); err != nil {
		return 0, 0, err
	}
	total := st.Blocks * uint64(st.Bsize)
	free := st.Bfree * uint64(st.Bsize)
	return total, total - free, nil
}

// UptimeSeconds reads the real kernel uptime counter.
func UptimeSeconds() (int64, error) {
	f, err := os.Open("/proc/uptime")
	if err != nil {
		return 0, err
	}
	defer f.Close()
	var up float64
	if _, err := fmt.Fscanf(f, "%f", &up); err != nil {
		return 0, err
	}
	return int64(up), nil
}

// Collect assembles a full ServerStatus from real host sources. Account
// counts are filled in by the caller (handlers layer) from the database,
// since this package has no DB dependency.
func Collect() (*models.ServerStatus, error) {
	st := &models.ServerStatus{Hostname: Hostname()}

	if up, err := UptimeSeconds(); err == nil {
		st.UptimeSeconds = up
	}
	if l1, l5, l15, err := LoadAvg(); err == nil {
		st.LoadAvg1, st.LoadAvg5, st.LoadAvg15 = l1, l5, l15
	}
	if total, used, err := MemInfo(); err == nil {
		st.MemTotalBytes, st.MemUsedBytes = total, used
	}
	if total, used, err := DiskUsage("/"); err == nil {
		st.DiskTotalBytes, st.DiskUsedBytes = total, used
	}
	return st, nil
}

// sshCommandExists is used by callers to decide whether to attempt `ss`
// based active-connection parsing at all, so a host missing it degrades to
// "unknown" rather than erroring the whole dashboard request.
func CommandAvailable(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}
