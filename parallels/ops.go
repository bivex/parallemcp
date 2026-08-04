package parallels

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf16"
)

// ExecResult is the captured result of a command run inside a guest VM.
type ExecResult struct {
	Stdout   string
	Stderr   string
	ExitCode int
}

// Exec runs command inside a running VM, choosing the guest shell by OS:
// Linux/macOS use `/bin/sh -lc`; Windows uses
// `powershell.exe -NoProfile -NonInteractive -EncodedCommand <base64>` (see
// guestShellArgs). The guest command's non-zero exit code is surfaced in
// ExitCode (not as an error); only a failure to execute at all (e.g. Parallels
// Tools not running) is an error.
//
// The OS is "" here, which defaults to the Unix shell. Callers that already know
// the guest OS (e.g. after a lookup) should use ExecOS to avoid a detection
// round-trip.
func (c *Client) Exec(ctx context.Context, id, command string) (*ExecResult, error) {
	return c.ExecOS(ctx, id, command, "")
}

// ExecOS is like Exec but takes the guest OS so the right shell is selected
// without an extra lookup. The OS string is the one reported by `prlctl list -i`
// (e.g. "win-11", "linux", "macos").
func (c *Client) ExecOS(ctx context.Context, id, command, os string) (*ExecResult, error) {
	args := append([]string{"exec", id}, guestShellArgs(os, command)...)
	r, err := c.exec(ctx, Prlctl, args...)
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

// guestShellArgs returns the shell+args to run command in a guest of the given OS.
// For Windows guests, it runs cmd.exe /c. If the command appears to be a PowerShell
// cmdlet or script, it automatically wraps it with powershell.exe -Command "...".
func guestShellArgs(os, command string) []string {
	if isWindowsOS(os) {
		if isPowerShellCommand(command) {
			psCmd := fmt.Sprintf("powershell -NoProfile -NonInteractive -ExecutionPolicy Bypass -Command %s", quoteCmdArg(command))
			return []string{"cmd.exe", "/c", psCmd}
		}
		return []string{"cmd.exe", "/c", command}
	}
	return []string{"/bin/sh", "-lc", command}
}

// isPowerShellCommand reports whether command appears to be a PowerShell script
// or cmdlet (e.g. starts with Get-, Set-, $, or contains PS cmdlets).
func isPowerShellCommand(command string) bool {
	trimmed := strings.TrimSpace(command)
	if strings.HasPrefix(trimmed, "$") {
		return true
	}
	lower := strings.ToLower(trimmed)
	prefixes := []string{
		"get-", "set-", "new-", "remove-", "start-", "stop-", "restart-",
		"invoke-", "test-", "update-", "enable-", "disable-", "clear-", "copy-item",
	}
	for _, p := range prefixes {
		if strings.HasPrefix(lower, p) {
			return true
		}
	}
	for _, kw := range []string{"select-object", "where-object", "foreach-object", "format-table", "format-list"} {
		if strings.Contains(lower, kw) {
			return true
		}
	}
	return false
}

func quoteCmdArg(arg string) string {
	if strings.ContainsAny(arg, " \t\n\v\"") || strings.Contains(arg, "|") || strings.Contains(arg, "&") {
		escaped := strings.ReplaceAll(arg, `"`, `\"`)
		return `"` + escaped + `"`
	}
	return arg
}

// isWindowsOS reports whether os denotes a Windows guest (Parallels reports
// values like "win-11", "win-10").
func isWindowsOS(os string) bool {
	return strings.HasPrefix(strings.ToLower(strings.TrimSpace(os)), "win")
}

// encodePowerShell encodes s as UTF-16LE and base64, the form expected by
// PowerShell's -EncodedCommand. Newlines, quotes, $, |, >, backticks, and
// non-ASCII all survive intact.
func encodePowerShell(s string) string {
	u16 := utf16.Encode([]rune(s))
	buf := make([]byte, len(u16)*2)
	for i, v := range u16 {
		buf[2*i] = byte(v)
		buf[2*i+1] = byte(v >> 8)
	}
	return base64.StdEncoding.EncodeToString(buf)
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

// SharedFolderAddParams describes options for adding a host shared folder.
type SharedFolderAddParams struct {
	Name string
	Path string
	Mode string // "rw" | "ro"
}

// SharedFolderAdd adds a host shared folder to a VM using `prlctl set <vm> --shf-host-add <name> --path <path>`.
func (c *Client) SharedFolderAdd(ctx context.Context, id string, p SharedFolderAddParams) error {
	if strings.TrimSpace(p.Name) == "" || strings.TrimSpace(p.Path) == "" {
		return errors.New("name and path are required")
	}
	args := []string{"set", id, "--shf-host-add", p.Name, "--path", p.Path}
	if p.Mode != "" {
		args = append(args, "--mode", p.Mode)
	}
	return c.ok(ctx, Prlctl, args...)
}

// SharedFolderRemove removes a host shared folder from a VM using `prlctl set <vm> --shf-host-del <name>`.
func (c *Client) SharedFolderRemove(ctx context.Context, id, name string) error {
	if strings.TrimSpace(name) == "" {
		return errors.New("shared folder name is required")
	}
	return c.ok(ctx, Prlctl, "set", id, "--shf-host-del", name)
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
