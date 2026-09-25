package server

import (
	"context"
	"net"
	"net/http"
	"time"

	"github.com/aaronburt/pve-mcp/internal/config"
	"github.com/aaronburt/pve-mcp/internal/prompts"
	"github.com/aaronburt/pve-mcp/internal/pve"
	"github.com/aaronburt/pve-mcp/internal/resources"
	"github.com/aaronburt/pve-mcp/internal/tools"
	mcpserver "github.com/mark3labs/mcp-go/server"
)

const Instructions = `Proxmox VE (PVE) Model Context Protocol Server.

Guiding Principles:
1. Resources are nouns: Read cluster state, node status, guest configurations, and metrics via MCP Resources (pve://...).
2. Tools are verbs: Execute mutations and state changes via MCP Tools (pve_qemu_*, pve_lxc_*, pve_task_status).
3. Two-phase confirmation: Destructive or state-changing operations require confirm=true. Without confirm=true, a dry-run preview is returned.
4. Asynchronous Task Tracking: Async operations return a UPID. Track completion using pve_task_status or subscribe to progress notifications.
5. High-level workflows: Use MCP Prompts for guided operational procedures (audits, triage, evacuations, provisioning, cleanup).`

type Server struct {
	httpServer       *http.Server
	sseServer        *mcpserver.SSEServer
	streamableServer *mcpserver.StreamableHTTPServer
	cfg              *config.Config
}

func CreateMCPServer(cfg *config.Config, client *pve.Client) *mcpserver.MCPServer {
	mcpServer := mcpserver.NewMCPServer(
		"pve-mcp",
		"1.1.0",
		mcpserver.WithRecovery(),
		mcpserver.WithInstructions(Instructions),
	)
	tools.RegisterAll(mcpServer, client, cfg)
	resources.RegisterAll(mcpServer, client)
	prompts.RegisterAll(mcpServer)
	return mcpServer
}

func NewServer(cfg *config.Config, client *pve.Client) (*Server, error) {
	mcpServer := CreateMCPServer(cfg, client)

	sseServer := mcpserver.NewSSEServer(mcpServer)
	streamableServer := mcpserver.NewStreamableHTTPServer(mcpServer, mcpserver.WithStateLess(true))

	sseGetHandler := sseServer.SSEHandler()
	combinedSSEHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			sseGetHandler.ServeHTTP(w, r)
			return
		}
		streamableServer.ServeHTTP(w, r)
	})

	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	})
	mux.Handle("/sse", combinedSSEHandler)
	mux.Handle("/message", sseServer.MessageHandler())
	mux.Handle("/mcp", streamableServer)
	mux.Handle("/", combinedSSEHandler)

	handler := RequestLoggerMiddleware(OriginMiddleware(cfg, AuthMiddleware(cfg, mux)))

	addr := net.JoinHostPort(cfg.BindAddress, cfg.Port)
	httpServer := &http.Server{
		Addr:              addr,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	return &Server{
		httpServer:       httpServer,
		sseServer:        sseServer,
		streamableServer: streamableServer,
		cfg:              cfg,
	}, nil
}

func (s *Server) Start() error {
	return s.httpServer.ListenAndServe()
}

func (s *Server) Shutdown(ctx context.Context) error {
	_ = s.sseServer.Shutdown(ctx)
	_ = s.streamableServer.Shutdown(ctx)
	return s.httpServer.Shutdown(ctx)
}

func (s *Server) Addr() string {
	return s.httpServer.Addr
}

func (s *Server) Handler() http.Handler {
	return s.httpServer.Handler
}
