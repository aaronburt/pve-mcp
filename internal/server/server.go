package server

import (
	"context"
	"net"
	"net/http"
	"time"

	"github.com/aaronburt/pve-mcp-go/internal/config"
	"github.com/aaronburt/pve-mcp-go/internal/prompts"
	"github.com/aaronburt/pve-mcp-go/internal/pve"
	"github.com/aaronburt/pve-mcp-go/internal/resources"
	"github.com/aaronburt/pve-mcp-go/internal/tools"
	"github.com/aaronburt/pve-mcp-go/internal/webui"
	mcpserver "github.com/mark3labs/mcp-go/server"
)

const Instructions = `Proxmox VE (PVE) Model Context Protocol Server.

MANDATORY OPERATIONAL DIRECTIVES FOR THE MODEL:

1. Precision & Zero-Guessing Rule ("Read Before Write"):
   - You must be strictly precise on all calls. NEVER guess, estimate, or assume VMIDs, guest hostnames, node names, storage pools, network bridges, or appliance templates.
   - ALWAYS inspect current state first using MCP Resources (e.g. pve://cluster/resources, pve://nodes/{node}/status, pve://nodes/{node}/qemu/{vmid}/config, pve://nodes/{node}/lxc/{vmid}/config).
   - Verify that target guests, nodes, and storages actually exist before invoking any tool.

2. Two-Phase Confirmation Protocol ("confirm": true Rule):
   - ALL state-changing or mutating tools (power actions, hardware resizing, network updates, creation, cloning, reboot) enforce strict two-phase confirmation.
   - Initial Call: ALWAYS invoke the tool with "confirm": false (or omit "confirm"). This generates a structured [DRY RUN PREVIEW - USER CONFIRMATION REQUIRED] impact plan without making changes.
   - User Presentation: Present the full dry-run plan clearly to the human user and explicitly request their confirmation to proceed.
   - Confirmation Call: Re-run the tool with "confirm": true ONLY AFTER the human user explicitly responds with unambiguous authorization (e.g., "yes", "proceed", "confirmed").
   - Absolute Restrictions:
     * NEVER pass "confirm": true on an initial call or in anticipation of approval.
     * NEVER pass "confirm": true in the same turn as generating the dry-run preview.
     * If the user rejects, says "no", or provides alternative instructions, immediately abort and NEVER pass "confirm": true.

3. Anti-Hallucination & Destructive Action Safeguards:
   - When stopping, shutting down, or powering off a guest (pve_qemu_power, pve_lxc_power), you MUST supply the "expected_name" parameter matching the verified hostname. The server verifies this to prevent VMID collision accidents.
   - When creating or modifying hardware, specify exact numeric integers (e.g., cores: 2, memory: 1024 in MB) and valid disk sizes (e.g. disk: "4G").

4. Asynchronous Task Lifecycle & Completion Verification:
   - Mutations (start, stop, reboot, create, clone, disk resize) return a Proxmox UPID.
   - Do NOT declare an operation completed simply because a task UPID was returned.
   - You MUST call pve_task_status with the node and UPID to verify the task reaches status: "stopped" and exitstatus: "OK".
   - If exitstatus indicates an error, report the exact failure to the user.

5. Node Boundary & Security Scoping:
   - Always specify the exact node (e.g. node: "pve"). Disallowed nodes are rejected immediately by policy.
   - Tools are restricted at server boot by security categories (Power, Config, Storage, Destroy, Node Ops).
   - If a tool returns "tool not found" or Proxmox returns HTTP 403, explain the active permission boundary rather than attempting workarounds.

6. Creation & Storage Reference Formats:
   - Valid VMIDs range from 100 to 999999999.
   - Appliance templates use "storage:vztmpl/<archive>" format (e.g. "SSD:vztmpl/alpine-3.23-default_20260116_amd64.tar.xz").
   - Storage pools for rootfs/disks must be active and capable of storing images/rootdir (e.g. "local-lvm", "SSD").`

type Server struct {
	httpServer       *http.Server
	sseServer        *mcpserver.SSEServer
	streamableServer *mcpserver.StreamableHTTPServer
	cfg              *config.Config
	mcpServer        *mcpserver.MCPServer
}

type Option func(*options)

type options struct {
	observer *webui.Observer
}

func WithObserver(obs *webui.Observer) Option {
	return func(o *options) { o.observer = obs }
}

func CreateMCPServer(cfg *config.Config, client *pve.Client, opts ...Option) *mcpserver.MCPServer {
	var o options
	for _, opt := range opts {
		opt(&o)
	}

	serverOpts := []mcpserver.ServerOption{
		mcpserver.WithRecovery(),
		mcpserver.WithInstructions(Instructions),
	}
	if o.observer != nil {
		serverOpts = append(serverOpts,
			mcpserver.WithToolHandlerMiddleware(o.observer.Middleware()),
			mcpserver.WithHooks(o.observer.Hooks()),
		)
	}
	mcpServer := mcpserver.NewMCPServer("pve-mcp", "1.1.0", serverOpts...)
	if o.observer != nil {
		o.observer.SetServer(mcpServer)
	}
	tools.RegisterAll(mcpServer, client, cfg)
	resources.RegisterAll(mcpServer, client)
	prompts.RegisterAll(mcpServer)
	return mcpServer
}

func NewServer(cfg *config.Config, client *pve.Client, opts ...Option) (*Server, error) {
	mcpServer := CreateMCPServer(cfg, client, opts...)

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
		mcpServer:        mcpServer,
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

func (s *Server) MCPServer() *mcpserver.MCPServer {
	return s.mcpServer
}

func (s *Server) Addr() string {
	return s.httpServer.Addr
}

func (s *Server) Handler() http.Handler {
	return s.httpServer.Handler
}
