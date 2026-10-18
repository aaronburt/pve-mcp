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
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/aaronburt/pve-mcp-go/internal/config"
	"github.com/aaronburt/pve-mcp-go/internal/pve"
	"github.com/aaronburt/pve-mcp-go/internal/server"
	"github.com/aaronburt/pve-mcp-go/internal/webui"
	mcpserver "github.com/mark3labs/mcp-go/server"
)

var logLevel = new(slog.LevelVar)

func parseLevel(levelStr string) slog.Level {
	switch strings.ToLower(levelStr) {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

func setupLogger(levelStr string, out io.Writer, wrap func(slog.Handler) slog.Handler) {
	logLevel.Set(parseLevel(levelStr))
	var handler slog.Handler = slog.NewJSONHandler(out, &slog.HandlerOptions{Level: logLevel})
	if wrap != nil {
		handler = wrap(handler)
	}
	slog.SetDefault(slog.New(handler))
}

func uiFlags(args []string) (enabled bool, port string) {
	for i := 0; i < len(args); i++ {
		switch {
		case args[i] == "--ui" || args[i] == "-ui":
			enabled = true
		case args[i] == "--ui-port" || args[i] == "-ui-port":
			if i+1 < len(args) {
				port = args[i+1]
				i++
			}
		case strings.HasPrefix(args[i], "--ui-port="):
			port = strings.TrimPrefix(args[i], "--ui-port=")
		}
	}
	return enabled, port
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
		setupLogger("error", os.Stderr, nil)
	}

	cfg, err := config.LoadFromEnv()
	if err != nil {
		slog.Error("failed to load configuration", "error", err)
		os.Exit(1)
	}

	uiOn, uiPort := uiFlags(os.Args[1:])
	if uiOn {
		cfg.UIEnabled = true
	}
	if uiPort != "" {
		if p, err := strconv.Atoi(uiPort); err != nil || p < 1 || p > 65535 {
			slog.Error("--ui-port must be a valid TCP port", "value", uiPort)
			os.Exit(1)
		}
		cfg.UIPort = uiPort
	}

	logOut := io.Writer(os.Stdout)
	if isStdio {
		logOut = os.Stderr
	}

	if !isStdio {
		setupLogger(cfg.LogLevel, os.Stdout, nil)
		slog.Info("starting pve-mcp server",
			"host", cfg.Host,
			"token_id", cfg.TokenID,
			"verify_ssl", cfg.VerifySSL,
			"bind_address", cfg.BindAddress,
			"port", cfg.Port,
			"auth_required", cfg.MCPAuthToken != "",
		)
	} else {
		setupLogger(cfg.LogLevel, os.Stderr, nil)
	}

	client, err := pve.NewClient(cfg)
	if err != nil {
		slog.Error("failed to initialize pve client", "error", err)
		os.Exit(1)
	}

	var (
		serverOpts []server.Option
		ui         *webui.Server
		toolCount  func() int
	)
	if cfg.UIEnabled {
		if cfg.UIToken == "" {
			slog.Error("web ui not started: MCP_UI_TOKEN is required")
		} else {
			buf := webui.NewBuffer()
			obs := webui.NewObserver(buf, client.Sanitize)
			transport := "http"
			if isStdio {
				transport = "stdio"
			}
			var err error
			ui, err = webui.New(webui.Options{
				BindAddress:       cfg.UIBindAddress,
				Port:              cfg.UIPort,
				Token:             cfg.UIToken,
				AllowInsecureBind: cfg.UIAllowInsecure,
				AllowedHosts:      cfg.UIAllowedHosts,
				Transport:         transport,
				Buffer:            buf,
				LogLevel:          logLevel,
				Ping: func(ctx context.Context) error {
					var v map[string]any
					return client.Get(ctx, "/version", nil, &v)
				},
				ToolCount: func() int { return toolCount() },
			})
			if err != nil {
				slog.Error("web ui not started", "error", err)
				buf.Close()
				ui = nil
			} else {
				setupLogger(cfg.LogLevel, logOut, obs.LogHandler)
				serverOpts = append(serverOpts, server.WithObserver(obs))
			}
		}
	}

	startUI := func() {
		if ui == nil {
			return
		}
		go func() {
			slog.Info("web ui listening", "url", "http://"+ui.Addr())
			if err := ui.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
				slog.Error("web ui listener error", "error", err)
			}
		}()
	}
	stopUI := func() {
		if ui == nil {
			return
		}
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := ui.Shutdown(ctx); err != nil {
			slog.Error("error during web ui shutdown", "error", err)
		}
	}

	if isStdio {
		mcpServer := server.CreateMCPServer(cfg, client, serverOpts...)
		toolCount = func() int { return len(mcpServer.ListTools()) }
		startUI()
		defer stopUI()
		stdioServer := mcpserver.NewStdioServer(mcpServer)
		ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer cancel()
		if err := stdioServer.Listen(ctx, os.Stdin, os.Stdout); err != nil && !errors.Is(err, context.Canceled) {
			slog.Error("stdio server error", "error", err)
			os.Exit(1)
		}
		return
	}

	srv, err := server.NewServer(cfg, client, serverOpts...)
	if err != nil {
		slog.Error("failed to initialize http server", "error", err)
		os.Exit(1)
	}
	toolCount = func() int { return len(srv.MCPServer().ListTools()) }
	startUI()

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

	stopUI()
	if err := srv.Shutdown(ctx); err != nil {
		slog.Error("error during server shutdown", "error", err)
		os.Exit(1)
	}

	slog.Info("pve-mcp server gracefully stopped")
}
