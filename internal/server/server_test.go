package server_test

import (
	"bufio"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
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

	go func() {
		_ = srv.Start()
	}()
	time.Sleep(50 * time.Millisecond)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	err = srv.Shutdown(ctx)
	if err != nil {
		t.Fatalf("unexpected shutdown error: %v", err)
	}
}

func newTestServer(t *testing.T) (*server.Server, *httptest.Server) {
	t.Helper()
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

	ts := httptest.NewServer(srv.Handler())
	t.Cleanup(func() {
		ts.Close()
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = srv.Shutdown(ctx)
	})

	return srv, ts
}

func TestHealthEndpoint(t *testing.T) {
	_, ts := newTestServer(t)

	resp, err := http.Get(ts.URL + "/health")
	if err != nil {
		t.Fatalf("unexpected GET error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	if ct := resp.Header.Get("Content-Type"); ct != "application/json" {
		t.Errorf("expected application/json, got %s", ct)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("failed to read body: %v", err)
	}

	if string(body) != `{"status":"ok"}` {
		t.Errorf("expected {\"status\":\"ok\"}, got %s", string(body))
	}
}

func TestStreamableHTTPEndpoints(t *testing.T) {
	_, ts := newTestServer(t)

	initPayload := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test-client","version":"1.0"}}}`

	endpoints := []string{"/sse", "/mcp", "/"}
	for _, ep := range endpoints {
		resp, err := http.Post(ts.URL+ep, "application/json", strings.NewReader(initPayload))
		if err != nil {
			t.Fatalf("failed POST %s: %v", ep, err)
		}

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			t.Fatalf("expected 200 on %s, got %d: %s", ep, resp.StatusCode, string(body))
		}

		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			t.Fatalf("failed to read %s body: %v", ep, err)
		}

		if !strings.Contains(string(body), `"protocolVersion"`) {
			t.Errorf("expected initialize result on %s, got %s", ep, string(body))
		}
	}

	toolsPayload := `{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}`
	resp, err := http.Post(ts.URL+"/sse", "application/json", strings.NewReader(toolsPayload))
	if err != nil {
		t.Fatalf("failed tools/list POST: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 on tools/list, got %d", resp.StatusCode)
	}

	toolsBody, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(toolsBody), `"tools"`) {
		t.Errorf("expected tools list in response, got %s", string(toolsBody))
	}
}

func TestSSEEndpointStreamAndHandshake(t *testing.T) {
	_, ts := newTestServer(t)

	req, err := http.NewRequest(http.MethodGet, ts.URL+"/sse", nil)
	if err != nil {
		t.Fatalf("failed to create SSE request: %v", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("failed to connect to SSE endpoint: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	if ct := resp.Header.Get("Content-Type"); !strings.HasPrefix(ct, "text/event-stream") {
		t.Fatalf("expected text/event-stream, got %s", ct)
	}

	reader := bufio.NewReader(resp.Body)
	foundEndpoint := false
	var endpointData string

	for i := 0; i < 10; i++ {
		line, err := reader.ReadString('\n')
		if err != nil {
			t.Fatalf("failed reading SSE stream: %v", err)
		}
		trimmed := strings.TrimSpace(line)
		if trimmed == "event: endpoint" {
			foundEndpoint = true
			dataLine, err := reader.ReadString('\n')
			if err != nil {
				t.Fatalf("failed reading endpoint data: %v", err)
			}
			endpointData = strings.TrimSpace(dataLine)
			break
		}
	}

	if !foundEndpoint {
		t.Fatal("expected event: endpoint in SSE stream")
	}

	if !strings.HasPrefix(endpointData, "data: ") || !strings.Contains(endpointData, "sessionId=") {
		t.Fatalf("expected data line with sessionId, got %s", endpointData)
	}
}

func TestSSEMessageRoundtrip(t *testing.T) {
	_, ts := newTestServer(t)

	req, err := http.NewRequest(http.MethodGet, ts.URL+"/sse", nil)
	if err != nil {
		t.Fatalf("failed to create SSE request: %v", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("failed to connect to SSE: %v", err)
	}
	defer resp.Body.Close()

	reader := bufio.NewReader(resp.Body)
	var messagePath string

	for i := 0; i < 10; i++ {
		line, err := reader.ReadString('\n')
		if err != nil {
			t.Fatalf("error reading SSE stream: %v", err)
		}
		if strings.TrimSpace(line) == "event: endpoint" {
			dataLine, err := reader.ReadString('\n')
			if err != nil {
				t.Fatalf("error reading endpoint data: %v", err)
			}
			raw := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(dataLine), "data:"))
			messagePath = raw
			break
		}
	}

	if messagePath == "" {
		t.Fatal("failed to extract message endpoint from SSE stream")
	}

	targetURL := ts.URL + messagePath
	if strings.HasPrefix(messagePath, "http://") || strings.HasPrefix(messagePath, "https://") {
		targetURL = messagePath
	}

	initPayload := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test-client","version":"1.0"}}}`
	postResp, err := http.Post(targetURL, "application/json", strings.NewReader(initPayload))
	if err != nil {
		t.Fatalf("failed to post message: %v", err)
	}
	defer postResp.Body.Close()

	if postResp.StatusCode != http.StatusAccepted {
		t.Fatalf("expected 202 Accepted on message post, got %d", postResp.StatusCode)
	}

	receivedMessage := false
	for i := 0; i < 15; i++ {
		line, err := reader.ReadString('\n')
		if err != nil {
			t.Fatalf("error reading response event: %v", err)
		}
		if strings.TrimSpace(line) == "event: message" {
			receivedMessage = true
			dataLine, _ := reader.ReadString('\n')
			if !strings.Contains(dataLine, `"protocolVersion"`) {
				t.Errorf("expected protocolVersion in message data, got %s", dataLine)
			}
			break
		}
	}

	if !receivedMessage {
		t.Fatal("expected event: message with JSON-RPC response on SSE stream")
	}
}

func TestSSEMessageMissingSession(t *testing.T) {
	_, ts := newTestServer(t)

	payload := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`
	resp, err := http.Post(ts.URL+"/message", "application/json", strings.NewReader(payload))
	if err != nil {
		t.Fatalf("unexpected post error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusAccepted {
		t.Fatalf("expected rejection for missing session ID on /message, got %d", resp.StatusCode)
	}
}

func TestCreateMCPServer(t *testing.T) {
	cfg := &config.Config{
		Host:        "https://127.0.0.1:8006",
		TokenID:     "user@pve!token",
		TokenSecret: "secret",
		VerifySSL:   false,
	}
	client, err := pve.NewClient(cfg)
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	mcpServer := server.CreateMCPServer(cfg, client)
	if len(mcpServer.ListTools()) != 16 {
		t.Fatalf("expected 16 tools, got %d", len(mcpServer.ListTools()))
	}
	if len(mcpServer.ListPrompts()) != 5 {
		t.Fatalf("expected 5 prompts, got %d", len(mcpServer.ListPrompts()))
	}
}

func TestStreamableHTTPResourcesAndPrompts(t *testing.T) {
	_, ts := newTestServer(t)

	resourcesPayload := `{"jsonrpc":"2.0","id":10,"method":"resources/list","params":{}}`
	resp, err := http.Post(ts.URL+"/mcp", "application/json", strings.NewReader(resourcesPayload))
	if err != nil {
		t.Fatalf("failed resources/list POST: %v", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), "pve://") {
		t.Errorf("expected pve:// resources in response, got %s", string(body))
	}

	templatesPayload := `{"jsonrpc":"2.0","id":11,"method":"resources/templates/list","params":{}}`
	respTemplates, err := http.Post(ts.URL+"/mcp", "application/json", strings.NewReader(templatesPayload))
	if err != nil {
		t.Fatalf("failed resources/templates/list POST: %v", err)
	}
	defer respTemplates.Body.Close()
	bodyTemplates, _ := io.ReadAll(respTemplates.Body)
	if !strings.Contains(string(bodyTemplates), "pve://") {
		t.Errorf("expected pve:// templates in response, got %s", string(bodyTemplates))
	}

	promptsPayload := `{"jsonrpc":"2.0","id":12,"method":"prompts/list","params":{}}`
	respPrompts, err := http.Post(ts.URL+"/mcp", "application/json", strings.NewReader(promptsPayload))
	if err != nil {
		t.Fatalf("failed prompts/list POST: %v", err)
	}
	defer respPrompts.Body.Close()
	bodyPrompts, _ := io.ReadAll(respPrompts.Body)
	if !strings.Contains(string(bodyPrompts), "cluster_health_audit") {
		t.Errorf("expected cluster_health_audit in prompts list, got %s", string(bodyPrompts))
	}
}
