package tools

import (
	"context"
	"encoding/json"
	"net/url"
	"strconv"

	"github.com/aaronburt/pve-mcp/internal/pve"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func RegisterClusterTools(s *server.MCPServer, client *pve.Client) {
	s.AddTool(
		mcp.NewTool(
			"pve_cluster_status",
			mcp.WithDescription("Get Proxmox VE cluster status and quorum information"),
			mcp.WithReadOnlyHintAnnotation(true),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			var data json.RawMessage
			if err := client.Get(ctx, "/cluster/status", nil, &data); err != nil {
				return ErrorResult(err.Error())
			}
			return JSONResult(data)
		},
	)

	s.AddTool(
		mcp.NewTool(
			"pve_cluster_resources",
			mcp.WithDescription("Get cluster-wide resources including nodes, VMs, storage, and pools"),
			mcp.WithReadOnlyHintAnnotation(true),
			mcp.WithString("type", mcp.Description("Resource type filter (vm, storage, node, sdn)"), mcp.Enum("vm", "storage", "node", "sdn")),
			mcp.WithString("mode", mcp.Description("Output mode ('compressed' for token-efficient summary, 'full' for raw API output)"), mcp.Enum("compressed", "full")),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			query := url.Values{}
			if resType := ParseOptionalString(req.Params.Arguments, "type", ""); resType != "" {
				query.Set("type", resType)
			}

			if !IsCompressedMode(req.Params.Arguments) {
				var rawData json.RawMessage
				if err := client.Get(ctx, "/cluster/resources", query, &rawData); err != nil {
					return ErrorResult(err.Error())
				}
				return JSONResult(rawData)
			}

			var resources []pve.ClusterResource
			if err := client.Get(ctx, "/cluster/resources", query, &resources); err != nil {
				return ErrorResult(err.Error())
			}

			compressed := make([]CompressedResource, 0, len(resources))
			for _, r := range resources {
				compressed = append(compressed, CompressedResource{
					ID:     r.ID,
					Type:   r.Type,
					Name:   r.Name,
					Status: r.Status,
					Node:   r.Node,
					VMID:   r.VMID,
					CPU:    r.CPU,
					MemMB:  r.MaxMem / (1024 * 1024),
					DiskGB: r.MaxDisk / (1024 * 1024 * 1024),
				})
			}
			return JSONResult(compressed)
		},
	)

	s.AddTool(
		mcp.NewTool(
			"pve_cluster_nextid",
			mcp.WithDescription("Get the next available free VMID in the cluster"),
			mcp.WithReadOnlyHintAnnotation(true),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			var data json.RawMessage
			if err := client.Get(ctx, "/cluster/nextid", nil, &data); err != nil {
				return ErrorResult(err.Error())
			}
			return JSONResult(data)
		},
	)

	s.AddTool(
		mcp.NewTool(
			"pve_cluster_log",
			mcp.WithDescription("Read cluster log entries"),
			mcp.WithReadOnlyHintAnnotation(true),
			mcp.WithInteger("max", mcp.Description("Maximum number of log entries to return")),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			query := url.Values{}
			maxVal, err := ParseOptionalInt(req.Params.Arguments, "max", 0)
			if err != nil {
				return ErrorResult(err.Error())
			}
			if maxVal > 0 {
				query.Set("max", strconv.Itoa(maxVal))
			}
			var data json.RawMessage
			if err := client.Get(ctx, "/cluster/log", query, &data); err != nil {
				return ErrorResult(err.Error())
			}
			return JSONResult(data)
		},
	)

	s.AddTool(
		mcp.NewTool(
			"pve_cluster_ha_status",
			mcp.WithDescription("Get current High Availability (HA) cluster status"),
			mcp.WithReadOnlyHintAnnotation(true),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			var data json.RawMessage
			if err := client.Get(ctx, "/cluster/ha/status/current", nil, &data); err != nil {
				return ErrorResult(err.Error())
			}
			return JSONResult(data)
		},
	)
}
