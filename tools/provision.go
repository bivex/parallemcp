package tools

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"parallemcp/parallels"
)

func (t *Tools) registerProvisioning(s *mcp.Server) {
	mcp.AddTool(s, &mcp.Tool{
		Name:        "vm_create",
		Description: "Create a new VM from a Parallels ostemplate (default Debian). Optionally sets CPU/memory, starts it, injects your SSH public key for root, and reports the guest IP. Note: a VM created from an ostype without a registered template may lack an OS disk; SSH injection and IP discovery are best-effort in that case.",
	}, t.vmCreate)
	mcp.AddTool(s, &mcp.Tool{
		Name:        "vm_clone",
		Description: "Clone an existing VM into a new one. Linked clones share the source disk (fast, small); full clones copy everything.",
	}, t.vmClone)
	mcp.AddTool(s, &mcp.Tool{
		Name:        "vm_delete",
		Description: "Delete a VM and its disk files. Destructive — requires confirm=true.",
	}, t.vmDelete)
	mcp.AddTool(s, &mcp.Tool{
		Name:        "template_list",
		Description: "List registered VM templates and common base-image (ostype/distribution) tokens.",
	}, t.templateList)
}

type vmCreateInput struct {
	Name          string `json:"name" jsonschema:"name for the new VM"`
	CPUs          int    `json:"cpus,omitempty" jsonschema:"number of virtual CPUs to assign"`
	MemoryMB      int    `json:"memory_mb,omitempty" jsonschema:"memory in megabytes to assign"`
	Distribution  string `json:"distribution,omitempty" jsonschema:"Parallels distribution/ostype token (default 'debian')"`
	SSHPubKeyPath string `json:"ssh_pubkey_path,omitempty" jsonschema:"path to a public key to inject for root; defaults to the first key found under ~/.ssh"`
	IPWaitSeconds int    `json:"ip_wait_seconds,omitempty" jsonschema:"seconds to wait for a guest IPv4 (default 60)"`
}

func (t *Tools) vmCreate(ctx context.Context, req *mcp.CallToolRequest, in vmCreateInput) (*mcp.CallToolResult, noOut, error) {
	if strings.TrimSpace(in.Name) == "" {
		return errResult("`name` is required"), noOut{}, nil
	}

	// Resolve the SSH key (explicit path, else auto-detect from ~/.ssh).
	pubkey, keyErr := parallels.DetectSSHPubKey(in.SSHPubKeyPath)
	if keyErr != nil {
		// Non-fatal: proceed without injection, but warn.
		pubkey = ""
	}

	var wait time.Duration
	if in.IPWaitSeconds > 0 {
		wait = time.Duration(in.IPWaitSeconds) * time.Second
	}

	res, err := t.cli.Create(ctx, parallels.CreateParams{
		Name:         in.Name,
		Distribution: in.Distribution,
		CPUs:         in.CPUs,
		MemoryMB:     in.MemoryMB,
		SSHPubKey:    pubkey,
		IPWait:       wait,
	})
	if err != nil {
		return fail("create VM", err), noOut{}, nil
	}

	var b strings.Builder
	fmt.Fprintf(&b, "## Created **%s**\n\n", res.Name)
	if res.UUID != "" {
		fmt.Fprintf(&b, "- **uuid**: `%s`\n", res.UUID)
	}
	fmt.Fprintf(&b, "- **created**: %v\n", res.Created)
	fmt.Fprintf(&b, "- **configured (cpu/mem)**: %v\n", res.Configured)
	fmt.Fprintf(&b, "- **started**: %v\n", res.Started)
	fmt.Fprintf(&b, "- **ssh key injected (root)**: %v\n", res.SSHInjected)

	if keyErr != nil {
		fmt.Fprintf(&b, "\n_⚠️ SSH key not available (%s); skipping injection._\n", keyErr)
	}
	if len(res.IPs) > 0 {
		b.WriteString("\n### Guest IPv4\n\n")
		for _, ip := range res.IPs {
			fmt.Fprintf(&b, "- `%s`\n", ip)
		}
		if res.SSHInjected {
			fmt.Fprintf(&b, "\n`ssh root@%s`\n", res.IPs[0])
		}
	} else if res.Started {
		b.WriteString("\n_No guest IPv4 was detected within the wait window; the VM may still be booting or lacks Parallels Tools._\n")
	}
	if len(res.Warnings) > 0 {
		b.WriteString("\n### Warnings\n\n")
		for _, w := range res.Warnings {
			fmt.Fprintf(&b, "- %s\n", w)
		}
	}
	return textResult(b.String()), noOut{}, nil
}

type vmCloneInput struct {
	Source string `json:"source" jsonschema:"name or UUID of the VM to clone"`
	Name   string `json:"name" jsonschema:"name for the new clone"`
	Linked bool   `json:"linked,omitempty" jsonschema:"create a linked clone (shares source disk) instead of a full copy"`
}

func (t *Tools) vmClone(ctx context.Context, req *mcp.CallToolRequest, in vmCloneInput) (*mcp.CallToolResult, noOut, error) {
	if in.Source == "" || in.Name == "" {
		return errResult("`source` and `name` are required"), noOut{}, nil
	}
	if err := t.cli.Clone(ctx, in.Source, in.Name, in.Linked); err != nil {
		return fail("clone VM", err), noOut{}, nil
	}
	kind := "full"
	if in.Linked {
		kind = "linked"
	}
	return textResult(fmt.Sprintf("✅ %s clone: **%s** → **%s**.", kind, in.Source, in.Name)), noOut{}, nil
}

type vmDeleteInput struct {
	VM      string `json:"vm" jsonschema:"VM name or UUID to delete"`
	Confirm bool   `json:"confirm" jsonschema:"must be true to confirm deletion of the VM and its disk files"`
}

func (t *Tools) vmDelete(ctx context.Context, req *mcp.CallToolRequest, in vmDeleteInput) (*mcp.CallToolResult, noOut, error) {
	if in.VM == "" {
		return errResult("`vm` is required"), noOut{}, nil
	}
	if !in.Confirm {
		return errResult(fmt.Sprintf(
			"Refusing to delete **%s** without confirmation. Re-run with `confirm: true`.", in.VM)), noOut{}, nil
	}
	if err := t.cli.Delete(ctx, in.VM); err != nil {
		return fail("delete VM", err), noOut{}, nil
	}
	return textResult(fmt.Sprintf("🗑️ Deleted **%s** and its disk files.", in.VM)), noOut{}, nil
}

type templateListInput struct{}

func (t *Tools) templateList(ctx context.Context, req *mcp.CallToolRequest, in templateListInput) (*mcp.CallToolResult, noOut, error) {
	tl, err := t.cli.Templates(ctx)
	if err != nil {
		return fail("list templates", err), noOut{}, nil
	}
	var b strings.Builder
	b.WriteString("## Registered templates\n\n")
	if len(tl.Registered) == 0 {
		b.WriteString("_None. Register a template with `prlctl register`, or download one from the Parallels catalog._\n")
	} else {
		b.WriteString("| Name | Status | UUID |\n|---|---|---|\n")
		for _, v := range tl.Registered {
			fmt.Fprintf(&b, "| %s | %s | `%s` |\n", v.Name, v.Status, v.UUID)
		}
	}
	b.WriteString("\n## Common base images (ostype / distribution tokens)\n\n")
	for _, d := range tl.Distributions {
		fmt.Fprintf(&b, "- `%s`\n", d)
	}
	b.WriteString("\n_Pass one of these as `distribution` to `vm_create` (default `debian`)._\n")
	return textResult(b.String()), noOut{}, nil
}
