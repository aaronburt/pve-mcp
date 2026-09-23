package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"

	"github.com/aaronburt/pve-mcp/internal/pve"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func RegisterNetworkTools(s *server.MCPServer, client *pve.Client) {
	s.AddTool(
		mcp.NewTool(
			"pve_network_list",
			mcp.WithDescription("List network interfaces on a node"),
			mcp.WithReadOnlyHintAnnotation(true),
			mcp.WithString("node", mcp.Required(), mcp.Description("Node name")),
			mcp.WithString("type", mcp.Description("Filter by interface type (bridge, bond, eth, alias, vlan, OVSBridge, etc.)")),
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
			if ifaceType := ParseOptionalString(req.Params.Arguments, "type", ""); ifaceType != "" {
				query.Set("type", ifaceType)
			}

			path := fmt.Sprintf("/nodes/%s/network", url.PathEscape(node))
			var data json.RawMessage
			if err := client.Get(ctx, path, query, &data); err != nil {
				return ErrorResult(err.Error())
			}
			return JSONResult(data)
		},
	)
}
