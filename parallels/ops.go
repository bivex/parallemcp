package parallels

import (
	"context"
	"errors"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// ExecResult is the captured result of a command run inside a guest VM.
type ExecResult struct {
	Stdout   string
	Stderr   string
	ExitCode int
}

// Exec runs command inside a running VM via `prlctl exec`. The command is a shell
// string executed with `/bin/sh -lc`, so pipes, redirection and `&&` work. The
// guest command's non-zero exit code is surfaced in ExitCode (not as an error);
// only a failure to execute at all (e.g. Parallels Tools not running) is an error.
func (c *Client) Exec(ctx context.Context, id, command string) (*ExecResult, error) {
	r, err := c.exec(ctx, Prlctl, "exec", id, "/bin/sh", "-lc", command)
	res := &ExecResult{Stdout: r.Stdout, Stderr: r.Stderr, ExitCode: r.ExitCode}
	if err == nil {
		return res, nil
	}
	if ctx.Err() != nil {
		return res, ctx.Err()
	}
	if looksLikeExecFailure(r.Stderr) {
		return res, err
	}
	// Otherwise: the guest command itself exited non-zero. Return its output with
	// the exit code, and no Go-level error so the caller can present stdout/stderr.
	return res, nil
}

// looksLikeExecFailure reports whether stderr indicates prlctl could not run the
// command at all (vs. the guest command merely exiting non-zero).
func looksLikeExecFailure(stderr string) bool {
	s := strings.ToLower(stderr)
	for _, p := range []string{
		"unable to execute", "unable to perform action",
		"parallels tools", "are not installed", "is not installed",
		"is not running", "could not be found", "not registered",
	} {
		if strings.Contains(s, p) {
			return true
		}
	}
	return false
}

// ConfigureParams describes which VM attributes to change. Only non-zero / non-
// empty fields are applied.
type ConfigureParams struct {
	CPUs     int
	MemoryMB int
	Name     string
}

// Configure updates CPU count, memory, and/or the VM name via `prlctl set`.
func (c *Client) Configure(ctx context.Context, id string, p ConfigureParams) error {
	args := []string{"set", id}
	if p.CPUs > 0 {
		args = append(args, "--cpus", strconv.Itoa(p.CPUs))
	}
	if p.MemoryMB > 0 {
		args = append(args, "--memsize", strconv.Itoa(p.MemoryMB))
	}
	if strings.TrimSpace(p.Name) != "" {
		args = append(args, "--name", p.Name)
	}
	if len(args) <= 2 {
		return errors.New("nothing to configure: provide cpus, memory_mb, or name")
	}
	return c.ok(ctx, Prlctl, args...)
}

// ServerInfo returns Parallels Desktop version, license, and host info.
func (c *Client) ServerInfo(ctx context.Context) (*ServerInfo, error) {
	var si ServerInfo
	if _, err := c.runJSON(ctx, &si, Prlsrvctl, "info", "--json"); err != nil {
		return nil, err
	}
	return &si, nil
}

// HostStats gathers macOS host statistics. Each source is parsed independently
// and best-effort; a failing command leaves its fields zero rather than failing
// the whole call.
func (c *Client) HostStats(ctx context.Context) (*HostStats, error) {
	hs := &HostStats{}

	if out := c.capture(ctx, "system_profiler", "SPHardwareDataType"); out != "" {
		hs.Chip = matchFirst(out, regexp.MustCompile(`(?m)^\s*Chip:\s*(.+)$`))
		if n := matchFirst(out, regexp.MustCompile(`(?m)^\s*Total Number of Cores:\s*(\d+)`)); n != "" {
			hs.PhysicalCores, _ = strconv.Atoi(n)
			hs.LogicalCores = hs.PhysicalCores
		}
		if v, unit := memLabel(out); v > 0 {
			hs.MemoryTotalGB = toGB(v, unit)
		}
	}

	if n := c.capture(ctx, "sysctl", "-n", "hw.logicalcpu"); n != "" {
		if v, err := strconv.Atoi(strings.TrimSpace(n)); err == nil && v > 0 {
			hs.LogicalCores = v
		}
	}
	if hs.PhysicalCores == 0 {
		if n := c.capture(ctx, "sysctl", "-n", "hw.physicalcpu"); n != "" {
			hs.PhysicalCores, _ = strconv.Atoi(strings.TrimSpace(n))
		}
	}
	if hs.MemoryTotalGB == 0 {
		if n := c.capture(ctx, "sysctl", "-n", "hw.memsize"); n != "" {
			if bytes, err := strconv.ParseInt(strings.TrimSpace(n), 10, 64); err == nil {
				hs.MemoryTotalGB = float64(bytes) / 1e9
			}
		}
	}
	hs.MemoryFreeGB = c.freeMemoryGB(ctx)

	if used, total, ok := c.rootDiskGB(ctx); ok {
		hs.DiskUsedGB, hs.DiskTotalGB = used, total
	}

	if la := parseLoadAvg(c.capture(ctx, "sysctl", "-n", "vm.loadavg")); la != nil {
		hs.LoadAvg1, hs.LoadAvg5, hs.LoadAvg15 = la[0], la[1], la[2]
	}

	hs.UptimeDays = c.uptimeDays(ctx)
	return hs, nil
}

// capture runs `bin args...` and returns its stdout ("" on any error).
func (c *Client) capture(ctx context.Context, bin string, args ...string) string {
	r, err := c.exec(ctx, bin, args...)
	if err != nil {
		return ""
	}
	return r.Stdout
}

// freeMemoryGB computes available memory (free + inactive + speculative) in GB
// from `vm_stat`.
func (c *Client) freeMemoryGB(ctx context.Context) float64 {
	out := c.capture(ctx, "vm_stat")
	if out == "" {
		return 0
	}
	page := 4096
	if m := regexp.MustCompile(`page size of (\d+) bytes`).FindStringSubmatch(out); len(m) == 2 {
		if v, err := strconv.Atoi(m[1]); err == nil {
			page = v
		}
	}
	pages := 0
	for _, key := range []string{"Pages free", "Pages inactive", "Pages speculative"} {
		if m := regexp.MustCompile(`(?m)^` + regexp.QuoteMeta(key) + `:\s+(\d+)`).FindStringSubmatch(out); len(m) == 2 {
			if v, err := strconv.Atoi(m[1]); err == nil {
				pages += v
			}
		}
	}
	return float64(pages*page) / 1e9
}

// rootDiskGB returns (usedGB, totalGB, ok) for the root filesystem from `df -k /`.
func (c *Client) rootDiskGB(ctx context.Context) (float64, float64, bool) {
	out := c.capture(ctx, "df", "-k", "/")
	lines := strings.Split(out, "\n")
	for _, ln := range lines[1:] {
		f := strings.Fields(ln)
		if len(f) < 4 || f[len(f)-1] != "/" {
			continue
		}
		totalKB, e1 := strconv.ParseFloat(f[1], 64)
		usedKB, e2 := strconv.ParseFloat(f[2], 64)
		if e1 == nil && e2 == nil {
			return usedKB / 1e6, totalKB / 1e6, true
		}
	}
	return 0, 0, false
}

// uptimeDays returns host uptime in days from `sysctl kern.boottime`.
func (c *Client) uptimeDays(ctx context.Context) float64 {
	out := c.capture(ctx, "sysctl", "-n", "kern.boottime")
	m := regexp.MustCompile(`sec\s*=\s*(\d+)`).FindStringSubmatch(out)
	if len(m) != 2 {
		return 0
	}
	boot, err := strconv.ParseInt(m[1], 10, 64)
	if err != nil {
		return 0
	}
	d := time.Since(time.Unix(boot, 0))
	if d < 0 {
		return 0
	}
	return d.Hours() / 24
}

func matchFirst(s string, re *regexp.Regexp) string {
	m := re.FindStringSubmatch(s)
	if len(m) < 2 {
		return ""
	}
	return strings.TrimSpace(m[1])
}

// memLabel extracts the numeric value and unit from a "Memory: 64 GB" line.
func memLabel(s string) (float64, string) {
	m := regexp.MustCompile(`(?m)^\s*Memory:\s*([\d.]+)\s*(GB|MB|TB)`).FindStringSubmatch(s)
	if len(m) != 3 {
		return 0, ""
	}
	v, err := strconv.ParseFloat(m[1], 64)
	if err != nil {
		return 0, ""
	}
	return v, m[2]
}

// toGB converts a value in the given unit to gigabytes.
func toGB(v float64, unit string) float64 {
	switch strings.ToUpper(unit) {
	case "TB":
		return v * 1024
	case "MB":
		return v / 1024
	default:
		return v
	}
}

// parseLoadAvg parses the three load averages from `sysctl -n vm.loadavg`
// ("{ 0.10 0.20 0.30 }").
func parseLoadAvg(s string) []float64 {
	matches := regexp.MustCompile(`\d+\.\d+`).FindAllString(s, -1)
	if len(matches) < 3 {
		return nil
	}
	out := make([]float64, 3)
	for i := 0; i < 3; i++ {
		out[i], _ = strconv.ParseFloat(matches[i], 64)
	}
	return out
}
