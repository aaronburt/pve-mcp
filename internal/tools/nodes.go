package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"

	"github.com/aaronburt/pve-mcp/internal/pve"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func RegisterNodesTools(s *server.MCPServer, client *pve.Client) {
	s.AddTool(
		mcp.NewTool(
			"pve_nodes_list",
			mcp.WithDescription("List all cluster nodes and their general status"),
			mcp.WithReadOnlyHintAnnotation(true),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			var data json.RawMessage
			if err := client.Get(ctx, "/nodes", nil, &data); err != nil {
				return ErrorResult(err.Error())
			}
			return JSONResult(data)
		},
	)

	s.AddTool(
		mcp.NewTool(
			"pve_node_status",
			mcp.WithDescription("Get detailed status information for a specific node"),
			mcp.WithReadOnlyHintAnnotation(true),
			mcp.WithString("node", mcp.Required(), mcp.Description("Node name")),
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
			path := fmt.Sprintf("/nodes/%s/status", url.PathEscape(node))
			var data json.RawMessage
			if err := client.Get(ctx, path, nil, &data); err != nil {
				return ErrorResult(err.Error())
			}
			return JSONResult(data)
		},
	)

	s.AddTool(
		mcp.NewTool(
			"pve_node_version",
			mcp.WithDescription("Get version information for packages installed on a specific node"),
			mcp.WithReadOnlyHintAnnotation(true),
			mcp.WithString("node", mcp.Required(), mcp.Description("Node name")),
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
			path := fmt.Sprintf("/nodes/%s/version", url.PathEscape(node))
			var data json.RawMessage
			if err := client.Get(ctx, path, nil, &data); err != nil {
				return ErrorResult(err.Error())
			}
			return JSONResult(data)
		},
	)

	s.AddTool(
		mcp.NewTool(
			"pve_node_syslog",
			mcp.WithDescription("Read system log of a specific node"),
			mcp.WithReadOnlyHintAnnotation(true),
			mcp.WithString("node", mcp.Required(), mcp.Description("Node name")),
			mcp.WithInteger("limit", mcp.Description("Maximum number of lines to return")),
			mcp.WithString("since", mcp.Description("Filter log entries since date/time")),
			mcp.WithString("until", mcp.Description("Filter log entries until date/time")),
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

			query := url.Values{}
			limit, err := ParseOptionalInt(req.Params.Arguments, "limit", 50)
			if err != nil {
				return ErrorResult(err.Error())
			}
			if limit > 0 {
				query.Set("limit", strconv.Itoa(limit))
			}
			if since := ParseOptionalString(req.Params.Arguments, "since", ""); since != "" {
				query.Set("since", since)
			}
			if until := ParseOptionalString(req.Params.Arguments, "until", ""); until != "" {
				query.Set("until", until)
			}

			path := fmt.Sprintf("/nodes/%s/syslog", url.PathEscape(node))
			var data json.RawMessage
			if err := client.Get(ctx, path, query, &data); err != nil {
				return ErrorResult(err.Error())
			}
			return JSONResult(data)
		},
	)

	s.AddTool(
		mcp.NewTool(
			"pve_node_rrddata",
			mcp.WithDescription("Read node RRD performance data"),
			mcp.WithReadOnlyHintAnnotation(true),
			mcp.WithString("node", mcp.Required(), mcp.Description("Node name")),
			mcp.WithString("timeframe", mcp.Required(), mcp.Description("Timeframe (hour, day, week, month, year)"), mcp.Enum("hour", "day", "week", "month", "year")),
			mcp.WithString("cf", mcp.Description("Consolidation function (AVERAGE, MAX)"), mcp.Enum("AVERAGE", "MAX")),
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

			timeframe, err := ParseRequiredString(req.Params.Arguments, "timeframe")
			if err != nil {
				return ErrorResult(err.Error())
			}

			query := url.Values{}
			query.Set("timeframe", timeframe)
			if cf := ParseOptionalString(req.Params.Arguments, "cf", ""); cf != "" {
				query.Set("cf", cf)
			}

			path := fmt.Sprintf("/nodes/%s/rrddata", url.PathEscape(node))
			var data json.RawMessage
			if err := client.Get(ctx, path, query, &data); err != nil {
				return ErrorResult(err.Error())
			}
			return JSONResult(data)
		},
	)
}
