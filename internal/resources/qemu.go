package resources

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"

	"github.com/aaronburt/pve-mcp/internal/pve"
	"github.com/aaronburt/pve-mcp/internal/tools"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/yosida95/uritemplate/v3"
)

var (
	qemuListTemplate      = uritemplate.MustNew("pve://nodes/{node}/qemu")
	qemuStatusTemplate    = uritemplate.MustNew("pve://nodes/{node}/qemu/{vmid}/status")
	qemuConfigTemplate    = uritemplate.MustNew("pve://nodes/{node}/qemu/{vmid}/config")
	qemuSnapshotsTemplate = uritemplate.MustNew("pve://nodes/{node}/qemu/{vmid}/snapshots")
	qemuFirewallTemplate  = uritemplate.MustNew("pve://nodes/{node}/qemu/{vmid}/firewall")
)

func parseNodeAndVMID(values uritemplate.Values) (string, int, error) {
	nodeName, err := tools.ValidateNode(values.Get("node").String())
	if err != nil {
		return "", 0, err
	}
	rawVMID := values.Get("vmid").String()
	vmidInt, err := strconv.Atoi(rawVMID)
	if err != nil {
		return "", 0, fmt.Errorf("invalid vmid: %q", rawVMID)
	}
	validatedVMID, err := tools.ValidateVMID(vmidInt)
	if err != nil {
		return "", 0, err
	}
	return nodeName, validatedVMID, nil
}

func RegisterQEMUResources(mcpServer *server.MCPServer, client *pve.Client) {
	mcpServer.AddResourceTemplate(
		mcp.NewResourceTemplate("pve://nodes/{node}/qemu", "qemu_list", mcp.WithTemplateMIMEType("application/json")),
		func(ctx context.Context, req mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
			values := qemuListTemplate.Match(req.Params.URI)
			nodeName, err := tools.ValidateNode(values.Get("node").String())
			if err != nil {
				return nil, err
			}
			path := fmt.Sprintf("/nodes/%s/qemu", url.PathEscape(nodeName))
			var rawResponse json.RawMessage
			if err := client.Get(ctx, path, nil, &rawResponse); err != nil {
				return nil, err
			}
			return JSONResource(req.Params.URI, rawResponse)
		},
	)

	mcpServer.AddResourceTemplate(
		mcp.NewResourceTemplate("pve://nodes/{node}/qemu/{vmid}/status", "qemu_status", mcp.WithTemplateMIMEType("application/json")),
		func(ctx context.Context, req mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
			values := qemuStatusTemplate.Match(req.Params.URI)
			nodeName, vmid, err := parseNodeAndVMID(values)
			if err != nil {
				return nil, err
			}
			path := fmt.Sprintf("/nodes/%s/qemu/%d/status/current", url.PathEscape(nodeName), vmid)
			var rawResponse json.RawMessage
			if err := client.Get(ctx, path, nil, &rawResponse); err != nil {
				return nil, err
			}
			return JSONResource(req.Params.URI, rawResponse)
		},
	)

	mcpServer.AddResourceTemplate(
		mcp.NewResourceTemplate("pve://nodes/{node}/qemu/{vmid}/config", "qemu_config", mcp.WithTemplateMIMEType("application/json")),
		func(ctx context.Context, req mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
			values := qemuConfigTemplate.Match(req.Params.URI)
			nodeName, vmid, err := parseNodeAndVMID(values)
			if err != nil {
				return nil, err
			}
			path := fmt.Sprintf("/nodes/%s/qemu/%d/config", url.PathEscape(nodeName), vmid)
			var rawResponse json.RawMessage
			if err := client.Get(ctx, path, nil, &rawResponse); err != nil {
				return nil, err
			}
			return JSONResource(req.Params.URI, rawResponse)
		},
	)

	mcpServer.AddResourceTemplate(
		mcp.NewResourceTemplate("pve://nodes/{node}/qemu/{vmid}/snapshots", "qemu_snapshots", mcp.WithTemplateMIMEType("application/json")),
		func(ctx context.Context, req mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
			values := qemuSnapshotsTemplate.Match(req.Params.URI)
			nodeName, vmid, err := parseNodeAndVMID(values)
			if err != nil {
				return nil, err
			}
			path := fmt.Sprintf("/nodes/%s/qemu/%d/snapshot", url.PathEscape(nodeName), vmid)
			var rawResponse json.RawMessage
			if err := client.Get(ctx, path, nil, &rawResponse); err != nil {
				return nil, err
			}
			return JSONResource(req.Params.URI, rawResponse)
		},
	)

	mcpServer.AddResourceTemplate(
		mcp.NewResourceTemplate("pve://nodes/{node}/qemu/{vmid}/firewall", "qemu_firewall", mcp.WithTemplateMIMEType("application/json")),
		func(ctx context.Context, req mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
			values := qemuFirewallTemplate.Match(req.Params.URI)
			nodeName, vmid, err := parseNodeAndVMID(values)
			if err != nil {
				return nil, err
			}
			path := fmt.Sprintf("/nodes/%s/qemu/%d/firewall/rules", url.PathEscape(nodeName), vmid)
			var rawResponse json.RawMessage
			if err := client.Get(ctx, path, nil, &rawResponse); err != nil {
				return nil, err
			}
			return JSONResource(req.Params.URI, rawResponse)
		},
	)
}
