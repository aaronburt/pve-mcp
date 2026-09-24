package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"

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
			mcp.WithString("mode", mcp.Description("Output mode ('compact' for tabular text, 'full' for raw API output)"), mcp.Enum("compact", "full")),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			query := url.Values{}
			if resType := ParseOptionalString(req.Params.Arguments, "type", ""); resType != "" {
				query.Set("type", resType)
			}

			if !IsCompactMode(req.Params.Arguments) {
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

			var sb strings.Builder
			sb.WriteString("ID\tTYPE\tNAME\tSTATUS\tNODE\tVMID\tCPU\tRAM_MB\tDISK_GB\n")
			for _, r := range resources {
				fmt.Fprintf(&sb, "%s\t%s\t%s\t%s\t%s\t%d\t%.2f\t%d\t%d\n",
					r.ID, r.Type, r.Name, r.Status, r.Node, r.VMID, r.CPU,
					r.MaxMem/(1024*1024), r.MaxDisk/(1024*1024*1024),
				)
			}
			return mcp.NewToolResultText(sb.String()), nil
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
