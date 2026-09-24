package tools_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/aaronburt/pve-mcp/internal/config"
	"github.com/aaronburt/pve-mcp/internal/pve"
	"github.com/aaronburt/pve-mcp/internal/tools"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

type toolTestCase struct {
	name        string
	validArgs   map[string]any
	invalidArgs map[string]any
}

func TestTools_RegisterAndExecuteAll(t *testing.T) {
	mockPVE := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"data":[{"status":"ok"}]}`))
	}))
	defer mockPVE.Close()

	cfg := &config.Config{
		Host:        mockPVE.URL,
		TokenID:     "user@pve!token",
		TokenSecret: "secret",
		VerifySSL:   false,
	}

	client, err := pve.NewClient(cfg)
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	mcpServer := server.NewMCPServer("pve-mcp", "1.0.0")
	tools.RegisterAll(mcpServer, client)

	cases := []toolTestCase{
		{name: "pve_cluster_status"},
		{name: "pve_cluster_resources", validArgs: map[string]any{"type": "vm"}},
		{name: "pve_cluster_nextid"},
		{name: "pve_cluster_log", validArgs: map[string]any{"max": 10}},
		{name: "pve_cluster_ha_status"},
		{name: "pve_nodes_list"},
		{name: "pve_node_status", validArgs: map[string]any{"node": "pve"}, invalidArgs: map[string]any{"node": "../bad"}},
		{name: "pve_node_version", validArgs: map[string]any{"node": "pve"}, invalidArgs: map[string]any{"node": ""}},
		{name: "pve_node_syslog", validArgs: map[string]any{"node": "pve", "limit": 10, "since": "2026-01-01", "until": "2026-01-02"}, invalidArgs: map[string]any{}},
		{name: "pve_node_rrddata", validArgs: map[string]any{"node": "pve", "timeframe": "hour", "cf": "AVERAGE"}, invalidArgs: map[string]any{"node": "pve"}},
		{name: "pve_qemu_list", validArgs: map[string]any{"node": "pve"}, invalidArgs: map[string]any{}},
		{name: "pve_qemu_status", validArgs: map[string]any{"node": "pve", "vmid": 100}, invalidArgs: map[string]any{"node": "pve", "vmid": 10}},
		{name: "pve_qemu_config", validArgs: map[string]any{"node": "pve", "vmid": 100}, invalidArgs: map[string]any{}},
		{name: "pve_qemu_snapshots", validArgs: map[string]any{"node": "pve", "vmid": 100}, invalidArgs: map[string]any{}},
		{name: "pve_qemu_firewall", validArgs: map[string]any{"node": "pve", "vmid": 100}, invalidArgs: map[string]any{}},
		{name: "pve_lxc_list", validArgs: map[string]any{"node": "pve"}, invalidArgs: map[string]any{}},
		{name: "pve_lxc_status", validArgs: map[string]any{"node": "pve", "vmid": 100}, invalidArgs: map[string]any{"node": "pve", "vmid": 99}},
		{name: "pve_lxc_config", validArgs: map[string]any{"node": "pve", "vmid": 100}, invalidArgs: map[string]any{}},
		{name: "pve_lxc_snapshots", validArgs: map[string]any{"node": "pve", "vmid": 100}, invalidArgs: map[string]any{}},
		{name: "pve_lxc_firewall", validArgs: map[string]any{"node": "pve", "vmid": 100}, invalidArgs: map[string]any{}},
		{name: "pve_storage_list", validArgs: map[string]any{"node": "pve"}, invalidArgs: map[string]any{}},
		{name: "pve_storage_status", validArgs: map[string]any{"node": "pve", "storage": "local"}, invalidArgs: map[string]any{"node": "pve", "storage": "../bad"}},
		{name: "pve_storage_content", validArgs: map[string]any{"node": "pve", "storage": "local", "content": "iso"}, invalidArgs: map[string]any{}},
		{name: "pve_network_list", validArgs: map[string]any{"node": "pve", "type": "bridge"}, invalidArgs: map[string]any{}},
		{name: "pve_access_users"},
		{name: "pve_access_groups"},
		{name: "pve_access_roles"},
		{name: "pve_access_domains"},
		{name: "pve_access_permissions", validArgs: map[string]any{"userid": "root@pam", "path": "/"}},
	}

	if len(cases) != 29 {
		t.Fatalf("expected exactly 29 test cases, got %d", len(cases))
	}

	ctx := context.Background()

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			st := mcpServer.GetTool(tc.name)
			if st == nil {
				t.Fatalf("tool %s not registered in server", tc.name)
			}

			reqValid := mcp.CallToolRequest{
				Params: mcp.CallToolParams{
					Name:      tc.name,
					Arguments: tc.validArgs,
				},
			}
			resValid, err := st.Handler(ctx, reqValid)
			if err != nil || resValid == nil || resValid.IsError {
				t.Fatalf("tool %s failed valid execution: err=%v, res=%+v", tc.name, err, resValid)
			}

			if tc.invalidArgs != nil {
				reqInvalid := mcp.CallToolRequest{
					Params: mcp.CallToolParams{
						Name:      tc.name,
						Arguments: tc.invalidArgs,
					},
				}
				resInvalid, err := st.Handler(ctx, reqInvalid)
				if err != nil || resInvalid == nil || !resInvalid.IsError {
					t.Fatalf("tool %s expected validation error on invalid args: err=%v, res=%+v", tc.name, err, resInvalid)
				}
			}
		})
	}

	errPVE := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "simulated pve failure", http.StatusInternalServerError)
	}))
	defer errPVE.Close()

	errClient, err := pve.NewClient(&config.Config{
		Host:        errPVE.URL,
		TokenID:     "user@pve!token",
		TokenSecret: "secret",
		VerifySSL:   false,
	})
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	errServer := server.NewMCPServer("pve-mcp", "1.0.0")
	tools.RegisterAll(errServer, errClient)

	for _, tc := range cases {
		st := errServer.GetTool(tc.name)
		res, _ := st.Handler(ctx, mcp.CallToolRequest{
			Params: mcp.CallToolParams{
				Name:      tc.name,
				Arguments: tc.validArgs,
			},
		})
		if res == nil || !res.IsError {
			t.Fatalf("expected error result from %s when PVE fails, got %+v", tc.name, res)
		}
	}

	modeTools := []string{"pve_cluster_resources", "pve_qemu_list", "pve_lxc_list", "pve_storage_list"}
	for _, toolName := range modeTools {
		st := mcpServer.GetTool(toolName)
		args := map[string]any{"node": "pve", "mode": "full"}
		res, err := st.Handler(ctx, mcp.CallToolRequest{
			Params: mcp.CallToolParams{
				Name:      toolName,
				Arguments: args,
			},
		})
		if err != nil || res == nil || res.IsError {
			t.Fatalf("tool %s failed in full mode: err=%v, res=%+v", toolName, err, res)
		}
	}
}
