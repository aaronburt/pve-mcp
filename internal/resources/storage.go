package resources

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"

	"github.com/aaronburt/pve-mcp/internal/pve"
	"github.com/aaronburt/pve-mcp/internal/tools"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/yosida95/uritemplate/v3"
)

var (
	storageStatusTemplate  = uritemplate.MustNew("pve://nodes/{node}/storage/{storage}/status")
	storageContentTemplate = uritemplate.MustNew("pve://nodes/{node}/storage/{storage}/content")
)

func parseNodeAndStorage(values uritemplate.Values) (string, string, error) {
	nodeName, err := tools.ValidateNode(values.Get("node").String())
	if err != nil {
		return "", "", err
	}
	storageName, err := tools.ValidateStorage(values.Get("storage").String())
	if err != nil {
		return "", "", err
	}
	return nodeName, storageName, nil
}

func RegisterStorageResources(mcpServer *server.MCPServer, client *pve.Client) {
	mcpServer.AddResource(
		mcp.NewResource("pve://storage", "storage_list", mcp.WithMIMEType("application/json")),
		func(ctx context.Context, req mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
			var rawResponse json.RawMessage
			if err := client.Get(ctx, "/storage", nil, &rawResponse); err != nil {
				return nil, err
			}
			return JSONResource(req.Params.URI, rawResponse)
		},
	)

	mcpServer.AddResourceTemplate(
		mcp.NewResourceTemplate("pve://nodes/{node}/storage/{storage}/status", "storage_status", mcp.WithTemplateMIMEType("application/json")),
		func(ctx context.Context, req mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
			values := storageStatusTemplate.Match(req.Params.URI)
			nodeName, storageName, err := parseNodeAndStorage(values)
			if err != nil {
				return nil, err
			}
			path := fmt.Sprintf("/nodes/%s/storage/%s/status", url.PathEscape(nodeName), url.PathEscape(storageName))
			var rawResponse json.RawMessage
			if err := client.Get(ctx, path, nil, &rawResponse); err != nil {
				return nil, err
			}
			return JSONResource(req.Params.URI, rawResponse)
		},
	)

	mcpServer.AddResourceTemplate(
		mcp.NewResourceTemplate("pve://nodes/{node}/storage/{storage}/content", "storage_content", mcp.WithTemplateMIMEType("application/json")),
		func(ctx context.Context, req mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
			values := storageContentTemplate.Match(req.Params.URI)
			nodeName, storageName, err := parseNodeAndStorage(values)
			if err != nil {
				return nil, err
			}
			path := fmt.Sprintf("/nodes/%s/storage/%s/content", url.PathEscape(nodeName), url.PathEscape(storageName))
			var rawResponse json.RawMessage
			if err := client.Get(ctx, path, nil, &rawResponse); err != nil {
				return nil, err
			}
			return JSONResource(req.Params.URI, rawResponse)
		},
	)
}
