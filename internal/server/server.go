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

OPERATIONAL PRINCIPLES & SAFETY GUARDRAILS:

1. Two-Phase Confirmation Protocol:
   - All state-changing operations (power transitions, hardware/network changes, creation, cloning, reboot) require two-phase confirmation.
   - ALWAYS invoke the tool first with "confirm": false (or omitted) to generate a dry-run impact preview.
   - Present the structured preview to the human user and request explicit authorization.
   - NEVER call a tool with "confirm": true without prior, explicit user approval.

2. Target Discovery Before Action ("Read Before Write"):
   - Inspect cluster and guest state via MCP Resources (pve://...) or task status before planning changes.
   - Avoid guessing guest status, CPU/RAM allocations, or network bridges. Verify actual values first.

3. Target Verification & Accidental Power-Off Protection:
   - Always verify the VMID and node before invoking actions.
   - When issuing power 'stop' or 'shutdown' actions on guests, provide "expected_name" matching the guest's actual hostname to prevent accidental VMID mismatch.

4. Asynchronous Task Lifecycle:
   - Proxmox mutations (start, stop, reboot, create, clone, resize) execute asynchronously and return a UPID (Unique Process ID).
   - After dispatching an action, track completion by calling "pve_task_status" with the node and UPID until the exit status is returned.

5. Node Routing & Security Tiering:
   - Every operation requires a valid "node" argument (e.g. "pve"). Operations targeting unauthorized nodes will be rejected by policy.
   - Tools are filtered at server boot by security category flags (Power, Config, Storage, Destroy, Node Ops).
   - If a tool returns "tool not found" or Proxmox returns HTTP 403 Forbidden, that capability is omitted by the active security tier. Explain the restriction to the user rather than hallucinating workarounds.

6. Creation & Resource Formats:
   - Valid VMIDs range from 100 to 999999999.
   - Template references use "storage:vztmpl/<archive>" format (e.g. "SSD:vztmpl/alpine-3.23-default_20260116_amd64.tar.xz").
   - Storage pools for rootfs/disks must be active and capable of storing images/rootdir (e.g. "local-lvm", "SSD").`

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
