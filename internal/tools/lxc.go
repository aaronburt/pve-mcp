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

func RegisterLXCTools(s *server.MCPServer, client *pve.Client) {
	s.AddTool(
		mcp.NewTool(
			"pve_lxc_list",
			mcp.WithDescription("List all LXC containers on a node"),
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
			path := fmt.Sprintf("/nodes/%s/lxc", url.PathEscape(node))

			if !IsCompressedMode(req.Params.Arguments) {
				var data json.RawMessage
				if err := client.Get(ctx, path, nil, &data); err != nil {
					return ErrorResult(err.Error())
				}
				return JSONResult(data)
			}

			var lxcs []struct {
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
			if err := client.Get(ctx, path, nil, &lxcs); err != nil {
				return ErrorResult(err.Error())
			}

			compressed := make([]CompressedVM, 0, len(lxcs))
			for _, c := range lxcs {
				compressed = append(compressed, CompressedVM{
					VMID:     c.VMID,
					Name:     c.Name,
					Status:   c.Status,
					CPUs:     c.CPUs,
					CPU:      c.CPU,
					MemMB:    c.Mem / (1024 * 1024),
					MaxMemMB: c.MaxMem / (1024 * 1024),
					DiskGB:   c.MaxDisk / (1024 * 1024 * 1024),
					Uptime:   c.Uptime,
				})
			}
			return JSONResult(compressed)
		},
	)

	s.AddTool(
		mcp.NewTool(
			"pve_lxc_status",
			mcp.WithDescription("Get the current status of an LXC container"),
			mcp.WithReadOnlyHintAnnotation(true),
			mcp.WithString("node", mcp.Required(), mcp.Description("Node name")),
			mcp.WithInteger("vmid", mcp.Required(), mcp.Description("Container ID")),
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

			path := fmt.Sprintf("/nodes/%s/lxc/%d/status/current", url.PathEscape(node), vmid)
			var data json.RawMessage
			if err := client.Get(ctx, path, nil, &data); err != nil {
				return ErrorResult(err.Error())
			}
			return JSONResult(data)
		},
	)

	s.AddTool(
		mcp.NewTool(
			"pve_lxc_config",
			mcp.WithDescription("Get the configuration parameters of an LXC container"),
			mcp.WithReadOnlyHintAnnotation(true),
			mcp.WithString("node", mcp.Required(), mcp.Description("Node name")),
			mcp.WithInteger("vmid", mcp.Required(), mcp.Description("Container ID")),
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

			path := fmt.Sprintf("/nodes/%s/lxc/%d/config", url.PathEscape(node), vmid)
			var data json.RawMessage
			if err := client.Get(ctx, path, nil, &data); err != nil {
				return ErrorResult(err.Error())
			}
			return JSONResult(data)
		},
	)

	s.AddTool(
		mcp.NewTool(
			"pve_lxc_snapshots",
			mcp.WithDescription("List snapshots of an LXC container"),
			mcp.WithReadOnlyHintAnnotation(true),
			mcp.WithString("node", mcp.Required(), mcp.Description("Node name")),
			mcp.WithInteger("vmid", mcp.Required(), mcp.Description("Container ID")),
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

			path := fmt.Sprintf("/nodes/%s/lxc/%d/snapshot", url.PathEscape(node), vmid)
			var data json.RawMessage
			if err := client.Get(ctx, path, nil, &data); err != nil {
				return ErrorResult(err.Error())
			}
			return JSONResult(data)
		},
	)

	s.AddTool(
		mcp.NewTool(
			"pve_lxc_firewall",
			mcp.WithDescription("Get firewall rules for an LXC container"),
			mcp.WithReadOnlyHintAnnotation(true),
			mcp.WithString("node", mcp.Required(), mcp.Description("Node name")),
			mcp.WithInteger("vmid", mcp.Required(), mcp.Description("Container ID")),
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

			path := fmt.Sprintf("/nodes/%s/lxc/%d/firewall/rules", url.PathEscape(node), vmid)
			var data json.RawMessage
			if err := client.Get(ctx, path, nil, &data); err != nil {
				return ErrorResult(err.Error())
			}
			return JSONResult(data)
		},
	)
}
