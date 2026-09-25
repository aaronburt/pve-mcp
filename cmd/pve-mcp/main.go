package main

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/aaronburt/pve-mcp/internal/config"
	"github.com/aaronburt/pve-mcp/internal/pve"
	"github.com/aaronburt/pve-mcp/internal/server"
	mcpserver "github.com/mark3labs/mcp-go/server"
)

func setupLogger(levelStr string, out io.Writer) {
	var level slog.Level
	switch strings.ToLower(levelStr) {
	case "debug":
		level = slog.LevelDebug
	case "warn", "warning":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	default:
		level = slog.LevelInfo
	}

	handler := slog.NewJSONHandler(out, &slog.HandlerOptions{
		Level: level,
	})
	slog.SetDefault(slog.New(handler))
}

func main() {
	config.LoadDotEnv(".env")
	if exe, err := os.Executable(); err == nil {
		config.LoadDotEnv(filepath.Join(filepath.Dir(exe), ".env"))
	}

	isStdio := os.Getenv("MCP_TRANSPORT") == "stdio"
	for _, arg := range os.Args[1:] {
		if arg == "--stdio" || arg == "-stdio" {
			isStdio = true
			break
		}
	}

	if isStdio {
		setupLogger("error", os.Stderr)
	}

	cfg, err := config.LoadFromEnv()
	if err != nil {
		slog.Error("failed to load configuration", "error", err)
		os.Exit(1)
	}

	if !isStdio {
		setupLogger(cfg.LogLevel, os.Stdout)
		slog.Info("starting pve-mcp server",
			"host", cfg.Host,
			"token_id", cfg.TokenID,
			"verify_ssl", cfg.VerifySSL,
			"bind_address", cfg.BindAddress,
			"port", cfg.Port,
			"auth_required", cfg.MCPAuthToken != "",
		)
	} else {
		setupLogger(cfg.LogLevel, os.Stderr)
	}

	client, err := pve.NewClient(cfg)
	if err != nil {
		slog.Error("failed to initialize pve client", "error", err)
		os.Exit(1)
	}

	if isStdio {
		mcpServer := server.CreateMCPServer(cfg, client)
		stdioServer := mcpserver.NewStdioServer(mcpServer)
		ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer cancel()
		if err := stdioServer.Listen(ctx, os.Stdin, os.Stdout); err != nil && !errors.Is(err, context.Canceled) {
			slog.Error("stdio server error", "error", err)
			os.Exit(1)
		}
		return
	}

	srv, err := server.NewServer(cfg, client)
	if err != nil {
		slog.Error("failed to initialize http server", "error", err)
		os.Exit(1)
	}

	shutdownChan := make(chan os.Signal, 1)
	signal.Notify(shutdownChan, os.Interrupt, syscall.SIGTERM)

	serverErrChan := make(chan error, 1)
	go func() {
		slog.Info("listening for incoming mcp connections", "addr", srv.Addr())
		if err := srv.Start(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErrChan <- err
		}
	}()

	select {
	case sig := <-shutdownChan:
		slog.Info("received shutdown signal", "signal", sig.String())
	case err := <-serverErrChan:
		slog.Error("server listener error", "error", err)
		os.Exit(1)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		slog.Error("error during server shutdown", "error", err)
		os.Exit(1)
	}

	slog.Info("pve-mcp server gracefully stopped")
}
