package tools

import (
	"context"
	"encoding/json"
	"net/url"

	"github.com/aaronburt/pve-mcp/internal/pve"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func RegisterAccessTools(s *server.MCPServer, client *pve.Client) {
	s.AddTool(
		mcp.NewTool(
			"pve_access_users",
			mcp.WithDescription("List users configured in the cluster"),
			mcp.WithReadOnlyHintAnnotation(true),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			var data json.RawMessage
			if err := client.Get(ctx, "/access/users", nil, &data); err != nil {
				return ErrorResult(err.Error())
			}
			return JSONResult(data)
		},
	)

	s.AddTool(
		mcp.NewTool(
			"pve_access_groups",
			mcp.WithDescription("List user groups configured in the cluster"),
			mcp.WithReadOnlyHintAnnotation(true),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			var data json.RawMessage
			if err := client.Get(ctx, "/access/groups", nil, &data); err != nil {
				return ErrorResult(err.Error())
			}
			return JSONResult(data)
		},
	)

	s.AddTool(
		mcp.NewTool(
			"pve_access_roles",
			mcp.WithDescription("List security roles configured in the cluster"),
			mcp.WithReadOnlyHintAnnotation(true),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			var data json.RawMessage
			if err := client.Get(ctx, "/access/roles", nil, &data); err != nil {
				return ErrorResult(err.Error())
			}
			return JSONResult(data)
		},
	)

	s.AddTool(
		mcp.NewTool(
			"pve_access_domains",
			mcp.WithDescription("List authentication realms/domains configured in the cluster"),
			mcp.WithReadOnlyHintAnnotation(true),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			var data json.RawMessage
			if err := client.Get(ctx, "/access/domains", nil, &data); err != nil {
				return ErrorResult(err.Error())
			}
			return JSONResult(data)
		},
	)

	s.AddTool(
		mcp.NewTool(
			"pve_access_permissions",
			mcp.WithDescription("Get user permissions or access control lists"),
			mcp.WithReadOnlyHintAnnotation(true),
			mcp.WithString("userid", mcp.Description("Optional user ID filter")),
			mcp.WithString("path", mcp.Description("Optional ACL path filter")),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			query := url.Values{}
			if userid := ParseOptionalString(req.Params.Arguments, "userid", ""); userid != "" {
				query.Set("userid", userid)
			}
			if path := ParseOptionalString(req.Params.Arguments, "path", ""); path != "" {
				query.Set("path", path)
			}

			var data json.RawMessage
			if err := client.Get(ctx, "/access/permissions", query, &data); err != nil {
				return ErrorResult(err.Error())
			}
			return JSONResult(data)
		},
	)
}
