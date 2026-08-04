package parallels

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// FileCopyParams specifies source, destination, and direction for a file transfer.
type FileCopyParams struct {
	Direction string // "to_guest" | "from_guest"
	HostPath  string
	GuestPath string
}

// FileCopy transfers a file between host and guest VM using base64 stream via ExecOS.
func (c *Client) FileCopy(ctx context.Context, vmID string, p FileCopyParams) error {
	info, err := c.Info(ctx, vmID)
	if err != nil {
		return fmt.Errorf("read vm info: %w", err)
	}

	switch strings.ToLower(p.Direction) {
	case "to_guest", "host_to_guest", "push":
		data, err := os.ReadFile(p.HostPath)
		if err != nil {
			return fmt.Errorf("read host file %q: %w", p.HostPath, err)
		}
		b64 := base64.StdEncoding.EncodeToString(data)
		return c.writeGuestFile(ctx, vmID, info.OS, p.GuestPath, b64)

	case "from_guest", "guest_to_host", "pull":
		b64, err := c.readGuestFile(ctx, vmID, info.OS, p.GuestPath)
		if err != nil {
			return fmt.Errorf("read guest file %q: %w", p.GuestPath, err)
		}
		data, err := base64.StdEncoding.DecodeString(strings.TrimSpace(b64))
		if err != nil {
			return fmt.Errorf("decode file payload: %w", err)
		}
		if err := os.MkdirAll(filepath.Dir(p.HostPath), 0755); err != nil {
			return fmt.Errorf("create host directory: %w", err)
		}
		if err := os.WriteFile(p.HostPath, data, 0644); err != nil {
			return fmt.Errorf("write host file %q: %w", p.HostPath, err)
		}
		return nil

	default:
		return errors.New("direction must be 'to_guest' or 'from_guest'")
	}
}

func (c *Client) writeGuestFile(ctx context.Context, vmID, guestOS, guestPath, b64 string) error {
	var cmd string
	if isWindowsOS(guestOS) {
		dir := filepath.Dir(guestPath)
		cmd = fmt.Sprintf(`$dir = '%s'; if (-not (Test-Path $dir)) { New-Item -ItemType Directory -Path $dir -Force }; [System.IO.File]::WriteAllBytes('%s', [System.Convert]::FromBase64String('%s'))`, dir, guestPath, b64)
	} else {
		dir := filepath.Dir(guestPath)
		cmd = fmt.Sprintf(`mkdir -p '%s' && echo '%s' | base64 -d > '%s'`, dir, b64, guestPath)
	}
	res, err := c.ExecOS(ctx, vmID, cmd, guestOS)
	if err != nil {
		return err
	}
	if res.ExitCode != 0 {
		return fmt.Errorf("write guest file failed (exit %d): %s", res.ExitCode, res.Stderr)
	}
	return nil
}

func (c *Client) readGuestFile(ctx context.Context, vmID, guestOS, guestPath string) (string, error) {
	var cmd string
	if isWindowsOS(guestOS) {
		cmd = fmt.Sprintf(`[System.Convert]::ToBase64String([System.IO.File]::ReadAllBytes('%s'))`, guestPath)
	} else {
		cmd = fmt.Sprintf(`base64 '%s'`, guestPath)
	}
	res, err := c.ExecOS(ctx, vmID, cmd, guestOS)
	if err != nil {
		return "", err
	}
	if res.ExitCode != 0 {
		return "", fmt.Errorf("read guest file failed (exit %d): %s", res.ExitCode, res.Stderr)
	}
	return res.Stdout, nil
}
