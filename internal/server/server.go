package server

import (
	"context"
	"net"
	"net/http"
	"time"

	"github.com/aaronburt/pve-mcp/internal/config"
	"github.com/aaronburt/pve-mcp/internal/pve"
	"github.com/aaronburt/pve-mcp/internal/tools"
	mcpserver "github.com/mark3labs/mcp-go/server"
)

type Server struct {
	httpServer *http.Server
	sseServer  *mcpserver.SSEServer
	cfg        *config.Config
}

func NewServer(cfg *config.Config, client *pve.Client) (*Server, error) {
	mcpServer := mcpserver.NewMCPServer("pve-mcp", "1.0.0", mcpserver.WithRecovery())
	tools.RegisterAll(mcpServer, client)

	sseServer := mcpserver.NewSSEServer(mcpServer)

	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	})
	mux.Handle("/", sseServer)

	handler := OriginMiddleware(cfg, AuthMiddleware(cfg, mux))

	addr := net.JoinHostPort(cfg.BindAddress, cfg.Port)
	httpServer := &http.Server{
		Addr:              addr,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	return &Server{
		httpServer: httpServer,
		sseServer:  sseServer,
		cfg:        cfg,
	}, nil
}

func (s *Server) Start() error {
	return s.httpServer.ListenAndServe()
}

func (s *Server) Shutdown(ctx context.Context) error {
	_ = s.sseServer.Shutdown(ctx)
	return s.httpServer.Shutdown(ctx)
}

func (s *Server) Addr() string {
	return s.httpServer.Addr
}
