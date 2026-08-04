package parallels

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const (
	DefaultOSType       = "debian"
	DefaultDistribution = "debian"

	// DefaultIPWait is how long Create polls for a guest IPv4 before giving up.
	DefaultIPWait  = 60 * time.Second
	ipPollInterval = 2 * time.Second
)

// KnownDistributions lists common Parallels ostype/distribution tokens. These
// are advisory: Parallels 20.x does not implement `prlctl create -o list`, so we
// surface a curated set instead.
var KnownDistributions = []string{
	"debian", "ubuntu", "fedora", "kali-linux", "centos", "rocky", "alma", "arch", "opensuse",
}

// CreateParams configures a new VM created from a Parallels ostemplate.
type CreateParams struct {
	Name         string
	OSType       string
	Distribution string
	CPUs         int
	MemoryMB     int
	SSHPubKey    string        // public key contents (not a path) to inject for root
	IPWait       time.Duration // how long to wait for a guest IPv4; 0 => DefaultIPWait
}

// CreateResult captures the outcome of a best-effort VM creation. Steps after
// the initial `create` may fail (e.g. the VM has no OS disk yet) without
// aborting; those failures are reported in Warnings.
type CreateResult struct {
	Name        string
	UUID        string
	Created     bool
	Configured  bool
	Started     bool
	SSHInjected bool
	IPs         []string
	Warnings    []string
}

// TemplateList combines registered templates with the curated distribution set.
type TemplateList struct {
	Registered    []VMListEntry
	Distributions []string
}

// Create provisions a new VM via `prlctl create` from an ostemplate, optionally
// sets CPU/memory, starts it, injects an SSH key for root (best effort), and
// polls for a guest IPv4. Post-create steps are best-effort: a VM created from
// an ostype without a registered template may lack an OS disk, so SSH injection
// and IP discovery can fail; those outcomes are recorded in Warnings rather than
// returned as errors.
func (c *Client) Create(ctx context.Context, p CreateParams) (*CreateResult, error) {
	if strings.TrimSpace(p.Name) == "" {
		return nil, errors.New("vm name is required")
	}
	ostype := p.OSType
	if ostype == "" {
		ostype = DefaultOSType
	}
	dist := p.Distribution
	if dist == "" {
		dist = DefaultDistribution
	}
	wait := p.IPWait
	if wait <= 0 {
		wait = DefaultIPWait
	}

	res := &CreateResult{Name: p.Name}

	// 1. Create the VM shell from the ostemplate.
	createArgs := []string{"create", p.Name}
	if p.OSType != "" {
		createArgs = append(createArgs, "-o", p.OSType)
	}
	if p.Distribution != "" {
		createArgs = append(createArgs, "-d", p.Distribution)
	}

	if _, err := c.exec(ctx, Prlctl, createArgs...); err != nil {
		// Fallback: if ostype/distribution flags fail on this host OS/arch, create basic VM shell
		if _, fallbackErr := c.exec(ctx, Prlctl, "create", p.Name); fallbackErr != nil {
			return res, err
		}
	}
	res.Created = true

	// 2. Read back the UUID (best effort).
	if e, err := c.Find(ctx, p.Name); err == nil {
		res.UUID = e.UUID
	} else {
		res.Warnings = append(res.Warnings, "could not read VM UUID: "+err.Error())
	}

	// 3. Apply CPU/memory when requested.
	setArgs := []string{"set", p.Name}
	if p.CPUs > 0 {
		setArgs = append(setArgs, "--cpus", strconv.Itoa(p.CPUs))
	}
	if p.MemoryMB > 0 {
		setArgs = append(setArgs, "--memsize", strconv.Itoa(p.MemoryMB))
	}
	if len(setArgs) > 2 {
		if err := c.ok(ctx, Prlctl, setArgs...); err != nil {
			res.Warnings = append(res.Warnings, "configure CPU/memory failed: "+oneLine(err.Error()))
		} else {
			res.Configured = true
		}
	}

	// 4. Start (best effort; the VM may not be bootable without an OS disk).
	if err := c.ok(ctx, Prlctl, "start", p.Name); err != nil {
		res.Warnings = append(res.Warnings, "start failed: "+oneLine(err.Error()))
		return res, nil
	}
	res.Started = true

	// 5. Inject SSH key for root (best effort).
	if strings.TrimSpace(p.SSHPubKey) != "" {
		ok, msg := c.injectRootSSHKey(ctx, p.Name, p.SSHPubKey)
		res.SSHInjected = ok
		if msg != "" {
			res.Warnings = append(res.Warnings, msg)
		}
	}

	// 6. Poll for a guest IPv4.
	res.IPs = c.waitForIPv4(ctx, p.Name, wait)
	return res, nil
}

// injectRootSSHKey appends pubkey to /root/.ssh/authorized_keys inside vmID via
// `prlctl exec`. The key is base64-encoded to avoid any shell-quoting issues.
// Returns (success, humanMessage).
func (c *Client) injectRootSSHKey(ctx context.Context, vmID, pubkey string) (bool, string) {
	b64 := base64.StdEncoding.EncodeToString([]byte(strings.TrimSpace(pubkey)))
	script := "umask 077; mkdir -p /root/.ssh; echo " + b64 + " | base64 -d >> /root/.ssh/authorized_keys"
	r, err := c.exec(ctx, Prlctl, "exec", vmID, "/bin/sh", "-lc", script)
	if err != nil {
		msg := strings.TrimSpace(r.Stderr)
		if msg == "" {
			msg = "SSH key injection failed (Parallels Tools may not be running yet)"
		}
		return false, "ssh injection: " + oneLine(msg)
	}
	return true, ""
}

// waitForIPv4 polls vmID until it reports at least one IPv4, the deadline elapses,
// or ctx is cancelled. Returns any IPs found (possibly empty).
func (c *Client) waitForIPv4(ctx context.Context, vmID string, wait time.Duration) []string {
	deadline := time.Now().Add(wait)
	for {
		if info, err := c.Info(ctx, vmID); err == nil {
			if ips := info.IPv4s(); len(ips) > 0 {
				return ips
			}
		}
		if time.Now().After(deadline) {
			return nil
		}
		select {
		case <-ctx.Done():
			return nil
		case <-time.After(ipPollInterval):
		}
	}
}

// Clone duplicates an existing VM. A linked clone shares the source's disk with
// the source snapshot (fast, small); a full clone copies everything.
func (c *Client) Clone(ctx context.Context, source, name string, linked bool) error {
	if strings.TrimSpace(source) == "" || strings.TrimSpace(name) == "" {
		return errors.New("source and name are required")
	}
	args := []string{"clone", source, "--name", name}
	if linked {
		args = append(args, "--linked")
	}
	return c.ok(ctx, Prlctl, args...)
}

// Delete removes a VM and its disk files. Callers MUST gate this behind an
// explicit confirmation; this method performs no confirmation of its own.
func (c *Client) Delete(ctx context.Context, id string) error {
	return c.ok(ctx, Prlctl, "delete", id)
}

// Templates lists registered templates plus the curated distribution set.
func (c *Client) Templates(ctx context.Context) (*TemplateList, error) {
	var regs []VMListEntry
	if _, err := c.runJSON(ctx, &regs, Prlctl, "list", "-t", "--json"); err != nil {
		return nil, err
	}
	return &TemplateList{Registered: regs, Distributions: KnownDistributions}, nil
}

// DetectSSHPubKey returns the contents of the SSH public key to inject. If path
// is non-empty it is read directly; otherwise the first existing default public
// key under ~/.ssh is returned. Returns ("", error) when none is found.
func DetectSSHPubKey(path string) (string, error) {
	if path != "" {
		b, err := os.ReadFile(path)
		if err != nil {
			return "", fmt.Errorf("read ssh pubkey %q: %w", path, err)
		}
		return strings.TrimSpace(string(b)), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	for _, name := range []string{"id_ed25519.pub", "id_ecdsa.pub", "id_rsa.pub"} {
		b, err := os.ReadFile(filepath.Join(home, ".ssh", name))
		if err == nil {
			return strings.TrimSpace(string(b)), nil
		}
	}
	return "", errors.New("no SSH public key found under ~/.ssh (pass ssh_pubkey_path)")
}
