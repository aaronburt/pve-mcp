package tools

import (
	"github.com/aaronburt/pve-mcp/internal/config"
	"github.com/aaronburt/pve-mcp/internal/pve"
	"github.com/mark3labs/mcp-go/server"
)

func RegisterAll(s *server.MCPServer, client *pve.Client, cfg *config.Config) {
	RegisterClusterTools(s, client)
	RegisterQEMUTools(s, client, cfg)
	RegisterLXCTools(s, client, cfg)
}
