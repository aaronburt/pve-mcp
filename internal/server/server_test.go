package server_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/aaronburt/pve-mcp/internal/config"
	"github.com/aaronburt/pve-mcp/internal/pve"
	"github.com/aaronburt/pve-mcp/internal/server"
)

func TestAuthMiddleware(t *testing.T) {
	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})

	noAuthCfg := &config.Config{MCPAuthToken: ""}
	noAuthHandler := server.AuthMiddleware(noAuthCfg, nextHandler)

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()
	noAuthHandler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 with no auth token configured, got %d", rec.Code)
	}

	authCfg := &config.Config{MCPAuthToken: "secret-token"}
	authHandler := server.AuthMiddleware(authCfg, nextHandler)

	healthReq := httptest.NewRequest(http.MethodGet, "/health", nil)
	healthRec := httptest.NewRecorder()
	authHandler.ServeHTTP(healthRec, healthReq)
	if healthRec.Code != http.StatusOK {
		t.Fatalf("expected 200 on /health bypass, got %d", healthRec.Code)
	}

	unauthReq := httptest.NewRequest(http.MethodGet, "/test", nil)
	unauthRec := httptest.NewRecorder()
	authHandler.ServeHTTP(unauthRec, unauthReq)
	if unauthRec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 on missing token, got %d", unauthRec.Code)
	}

	badReq := httptest.NewRequest(http.MethodGet, "/test", nil)
	badReq.Header.Set("Authorization", "Bearer wrong-token")
	badRec := httptest.NewRecorder()
	authHandler.ServeHTTP(badRec, badReq)
	if badRec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 on wrong token, got %d", badRec.Code)
	}

	goodReq := httptest.NewRequest(http.MethodGet, "/test", nil)
	goodReq.Header.Set("Authorization", "Bearer secret-token")
	goodRec := httptest.NewRecorder()
	authHandler.ServeHTTP(goodRec, goodReq)
	if goodRec.Code != http.StatusOK {
		t.Fatalf("expected 200 on valid bearer token, got %d", goodRec.Code)
	}

	queryReq := httptest.NewRequest(http.MethodGet, "/test?token=secret-token", nil)
	queryRec := httptest.NewRecorder()
	authHandler.ServeHTTP(queryRec, queryReq)
	if queryRec.Code != http.StatusOK {
		t.Fatalf("expected 200 on valid query token, got %d", queryRec.Code)
	}
}

func TestOriginMiddleware(t *testing.T) {
	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})

	noOriginsCfg := &config.Config{AllowedOrigins: nil}
	noOriginsHandler := server.OriginMiddleware(noOriginsCfg, nextHandler)

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Origin", "http://example.com")
	rec := httptest.NewRecorder()
	noOriginsHandler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 when allowed origins is empty, got %d", rec.Code)
	}

	originsCfg := &config.Config{
		AllowedOrigins: []string{"http://localhost:3000", "http://127.0.0.1:3000"},
	}
	originsHandler := server.OriginMiddleware(originsCfg, nextHandler)

	noOriginReq := httptest.NewRequest(http.MethodGet, "/test", nil)
	noOriginRec := httptest.NewRecorder()
	originsHandler.ServeHTTP(noOriginRec, noOriginReq)
	if noOriginRec.Code != http.StatusOK {
		t.Fatalf("expected 200 when Origin header is omitted, got %d", noOriginRec.Code)
	}

	allowedReq := httptest.NewRequest(http.MethodGet, "/test", nil)
	allowedReq.Header.Set("Origin", "http://localhost:3000")
	allowedRec := httptest.NewRecorder()
	originsHandler.ServeHTTP(allowedRec, allowedReq)
	if allowedRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for allowed origin, got %d", allowedRec.Code)
	}

	disallowedReq := httptest.NewRequest(http.MethodGet, "/test", nil)
	disallowedReq.Header.Set("Origin", "http://malicious.example.com")
	disallowedRec := httptest.NewRecorder()
	originsHandler.ServeHTTP(disallowedRec, disallowedReq)
	if disallowedRec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for disallowed origin, got %d", disallowedRec.Code)
	}
}

func TestServerLifecycle(t *testing.T) {
	cfg := &config.Config{
		Host:        "https://127.0.0.1:8006",
		TokenID:     "user@pve!token",
		TokenSecret: "secret",
		BindAddress: "127.0.0.1",
		Port:        "0",
		VerifySSL:   false,
	}

	client, err := pve.NewClient(cfg)
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	srv, err := server.NewServer(cfg, client)
	if err != nil {
		t.Fatalf("failed to create server: %v", err)
	}

	if srv.Addr() != "127.0.0.1:0" {
		t.Errorf("expected 127.0.0.1:0, got %s", srv.Addr())
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	err = srv.Shutdown(ctx)
	if err != nil {
		t.Fatalf("unexpected shutdown error: %v", err)
	}
}
