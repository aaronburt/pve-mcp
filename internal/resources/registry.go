package resources

import (
	"encoding/json"
	"fmt"

	"github.com/aaronburt/pve-mcp/internal/pve"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func RegisterAll(mcpServer *server.MCPServer, client *pve.Client) {
	RegisterClusterResources(mcpServer, client)
	RegisterNodesResources(mcpServer, client)
	RegisterQEMUResources(mcpServer, client)
	RegisterLXCResources(mcpServer, client)
	RegisterStorageResources(mcpServer, client)
	RegisterAccessResources(mcpServer, client)
}

func JSONResource(uri string, data any) ([]mcp.ResourceContents, error) {
	formattedBytes, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal resource json: %w", err)
	}
	return []mcp.ResourceContents{
		mcp.TextResourceContents{
			URI:      uri,
			MIMEType: "application/json",
			Text:     string(formattedBytes),
		},
	}, nil
}
