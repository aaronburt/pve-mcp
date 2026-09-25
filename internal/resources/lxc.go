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
	lxcListTemplate      = uritemplate.MustNew("pve://nodes/{node}/lxc")
	lxcStatusTemplate    = uritemplate.MustNew("pve://nodes/{node}/lxc/{vmid}/status")
	lxcConfigTemplate    = uritemplate.MustNew("pve://nodes/{node}/lxc/{vmid}/config")
	lxcSnapshotsTemplate = uritemplate.MustNew("pve://nodes/{node}/lxc/{vmid}/snapshots")
	lxcFirewallTemplate  = uritemplate.MustNew("pve://nodes/{node}/lxc/{vmid}/firewall")
)

func RegisterLXCResources(mcpServer *server.MCPServer, client *pve.Client) {
	mcpServer.AddResourceTemplate(
		mcp.NewResourceTemplate("pve://nodes/{node}/lxc", "lxc_list", mcp.WithTemplateMIMEType("application/json")),
		func(ctx context.Context, req mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
			values := lxcListTemplate.Match(req.Params.URI)
			nodeName, err := tools.ValidateNode(values.Get("node").String())
			if err != nil {
				return nil, err
			}
			path := fmt.Sprintf("/nodes/%s/lxc", url.PathEscape(nodeName))
			var rawResponse json.RawMessage
			if err := client.Get(ctx, path, nil, &rawResponse); err != nil {
				return nil, err
			}
			return JSONResource(req.Params.URI, rawResponse)
		},
	)

	mcpServer.AddResourceTemplate(
		mcp.NewResourceTemplate("pve://nodes/{node}/lxc/{vmid}/status", "lxc_status", mcp.WithTemplateMIMEType("application/json")),
		func(ctx context.Context, req mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
			values := lxcStatusTemplate.Match(req.Params.URI)
			nodeName, vmid, err := parseNodeAndVMID(values)
			if err != nil {
				return nil, err
			}
			path := fmt.Sprintf("/nodes/%s/lxc/%d/status/current", url.PathEscape(nodeName), vmid)
			var rawResponse json.RawMessage
			if err := client.Get(ctx, path, nil, &rawResponse); err != nil {
				return nil, err
			}
			return JSONResource(req.Params.URI, rawResponse)
		},
	)

	mcpServer.AddResourceTemplate(
		mcp.NewResourceTemplate("pve://nodes/{node}/lxc/{vmid}/config", "lxc_config", mcp.WithTemplateMIMEType("application/json")),
		func(ctx context.Context, req mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
			values := lxcConfigTemplate.Match(req.Params.URI)
			nodeName, vmid, err := parseNodeAndVMID(values)
			if err != nil {
				return nil, err
			}
			path := fmt.Sprintf("/nodes/%s/lxc/%d/config", url.PathEscape(nodeName), vmid)
			var rawResponse json.RawMessage
			if err := client.Get(ctx, path, nil, &rawResponse); err != nil {
				return nil, err
			}
			return JSONResource(req.Params.URI, rawResponse)
		},
	)

	mcpServer.AddResourceTemplate(
		mcp.NewResourceTemplate("pve://nodes/{node}/lxc/{vmid}/snapshots", "lxc_snapshots", mcp.WithTemplateMIMEType("application/json")),
		func(ctx context.Context, req mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
			values := lxcSnapshotsTemplate.Match(req.Params.URI)
			nodeName, vmid, err := parseNodeAndVMID(values)
			if err != nil {
				return nil, err
			}
			path := fmt.Sprintf("/nodes/%s/lxc/%d/snapshot", url.PathEscape(nodeName), vmid)
			var rawResponse json.RawMessage
			if err := client.Get(ctx, path, nil, &rawResponse); err != nil {
				return nil, err
			}
			return JSONResource(req.Params.URI, rawResponse)
		},
	)

	mcpServer.AddResourceTemplate(
		mcp.NewResourceTemplate("pve://nodes/{node}/lxc/{vmid}/firewall", "lxc_firewall", mcp.WithTemplateMIMEType("application/json")),
		func(ctx context.Context, req mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
			values := lxcFirewallTemplate.Match(req.Params.URI)
			nodeName, vmid, err := parseNodeAndVMID(values)
			if err != nil {
				return nil, err
			}
			path := fmt.Sprintf("/nodes/%s/lxc/%d/firewall/rules", url.PathEscape(nodeName), vmid)
			var rawResponse json.RawMessage
			if err := client.Get(ctx, path, nil, &rawResponse); err != nil {
				return nil, err
			}
			return JSONResource(req.Params.URI, rawResponse)
		},
	)
}
