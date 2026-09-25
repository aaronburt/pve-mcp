package resources_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/aaronburt/pve-mcp/internal/config"
	"github.com/aaronburt/pve-mcp/internal/pve"
	"github.com/aaronburt/pve-mcp/internal/resources"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func setupTestClient(t *testing.T, handler http.HandlerFunc) (*pve.Client, func()) {
	mockServer := httptest.NewServer(handler)
	cfg := &config.Config{
		Host:        mockServer.URL,
		TokenID:     "user@pve!token",
		TokenSecret: "secret",
		VerifySSL:   false,
	}
	client, err := pve.NewClient(cfg)
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}
	return client, mockServer.Close
}

func defaultMockHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"data":{"status":"ok"}}`))
	}
}

func TestResources_StaticResources(t *testing.T) {
	client, cleanup := setupTestClient(t, defaultMockHandler())
	defer cleanup()

	mcpServer := server.NewMCPServer("pve-mcp-test", "1.0.0")
	resources.RegisterAll(mcpServer, client)

	registeredResources := mcpServer.ListResources()
	staticURIs := []string{
		"pve://cluster/status",
		"pve://cluster/resources",
		"pve://cluster/ha-status",
		"pve://cluster/nextid",
		"pve://cluster/log",
		"pve://nodes",
		"pve://storage",
		"pve://access/users",
		"pve://access/groups",
		"pve://access/roles",
		"pve://access/domains",
		"pve://access/permissions",
	}

	for _, uri := range staticURIs {
		res, exists := registeredResources[uri]
		if !exists {
			t.Fatalf("expected static resource %s to be registered", uri)
		}

		req := mcp.ReadResourceRequest{}
		req.Params.URI = uri
		contents, err := res.Handler(context.Background(), req)
		if err != nil {
			t.Fatalf("handler failed for %s: %v", uri, err)
		}
		if len(contents) == 0 {
			t.Fatalf("expected contents for %s", uri)
		}
		textResource, ok := mcp.AsTextResourceContents(contents[0])
		if !ok || textResource.Text == "" {
			t.Fatalf("expected non-empty text content for %s", uri)
		}
	}
}

func TestResources_DynamicTemplates(t *testing.T) {
	client, cleanup := setupTestClient(t, defaultMockHandler())
	defer cleanup()

	mcpServer := server.NewMCPServer("pve-mcp-test", "1.0.0")
	resources.RegisterAll(mcpServer, client)

	templateURIs := []string{
		"pve://nodes/pve1/status",
		"pve://nodes/pve1/version",
		"pve://nodes/pve1/syslog",
		"pve://nodes/pve1/network",
		"pve://nodes/pve1/tasks/UPID:pve1:12345",
		"pve://nodes/pve1/qemu",
		"pve://nodes/pve1/qemu/100/status",
		"pve://nodes/pve1/qemu/100/config",
		"pve://nodes/pve1/qemu/100/snapshots",
		"pve://nodes/pve1/qemu/100/firewall",
		"pve://nodes/pve1/lxc",
		"pve://nodes/pve1/lxc/100/status",
		"pve://nodes/pve1/lxc/100/config",
		"pve://nodes/pve1/lxc/100/snapshots",
		"pve://nodes/pve1/lxc/100/firewall",
		"pve://nodes/pve1/storage/local-zfs/status",
		"pve://nodes/pve1/storage/local-zfs/content",
	}

	for idCounter, uri := range templateURIs {
		reqMap := map[string]any{
			"jsonrpc": "2.0",
			"id":      idCounter + 1,
			"method":  "resources/read",
			"params": map[string]any{
				"uri": uri,
			},
		}
		reqBytes, _ := json.Marshal(reqMap)
		resp := mcpServer.HandleMessage(context.Background(), reqBytes)
		if resp == nil {
			t.Fatalf("nil response for %s", uri)
		}

		respBytes, _ := json.Marshal(resp)
		var rpcResp struct {
			Error *struct {
				Message string `json:"message"`
			} `json:"error"`
			Result *struct {
				Contents []struct {
					Text string `json:"text"`
				} `json:"contents"`
			} `json:"result"`
		}
		if err := json.Unmarshal(respBytes, &rpcResp); err != nil {
			t.Fatalf("failed to decode response for %s: %v", uri, err)
		}
		if rpcResp.Error != nil {
			t.Fatalf("unexpected RPC error for %s: %s", uri, rpcResp.Error.Message)
		}
		if rpcResp.Result == nil || len(rpcResp.Result.Contents) == 0 || rpcResp.Result.Contents[0].Text == "" {
			t.Fatalf("expected result text for %s", uri)
		}
	}
}

func TestResources_ValidationErrors(t *testing.T) {
	client, cleanup := setupTestClient(t, defaultMockHandler())
	defer cleanup()

	mcpServer := server.NewMCPServer("pve-mcp-test", "1.0.0")
	resources.RegisterAll(mcpServer, client)

	invalidURIs := []string{
		"pve://nodes/bad~node/status",
		"pve://nodes/bad~node/version",
		"pve://nodes/bad~node/syslog",
		"pve://nodes/bad~node/network",
		"pve://nodes/bad~node/tasks/UPID:123",
		"pve://nodes/bad~node/qemu",
		"pve://nodes/pve1/qemu/9999999999/status",
		"pve://nodes/pve1/qemu/50/status",
		"pve://nodes/bad~node/qemu/100/status",
		"pve://nodes/bad~node/qemu/100/config",
		"pve://nodes/bad~node/qemu/100/snapshots",
		"pve://nodes/bad~node/qemu/100/firewall",
		"pve://nodes/bad~node/lxc",
		"pve://nodes/bad~node/lxc/100/status",
		"pve://nodes/bad~node/lxc/100/config",
		"pve://nodes/bad~node/lxc/100/snapshots",
		"pve://nodes/bad~node/lxc/100/firewall",
		"pve://nodes/pve1/lxc/9999999999/status",
		"pve://nodes/pve1/lxc/50/status",
		"pve://nodes/pve1/storage/bad~storage/status",
		"pve://nodes/pve1/storage/bad~storage/content",
		"pve://nodes/bad~node/storage/local/status",
		"pve://nodes/bad~node/storage/local/content",
	}

	for idCounter, uri := range invalidURIs {
		reqMap := map[string]any{
			"jsonrpc": "2.0",
			"id":      idCounter + 100,
			"method":  "resources/read",
			"params": map[string]any{
				"uri": uri,
			},
		}
		reqBytes, _ := json.Marshal(reqMap)
		resp := mcpServer.HandleMessage(context.Background(), reqBytes)
		respBytes, _ := json.Marshal(resp)
		var rpcResp struct {
			Error *struct {
				Message string `json:"message"`
			} `json:"error"`
		}
		json.Unmarshal(respBytes, &rpcResp)
		if rpcResp.Error == nil {
			t.Fatalf("expected validation error for invalid uri %s", uri)
		}
	}
}

func TestResources_UpstreamErrors(t *testing.T) {
	errorHandler := func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("internal server error"))
	}
	client, cleanup := setupTestClient(t, errorHandler)
	defer cleanup()

	mcpServer := server.NewMCPServer("pve-mcp-test", "1.0.0")
	resources.RegisterAll(mcpServer, client)

	allURIs := []string{
		"pve://cluster/status",
		"pve://cluster/resources",
		"pve://cluster/ha-status",
		"pve://cluster/nextid",
		"pve://cluster/log",
		"pve://nodes",
		"pve://nodes/pve1/status",
		"pve://nodes/pve1/version",
		"pve://nodes/pve1/syslog",
		"pve://nodes/pve1/network",
		"pve://nodes/pve1/tasks/UPID:pve1:12345",
		"pve://nodes/pve1/qemu",
		"pve://nodes/pve1/qemu/100/status",
		"pve://nodes/pve1/qemu/100/config",
		"pve://nodes/pve1/qemu/100/snapshots",
		"pve://nodes/pve1/qemu/100/firewall",
		"pve://nodes/pve1/lxc",
		"pve://nodes/pve1/lxc/100/status",
		"pve://nodes/pve1/lxc/100/config",
		"pve://nodes/pve1/lxc/100/snapshots",
		"pve://nodes/pve1/lxc/100/firewall",
		"pve://storage",
		"pve://nodes/pve1/storage/local-zfs/status",
		"pve://nodes/pve1/storage/local-zfs/content",
		"pve://access/users",
		"pve://access/groups",
		"pve://access/roles",
		"pve://access/domains",
		"pve://access/permissions",
	}

	for idCounter, uri := range allURIs {
		reqMap := map[string]any{
			"jsonrpc": "2.0",
			"id":      idCounter + 200,
			"method":  "resources/read",
			"params": map[string]any{
				"uri": uri,
			},
		}
		reqBytes, _ := json.Marshal(reqMap)
		resp := mcpServer.HandleMessage(context.Background(), reqBytes)
		respBytes, _ := json.Marshal(resp)
		var rpcResp struct {
			Error *struct {
				Message string `json:"message"`
			} `json:"error"`
		}
		json.Unmarshal(respBytes, &rpcResp)
		if rpcResp.Error == nil {
			t.Fatalf("expected upstream error response for %s", uri)
		}
	}
}

func TestJSONResource_MarshalError(t *testing.T) {
	unmarshalableChannel := make(chan int)
	_, err := resources.JSONResource("pve://test", unmarshalableChannel)
	if err == nil {
		t.Fatalf("expected error marshaling unmarshalable channel")
	}
}
