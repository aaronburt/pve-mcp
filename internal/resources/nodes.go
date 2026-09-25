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
	nodeStatusTemplate  = uritemplate.MustNew("pve://nodes/{node}/status")
	nodeVersionTemplate = uritemplate.MustNew("pve://nodes/{node}/version")
	nodeSyslogTemplate  = uritemplate.MustNew("pve://nodes/{node}/syslog")
	nodeNetworkTemplate = uritemplate.MustNew("pve://nodes/{node}/network")
	nodeTaskTemplate    = uritemplate.MustNew("pve://nodes/{node}/tasks/{+upid}")
)

func RegisterNodesResources(mcpServer *server.MCPServer, client *pve.Client) {
	mcpServer.AddResource(
		mcp.NewResource("pve://nodes", "nodes_list", mcp.WithMIMEType("application/json")),
		func(ctx context.Context, req mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
			var rawResponse json.RawMessage
			if err := client.Get(ctx, "/nodes", nil, &rawResponse); err != nil {
				return nil, err
			}
			return JSONResource(req.Params.URI, rawResponse)
		},
	)

	mcpServer.AddResourceTemplate(
		mcp.NewResourceTemplate("pve://nodes/{node}/status", "node_status", mcp.WithTemplateMIMEType("application/json")),
		func(ctx context.Context, req mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
			values := nodeStatusTemplate.Match(req.Params.URI)
			nodeName, err := tools.ValidateNode(values.Get("node").String())
			if err != nil {
				return nil, err
			}
			path := fmt.Sprintf("/nodes/%s/status", url.PathEscape(nodeName))
			var rawResponse json.RawMessage
			if err := client.Get(ctx, path, nil, &rawResponse); err != nil {
				return nil, err
			}
			return JSONResource(req.Params.URI, rawResponse)
		},
	)

	mcpServer.AddResourceTemplate(
		mcp.NewResourceTemplate("pve://nodes/{node}/version", "node_version", mcp.WithTemplateMIMEType("application/json")),
		func(ctx context.Context, req mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
			values := nodeVersionTemplate.Match(req.Params.URI)
			nodeName, err := tools.ValidateNode(values.Get("node").String())
			if err != nil {
				return nil, err
			}
			path := fmt.Sprintf("/nodes/%s/version", url.PathEscape(nodeName))
			var rawResponse json.RawMessage
			if err := client.Get(ctx, path, nil, &rawResponse); err != nil {
				return nil, err
			}
			return JSONResource(req.Params.URI, rawResponse)
		},
	)

	mcpServer.AddResourceTemplate(
		mcp.NewResourceTemplate("pve://nodes/{node}/syslog", "node_syslog", mcp.WithTemplateMIMEType("application/json")),
		func(ctx context.Context, req mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
			values := nodeSyslogTemplate.Match(req.Params.URI)
			nodeName, err := tools.ValidateNode(values.Get("node").String())
			if err != nil {
				return nil, err
			}
			path := fmt.Sprintf("/nodes/%s/syslog", url.PathEscape(nodeName))
			var rawResponse json.RawMessage
			if err := client.Get(ctx, path, nil, &rawResponse); err != nil {
				return nil, err
			}
			return JSONResource(req.Params.URI, rawResponse)
		},
	)

	mcpServer.AddResourceTemplate(
		mcp.NewResourceTemplate("pve://nodes/{node}/network", "node_network", mcp.WithTemplateMIMEType("application/json")),
		func(ctx context.Context, req mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
			values := nodeNetworkTemplate.Match(req.Params.URI)
			nodeName, err := tools.ValidateNode(values.Get("node").String())
			if err != nil {
				return nil, err
			}
			path := fmt.Sprintf("/nodes/%s/network", url.PathEscape(nodeName))
			var rawResponse json.RawMessage
			if err := client.Get(ctx, path, nil, &rawResponse); err != nil {
				return nil, err
			}
			return JSONResource(req.Params.URI, rawResponse)
		},
	)

	mcpServer.AddResourceTemplate(
		mcp.NewResourceTemplate("pve://nodes/{node}/tasks/{+upid}", "task_status", mcp.WithTemplateMIMEType("application/json")),
		func(ctx context.Context, req mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
			values := nodeTaskTemplate.Match(req.Params.URI)
			nodeName, err := tools.ValidateNode(values.Get("node").String())
			if err != nil {
				return nil, err
			}
			taskUPID := values.Get("upid").String()
			if taskUPID == "" {
				return nil, fmt.Errorf("task upid is required")
			}
			path := fmt.Sprintf("/nodes/%s/tasks/%s/status", url.PathEscape(nodeName), url.PathEscape(taskUPID))
			var rawResponse json.RawMessage
			if err := client.Get(ctx, path, nil, &rawResponse); err != nil {
				return nil, err
			}
			return JSONResource(req.Params.URI, rawResponse)
		},
	)
}
