package tools

import (
	"github.com/aaronburt/pve-mcp/internal/pve"
	"github.com/mark3labs/mcp-go/server"
)

func RegisterAll(s *server.MCPServer, client *pve.Client) {
	RegisterClusterTools(s, client)
	RegisterNodesTools(s, client)
	RegisterQEMUTools(s, client)
	RegisterLXCTools(s, client)
	RegisterStorageTools(s, client)
	RegisterNetworkTools(s, client)
	RegisterAccessTools(s, client)
}
