package resources

import (
	"context"
	"encoding/json"

	"github.com/aaronburt/pve-mcp/internal/pve"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func RegisterClusterResources(mcpServer *server.MCPServer, client *pve.Client) {
	mcpServer.AddResource(
		mcp.NewResource("pve://cluster/status", "cluster_status", mcp.WithMIMEType("application/json")),
		func(ctx context.Context, req mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
			var rawResponse json.RawMessage
			if err := client.Get(ctx, "/cluster/status", nil, &rawResponse); err != nil {
				return nil, err
			}
			return JSONResource(req.Params.URI, rawResponse)
		},
	)

	mcpServer.AddResource(
		mcp.NewResource("pve://cluster/resources", "cluster_resources", mcp.WithMIMEType("application/json")),
		func(ctx context.Context, req mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
			var rawResponse json.RawMessage
			if err := client.Get(ctx, "/cluster/resources", nil, &rawResponse); err != nil {
				return nil, err
			}
			return JSONResource(req.Params.URI, rawResponse)
		},
	)

	mcpServer.AddResource(
		mcp.NewResource("pve://cluster/ha-status", "cluster_ha_status", mcp.WithMIMEType("application/json")),
		func(ctx context.Context, req mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
			var rawResponse json.RawMessage
			if err := client.Get(ctx, "/cluster/ha/status/current", nil, &rawResponse); err != nil {
				return nil, err
			}
			return JSONResource(req.Params.URI, rawResponse)
		},
	)

	mcpServer.AddResource(
		mcp.NewResource("pve://cluster/nextid", "cluster_nextid", mcp.WithMIMEType("application/json")),
		func(ctx context.Context, req mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
			var rawResponse json.RawMessage
			if err := client.Get(ctx, "/cluster/nextid", nil, &rawResponse); err != nil {
				return nil, err
			}
			return JSONResource(req.Params.URI, rawResponse)
		},
	)

	mcpServer.AddResource(
		mcp.NewResource("pve://cluster/log", "cluster_log", mcp.WithMIMEType("application/json")),
		func(ctx context.Context, req mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
			var rawResponse json.RawMessage
			if err := client.Get(ctx, "/cluster/log", nil, &rawResponse); err != nil {
				return nil, err
			}
			return JSONResource(req.Params.URI, rawResponse)
		},
	)
}
