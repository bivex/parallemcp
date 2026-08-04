package parallels

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

// SMBShareParams describes creating a new SMB share on a guest VM.
type SMBShareParams struct {
	ShareName  string
	FolderPath string
}

// SMBMountParams describes mounting a remote SMB share on a guest VM.
type SMBMountParams struct {
	RemoteIP    string
	ShareName   string
	DriveLetter string // e.g. "S:" or "S"
}

// SMBUnmountParams describes removing a share or unmounting a drive.
type SMBUnmountParams struct {
	DriveLetter string // e.g. "S:"
	ShareName   string
}

// SMBShare creates a local folder and exposes it as an SMB share.
func (c *Client) SMBShare(ctx context.Context, vmID string, p SMBShareParams) error {
	if strings.TrimSpace(p.ShareName) == "" || strings.TrimSpace(p.FolderPath) == "" {
		return errors.New("share_name and folder_path are required")
	}
	info, err := c.Info(ctx, vmID)
	if err != nil {
		return fmt.Errorf("read vm info: %w", err)
	}

	var cmd string
	if isWindowsOS(info.OS) {
		cmd = fmt.Sprintf(`$dir = '%s'; if (-not (Test-Path $dir)) { New-Item -ItemType Directory -Path $dir -Force }; net share "%s"="%s" /grant:everyone,full`, p.FolderPath, p.ShareName, p.FolderPath)
	} else {
		return fmt.Errorf("SMB sharing is currently supported for Windows guest VMs")
	}

	res, err := c.ExecOS(ctx, vmID, cmd, info.OS)
	if err != nil {
		return err
	}
	if res.ExitCode != 0 {
		return fmt.Errorf("create smb share failed (exit %d): %s", res.ExitCode, res.Stderr)
	}
	return nil
}

// SMBMount connects a remote SMB share to a local drive letter.
func (c *Client) SMBMount(ctx context.Context, vmID string, p SMBMountParams) error {
	if strings.TrimSpace(p.RemoteIP) == "" || strings.TrimSpace(p.ShareName) == "" {
		return errors.New("remote_ip and share_name are required")
	}
	drive := strings.TrimSpace(p.DriveLetter)
	if drive == "" {
		drive = "S:"
	}
	if !strings.HasSuffix(drive, ":") {
		drive += ":"
	}

	info, err := c.Info(ctx, vmID)
	if err != nil {
		return fmt.Errorf("read vm info: %w", err)
	}

	cmd := fmt.Sprintf(`net use %s \\%s\%s /persistent:yes`, drive, p.RemoteIP, p.ShareName)
	res, err := c.ExecOS(ctx, vmID, cmd, info.OS)
	if err != nil {
		return err
	}
	if res.ExitCode != 0 {
		return fmt.Errorf("mount smb share failed (exit %d): %s", res.ExitCode, res.Stderr)
	}
	return nil
}

// SMBUnmount removes a local share or unmounts a mapped drive.
func (c *Client) SMBUnmount(ctx context.Context, vmID string, p SMBUnmountParams) error {
	info, err := c.Info(ctx, vmID)
	if err != nil {
		return fmt.Errorf("read vm info: %w", err)
	}

	var cmd string
	if p.DriveLetter != "" {
		drive := p.DriveLetter
		if !strings.HasSuffix(drive, ":") {
			drive += ":"
		}
		cmd = fmt.Sprintf(`net use %s /delete /y`, drive)
	} else if p.ShareName != "" {
		cmd = fmt.Sprintf(`net share "%s" /delete /y`, p.ShareName)
	} else {
		return errors.New("drive_letter or share_name is required for unmount")
	}

	res, err := c.ExecOS(ctx, vmID, cmd, info.OS)
	if err != nil {
		return err
	}
	if res.ExitCode != 0 {
		return fmt.Errorf("unmount failed (exit %d): %s", res.ExitCode, res.Stderr)
	}
	return nil
}
