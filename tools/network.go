package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"parallemcp/parallels"
)

func (t *Tools) registerNetwork(s *mcp.Server) {
	mcp.AddTool(s, &mcp.Tool{
		Name:        "vm_network",
		Description: "Manage network adapters for a VM (list adapters, configure adapter type, interface, or status).",
	}, t.vmNetwork)
}

type vmNetworkInput struct {
	Action  string `json:"action" jsonschema:"action to perform: 'list' or 'configure'"`
	VM      string `json:"vm" jsonschema:"VM name or UUID"`
	Device  string `json:"device,omitempty" jsonschema:"network device name (default 'net0')"`
	Type    string `json:"type,omitempty" jsonschema:"network adapter type: 'shared', 'bridged', or 'host-only'"`
	Iface   string `json:"iface,omitempty" jsonschema:"host interface for bridged mode (e.g. 'Wi-Fi', 'default')"`
	Enabled *bool  `json:"enabled,omitempty" jsonschema:"enable or disable the network adapter"`
}

func (t *Tools) vmNetwork(ctx context.Context, req *mcp.CallToolRequest, in vmNetworkInput) (*mcp.CallToolResult, noOut, error) {
	if in.VM == "" {
		return errResult("`vm` is required"), noOut{}, nil
	}

	switch strings.ToLower(in.Action) {
	case "list":
		info, err := t.cli.Info(ctx, in.VM)
		if err != nil {
			return fail("list network adapters", err), noOut{}, nil
		}
		nets := info.Nets()
		if len(nets) == 0 {
			return textResult(fmt.Sprintf("No network adapters configured for **%s**.", in.VM)), noOut{}, nil
		}
		var b strings.Builder
		fmt.Fprintf(&b, "## Network Adapters @ %s\n\n", in.VM)
		b.WriteString("| Device | Type | Card | Host Interface | MAC | Enabled |\n|---|---|---|---|---|---|\n")
		for i, n := range nets {
			dev := fmt.Sprintf("net%d", i)
			iface := n.Iface
			if iface == "" {
				iface = "—"
			}
			enabled := "yes"
			if !n.Enabled {
				enabled = "no"
			}
			fmt.Fprintf(&b, "| %s | %s | %s | %s | `%s` | %s |\n", dev, n.Type, n.Card, iface, n.MAC, enabled)
		}
		return textResult(b.String()), noOut{}, nil

	case "configure", "set":
		dev := in.Device
		if dev == "" {
			dev = "net0"
		}
		err := t.cli.NetworkConfigure(ctx, in.VM, parallels.NetworkConfigureParams{
			Device:  dev,
			Type:    in.Type,
			Iface:   in.Iface,
			Enabled: in.Enabled,
		})
		if err != nil {
			return fail("configure network adapter", err), noOut{}, nil
		}
		var changes []string
		if in.Type != "" {
			changes = append(changes, "type="+in.Type)
		}
		if in.Iface != "" {
			changes = append(changes, "iface="+in.Iface)
		}
		if in.Enabled != nil {
			status := "enabled"
			if !*in.Enabled {
				status = "disabled"
			}
			changes = append(changes, "status="+status)
		}
		return textResult(fmt.Sprintf("✅ Configured adapter **%s** on **%s**: %s.", dev, in.VM, strings.Join(changes, ", "))), noOut{}, nil

	default:
		return errResult("`action` must be 'list' or 'configure'"), noOut{}, nil
	}
}
