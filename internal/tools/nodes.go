package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"

	"github.com/aaronburt/pve-mcp-go/internal/config"
	"github.com/aaronburt/pve-mcp-go/internal/pve"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

type nodeGuestInfo struct {
	VMID   int    `json:"vmid"`
	Name   string `json:"name"`
	Status string `json:"status"`
}

func getRunningGuestsOnNode(ctx context.Context, client *pve.Client, node string) ([]nodeGuestInfo, []nodeGuestInfo, error) {
	var qemuList []nodeGuestInfo
	qemuPath := fmt.Sprintf("/nodes/%s/qemu", url.PathEscape(node))
	if err := client.Get(ctx, qemuPath, nil, &qemuList); err != nil {
		return nil, nil, fmt.Errorf("failed to list qemu vms on node %s: %w", node, err)
	}

	var lxcList []nodeGuestInfo
	lxcPath := fmt.Sprintf("/nodes/%s/lxc", url.PathEscape(node))
	if err := client.Get(ctx, lxcPath, nil, &lxcList); err != nil {
		return nil, nil, fmt.Errorf("failed to list lxc containers on node %s: %w", node, err)
	}

	var runningVMs []nodeGuestInfo
	for _, vm := range qemuList {
		if vm.Status == "running" {
			runningVMs = append(runningVMs, vm)
		}
	}

	var runningLXCs []nodeGuestInfo
	for _, ct := range lxcList {
		if ct.Status == "running" {
			runningLXCs = append(runningLXCs, ct)
		}
	}

	return runningVMs, runningLXCs, nil
}

func RegisterNodeTools(s *server.MCPServer, client *pve.Client, cfg *config.Config) {
	if cfg == nil || !cfg.EnableNodeOps {
		return
	}

	s.AddTool(
		mcp.NewTool(
			"pve_node_reboot",
			mcp.WithDescription("Reboot a Proxmox VE hypervisor host node with dry-run impact preview"),
			mcp.WithReadOnlyHintAnnotation(false),
			mcp.WithDestructiveHintAnnotation(true),
			mcp.WithString("node", mcp.Required(), mcp.Description("Target Proxmox node name to reboot")),
			mcp.WithBoolean("confirm", mcp.Description("Set to true after user confirmation to execute the reboot; if false/omitted, returns an impact preview")),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			if err := EnsureNodeOpsAllowed(cfg); err != nil {
				return ErrorResult(err.Error())
			}

			rawNode, err := ParseRequiredString(req.Params.Arguments, "node")
			if err != nil {
				return ErrorResult(err.Error())
			}
			node, err := ValidateNode(rawNode)
			if err != nil {
				return ErrorResult(err.Error())
			}
			if err := ValidateAllowedNode(node, cfg.AllowedNodes); err != nil {
				return ErrorResult(err.Error())
			}

			confirm := IsConfirmed(req.Params.Arguments)
			if !confirm {
				runningVMs, runningLXCs, err := getRunningGuestsOnNode(ctx, client, node)
				if err != nil {
					return ErrorResult(err.Error())
				}

				preview := map[string]any{
					"dry_run":      true,
					"action":       "reboot",
					"node":         node,
					"running_vms":  runningVMs,
					"running_lxcs": runningLXCs,
					"warning":      "Node will reboot. All running guests will experience downtime unless migrated before reboot. Quorum will be affected if other cluster nodes are offline.",
					"next_step":    "Set confirm=true to execute the node reboot.",
				}
				return JSONResult(preview)
			}

			form := url.Values{
				"command": {"reboot"},
			}
			var rawResponse json.RawMessage
			endpoint := fmt.Sprintf("/nodes/%s/status", url.PathEscape(node))
			if err := client.Post(ctx, endpoint, form, &rawResponse); err != nil {
				return ErrorResult(err.Error())
			}

			return JSONResult(map[string]any{
				"status":   "initiated",
				"command":  "reboot",
				"node":     node,
				"response": rawResponse,
			})
		},
	)

	if cfg.EnableNodeShutdown {
		s.AddTool(
			mcp.NewTool(
				"pve_node_shutdown",
				mcp.WithDescription("Shut down (cold power-off) a Proxmox VE hypervisor host node with dry-run impact preview"),
				mcp.WithReadOnlyHintAnnotation(false),
				mcp.WithDestructiveHintAnnotation(true),
				mcp.WithString("node", mcp.Required(), mcp.Description("Target Proxmox node name to shut down")),
				mcp.WithBoolean("confirm", mcp.Description("Set to true after user confirmation to execute shutdown; if false/omitted, returns an impact preview")),
			),
			func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
				if err := EnsureNodeOpsAllowed(cfg); err != nil {
					return ErrorResult(err.Error())
				}
				if err := EnsureNodeShutdownAllowed(cfg); err != nil {
					return ErrorResult(err.Error())
				}

				rawNode, err := ParseRequiredString(req.Params.Arguments, "node")
				if err != nil {
					return ErrorResult(err.Error())
				}
				node, err := ValidateNode(rawNode)
				if err != nil {
					return ErrorResult(err.Error())
				}
				if err := ValidateAllowedNode(node, cfg.AllowedNodes); err != nil {
					return ErrorResult(err.Error())
				}

				confirm := IsConfirmed(req.Params.Arguments)
				if !confirm {
					runningVMs, runningLXCs, err := getRunningGuestsOnNode(ctx, client, node)
					if err != nil {
						return ErrorResult(err.Error())
					}

					preview := map[string]any{
						"dry_run":      true,
						"action":       "shutdown",
						"node":         node,
						"running_vms":  runningVMs,
						"running_lxcs": runningLXCs,
						"warning":      "CRITICAL: Node will power off completely. Automatic recovery is impossible without out-of-band IPMI/BMC or physical button press. Quorum will be affected if other cluster nodes are offline.",
						"next_step":    "Set confirm=true to execute host shutdown.",
					}
					return JSONResult(preview)
				}

				form := url.Values{
					"command": {"shutdown"},
				}
				var rawResponse json.RawMessage
				endpoint := fmt.Sprintf("/nodes/%s/status", url.PathEscape(node))
				if err := client.Post(ctx, endpoint, form, &rawResponse); err != nil {
					return ErrorResult(err.Error())
				}

				return JSONResult(map[string]any{
					"status":   "initiated",
					"command":  "shutdown",
					"node":     node,
					"response": rawResponse,
				})
			},
		)
	}
}
