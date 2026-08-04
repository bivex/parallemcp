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
	chunkSize := 4000

	if isWindowsOS(guestOS) {
		dir := guestDir(guestPath, true)
		tmpB64 := guestPath + ".tmp.b64"
		setupCmd := fmt.Sprintf(`$dir = '%s'; if (-not (Test-Path $dir)) { New-Item -ItemType Directory -Path $dir -Force }; Remove-Item -Path '%s' -ErrorAction SilentlyContinue`, dir, tmpB64)
		if _, err := c.ExecOS(ctx, vmID, setupCmd, guestOS); err != nil {
			return err
		}

		for i := 0; i < len(b64); i += chunkSize {
			end := i + chunkSize
			if end > len(b64) {
				end = len(b64)
			}
			chunk := b64[i:end]
			appendCmd := fmt.Sprintf(`[System.IO.File]::AppendAllText('%s', '%s')`, tmpB64, chunk)
			res, err := c.ExecOS(ctx, vmID, appendCmd, guestOS)
			if err != nil {
				return fmt.Errorf("append chunk failed: %w", err)
			}
			if res.ExitCode != 0 {
				return fmt.Errorf("append chunk failed (exit %d): %s", res.ExitCode, res.Stderr)
			}
		}

		decodeCmd := fmt.Sprintf(`$b64 = [System.IO.File]::ReadAllText('%s'); [System.IO.File]::WriteAllBytes('%s', [System.Convert]::FromBase64String($b64)); Remove-Item -Path '%s' -ErrorAction SilentlyContinue`, tmpB64, guestPath, tmpB64)
		res, err := c.ExecOS(ctx, vmID, decodeCmd, guestOS)
		if err != nil {
			return err
		}
		if res.ExitCode != 0 {
			return fmt.Errorf("decode file failed (exit %d): %s", res.ExitCode, res.Stderr)
		}
		return nil
	} else {
		dir := guestDir(guestPath, false)
		tmpB64 := guestPath + ".tmp.b64"
		setupCmd := fmt.Sprintf(`mkdir -p '%s' && rm -f '%s'`, dir, tmpB64)
		if _, err := c.ExecOS(ctx, vmID, setupCmd, guestOS); err != nil {
			return err
		}

		for i := 0; i < len(b64); i += chunkSize {
			end := i + chunkSize
			if end > len(b64) {
				end = len(b64)
			}
			chunk := b64[i:end]
			appendCmd := fmt.Sprintf(`printf '%%s' '%s' >> '%s'`, chunk, tmpB64)
			res, err := c.ExecOS(ctx, vmID, appendCmd, guestOS)
			if err != nil || res.ExitCode != 0 {
				return fmt.Errorf("append chunk failed: %v", err)
			}
		}

		decodeCmd := fmt.Sprintf(`base64 -d '%s' > '%s' && rm -f '%s'`, tmpB64, guestPath, tmpB64)
		res, err := c.ExecOS(ctx, vmID, decodeCmd, guestOS)
		if err != nil {
			return err
		}
		if res.ExitCode != 0 {
			return fmt.Errorf("decode file failed (exit %d): %s", res.ExitCode, res.Stderr)
		}
		return nil
	}
}

func guestDir(path string, isWin bool) string {
	if isWin {
		p := strings.ReplaceAll(path, "/", "\\")
		idx := strings.LastIndex(p, "\\")
		if idx > 0 {
			return p[:idx]
		}
		return p
	}
	return filepath.Dir(path)
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
