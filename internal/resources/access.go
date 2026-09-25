package resources

import (
	"context"
	"encoding/json"

	"github.com/aaronburt/pve-mcp/internal/pve"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func RegisterAccessResources(mcpServer *server.MCPServer, client *pve.Client) {
	mcpServer.AddResource(
		mcp.NewResource("pve://access/users", "access_users", mcp.WithMIMEType("application/json")),
		func(ctx context.Context, req mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
			var rawResponse json.RawMessage
			if err := client.Get(ctx, "/access/users", nil, &rawResponse); err != nil {
				return nil, err
			}
			return JSONResource(req.Params.URI, rawResponse)
		},
	)

	mcpServer.AddResource(
		mcp.NewResource("pve://access/groups", "access_groups", mcp.WithMIMEType("application/json")),
		func(ctx context.Context, req mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
			var rawResponse json.RawMessage
			if err := client.Get(ctx, "/access/groups", nil, &rawResponse); err != nil {
				return nil, err
			}
			return JSONResource(req.Params.URI, rawResponse)
		},
	)

	mcpServer.AddResource(
		mcp.NewResource("pve://access/roles", "access_roles", mcp.WithMIMEType("application/json")),
		func(ctx context.Context, req mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
			var rawResponse json.RawMessage
			if err := client.Get(ctx, "/access/roles", nil, &rawResponse); err != nil {
				return nil, err
			}
			return JSONResource(req.Params.URI, rawResponse)
		},
	)

	mcpServer.AddResource(
		mcp.NewResource("pve://access/domains", "access_domains", mcp.WithMIMEType("application/json")),
		func(ctx context.Context, req mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
			var rawResponse json.RawMessage
			if err := client.Get(ctx, "/access/domains", nil, &rawResponse); err != nil {
				return nil, err
			}
			return JSONResource(req.Params.URI, rawResponse)
		},
	)

	mcpServer.AddResource(
		mcp.NewResource("pve://access/permissions", "access_permissions", mcp.WithMIMEType("application/json")),
		func(ctx context.Context, req mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
			var rawResponse json.RawMessage
			if err := client.Get(ctx, "/access/permissions", nil, &rawResponse); err != nil {
				return nil, err
			}
			return JSONResource(req.Params.URI, rawResponse)
		},
	)
}
