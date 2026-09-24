package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"

	"github.com/aaronburt/pve-mcp/internal/pve"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func RegisterQEMUTools(s *server.MCPServer, client *pve.Client) {
	s.AddTool(
		mcp.NewTool(
			"pve_qemu_list",
			mcp.WithDescription("List all QEMU virtual machines on a node"),
			mcp.WithReadOnlyHintAnnotation(true),
			mcp.WithString("node", mcp.Required(), mcp.Description("Node name")),
			mcp.WithString("mode", mcp.Description("Output mode ('compressed' for token-efficient summary, 'full' for raw API output)"), mcp.Enum("compressed", "full")),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			rawNode, err := ParseRequiredString(req.Params.Arguments, "node")
			if err != nil {
				return ErrorResult(err.Error())
			}
			node, err := ValidateNode(rawNode)
			if err != nil {
				return ErrorResult(err.Error())
			}
			path := fmt.Sprintf("/nodes/%s/qemu", url.PathEscape(node))

			if !IsCompressedMode(req.Params.Arguments) {
				var data json.RawMessage
				if err := client.Get(ctx, path, nil, &data); err != nil {
					return ErrorResult(err.Error())
				}
				return JSONResult(data)
			}

			var vms []struct {
				VMID    int     `json:"vmid"`
				Name    string  `json:"name"`
				Status  string  `json:"status"`
				CPUs    int     `json:"cpus"`
				CPU     float64 `json:"cpu"`
				Mem     int64   `json:"mem"`
				MaxMem  int64   `json:"maxmem"`
				MaxDisk int64   `json:"maxdisk"`
				Uptime  int64   `json:"uptime"`
			}
			if err := client.Get(ctx, path, nil, &vms); err != nil {
				return ErrorResult(err.Error())
			}

			compressed := make([]CompressedVM, 0, len(vms))
			for _, v := range vms {
				compressed = append(compressed, CompressedVM{
					VMID:     v.VMID,
					Name:     v.Name,
					Status:   v.Status,
					CPUs:     v.CPUs,
					CPU:      v.CPU,
					MemMB:    v.Mem / (1024 * 1024),
					MaxMemMB: v.MaxMem / (1024 * 1024),
					DiskGB:   v.MaxDisk / (1024 * 1024 * 1024),
					Uptime:   v.Uptime,
				})
			}
			return JSONResult(compressed)
		},
	)

	s.AddTool(
		mcp.NewTool(
			"pve_qemu_status",
			mcp.WithDescription("Get the current status of a QEMU virtual machine"),
			mcp.WithReadOnlyHintAnnotation(true),
			mcp.WithString("node", mcp.Required(), mcp.Description("Node name")),
			mcp.WithInteger("vmid", mcp.Required(), mcp.Description("Virtual Machine ID")),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			rawNode, err := ParseRequiredString(req.Params.Arguments, "node")
			if err != nil {
				return ErrorResult(err.Error())
			}
			node, err := ValidateNode(rawNode)
			if err != nil {
				return ErrorResult(err.Error())
			}

			rawVMID, err := ParseRequiredInt(req.Params.Arguments, "vmid")
			if err != nil {
				return ErrorResult(err.Error())
			}
			vmid, err := ValidateVMID(rawVMID)
			if err != nil {
				return ErrorResult(err.Error())
			}

			path := fmt.Sprintf("/nodes/%s/qemu/%d/status/current", url.PathEscape(node), vmid)
			var data json.RawMessage
			if err := client.Get(ctx, path, nil, &data); err != nil {
				return ErrorResult(err.Error())
			}
			return JSONResult(data)
		},
	)

	s.AddTool(
		mcp.NewTool(
			"pve_qemu_config",
			mcp.WithDescription("Get the configuration parameters of a QEMU virtual machine"),
			mcp.WithReadOnlyHintAnnotation(true),
			mcp.WithString("node", mcp.Required(), mcp.Description("Node name")),
			mcp.WithInteger("vmid", mcp.Required(), mcp.Description("Virtual Machine ID")),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			rawNode, err := ParseRequiredString(req.Params.Arguments, "node")
			if err != nil {
				return ErrorResult(err.Error())
			}
			node, err := ValidateNode(rawNode)
			if err != nil {
				return ErrorResult(err.Error())
			}

			rawVMID, err := ParseRequiredInt(req.Params.Arguments, "vmid")
			if err != nil {
				return ErrorResult(err.Error())
			}
			vmid, err := ValidateVMID(rawVMID)
			if err != nil {
				return ErrorResult(err.Error())
			}

			path := fmt.Sprintf("/nodes/%s/qemu/%d/config", url.PathEscape(node), vmid)
			var data json.RawMessage
			if err := client.Get(ctx, path, nil, &data); err != nil {
				return ErrorResult(err.Error())
			}
			return JSONResult(data)
		},
	)

	s.AddTool(
		mcp.NewTool(
			"pve_qemu_snapshots",
			mcp.WithDescription("List snapshots of a QEMU virtual machine"),
			mcp.WithReadOnlyHintAnnotation(true),
			mcp.WithString("node", mcp.Required(), mcp.Description("Node name")),
			mcp.WithInteger("vmid", mcp.Required(), mcp.Description("Virtual Machine ID")),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			rawNode, err := ParseRequiredString(req.Params.Arguments, "node")
			if err != nil {
				return ErrorResult(err.Error())
			}
			node, err := ValidateNode(rawNode)
			if err != nil {
				return ErrorResult(err.Error())
			}

			rawVMID, err := ParseRequiredInt(req.Params.Arguments, "vmid")
			if err != nil {
				return ErrorResult(err.Error())
			}
			vmid, err := ValidateVMID(rawVMID)
			if err != nil {
				return ErrorResult(err.Error())
			}

			path := fmt.Sprintf("/nodes/%s/qemu/%d/snapshot", url.PathEscape(node), vmid)
			var data json.RawMessage
			if err := client.Get(ctx, path, nil, &data); err != nil {
				return ErrorResult(err.Error())
			}
			return JSONResult(data)
		},
	)

	s.AddTool(
		mcp.NewTool(
			"pve_qemu_firewall",
			mcp.WithDescription("Get firewall rules for a QEMU virtual machine"),
			mcp.WithReadOnlyHintAnnotation(true),
			mcp.WithString("node", mcp.Required(), mcp.Description("Node name")),
			mcp.WithInteger("vmid", mcp.Required(), mcp.Description("Virtual Machine ID")),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			rawNode, err := ParseRequiredString(req.Params.Arguments, "node")
			if err != nil {
				return ErrorResult(err.Error())
			}
			node, err := ValidateNode(rawNode)
			if err != nil {
				return ErrorResult(err.Error())
			}

			rawVMID, err := ParseRequiredInt(req.Params.Arguments, "vmid")
			if err != nil {
				return ErrorResult(err.Error())
			}
			vmid, err := ValidateVMID(rawVMID)
			if err != nil {
				return ErrorResult(err.Error())
			}

			path := fmt.Sprintf("/nodes/%s/qemu/%d/firewall/rules", url.PathEscape(node), vmid)
			var data json.RawMessage
			if err := client.Get(ctx, path, nil, &data); err != nil {
				return ErrorResult(err.Error())
			}
			return JSONResult(data)
		},
	)
}
