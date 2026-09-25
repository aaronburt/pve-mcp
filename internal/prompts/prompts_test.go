package prompts_test

import (
	"context"
	"testing"

	"github.com/aaronburt/pve-mcp/internal/prompts"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func TestPrompts_RegisterAndExecuteAll(t *testing.T) {
	mcpServer := server.NewMCPServer("pve-mcp-test", "1.0.0")
	prompts.RegisterAll(mcpServer)

	registeredPrompts := mcpServer.ListPrompts()
	expectedNames := []string{
		"cluster_health_audit",
		"vm_incident_triage",
		"safe_host_evacuation",
		"provision_workload_planner",
		"storage_cleanup_advisor",
	}

	for _, name := range expectedNames {
		promptEntry, exists := registeredPrompts[name]
		if !exists {
			t.Fatalf("expected prompt %s to be registered", name)
		}
		if promptEntry.Handler == nil {
			t.Fatalf("expected non-nil handler for %s", name)
		}
	}

	t.Run("cluster_health_audit without storage", func(t *testing.T) {
		req := mcp.GetPromptRequest{
			Params: mcp.GetPromptParams{
				Name:      "cluster_health_audit",
				Arguments: map[string]string{"check_storage": "false"},
			},
		}
		res, err := registeredPrompts["cluster_health_audit"].Handler(context.Background(), req)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(res.Messages) == 0 {
			t.Fatalf("expected messages in result")
		}
	})

	t.Run("cluster_health_audit with storage", func(t *testing.T) {
		req := mcp.GetPromptRequest{
			Params: mcp.GetPromptParams{
				Name:      "cluster_health_audit",
				Arguments: map[string]string{"check_storage": "true"},
			},
		}
		res, err := registeredPrompts["cluster_health_audit"].Handler(context.Background(), req)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(res.Messages) == 0 {
			t.Fatalf("expected messages in result")
		}
	})

	t.Run("vm_incident_triage valid", func(t *testing.T) {
		req := mcp.GetPromptRequest{
			Params: mcp.GetPromptParams{
				Name: "vm_incident_triage",
				Arguments: map[string]string{
					"node": "pve1",
					"vmid": "100",
				},
			},
		}
		res, err := registeredPrompts["vm_incident_triage"].Handler(context.Background(), req)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(res.Messages) == 0 {
			t.Fatalf("expected messages in result")
		}
	})

	t.Run("vm_incident_triage missing arguments", func(t *testing.T) {
		req := mcp.GetPromptRequest{
			Params: mcp.GetPromptParams{
				Name:      "vm_incident_triage",
				Arguments: map[string]string{"node": "pve1"},
			},
		}
		_, err := registeredPrompts["vm_incident_triage"].Handler(context.Background(), req)
		if err == nil {
			t.Fatalf("expected error for missing vmid")
		}
	})

	t.Run("safe_host_evacuation valid", func(t *testing.T) {
		req := mcp.GetPromptRequest{
			Params: mcp.GetPromptParams{
				Name:      "safe_host_evacuation",
				Arguments: map[string]string{"node": "pve1"},
			},
		}
		res, err := registeredPrompts["safe_host_evacuation"].Handler(context.Background(), req)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(res.Messages) == 0 {
			t.Fatalf("expected messages in result")
		}
	})

	t.Run("safe_host_evacuation missing node", func(t *testing.T) {
		req := mcp.GetPromptRequest{
			Params: mcp.GetPromptParams{
				Name:      "safe_host_evacuation",
				Arguments: map[string]string{},
			},
		}
		_, err := registeredPrompts["safe_host_evacuation"].Handler(context.Background(), req)
		if err == nil {
			t.Fatalf("expected error for missing node")
		}
	})

	t.Run("provision_workload_planner with qemu and node and name", func(t *testing.T) {
		req := mcp.GetPromptRequest{
			Params: mcp.GetPromptParams{
				Name: "provision_workload_planner",
				Arguments: map[string]string{
					"workload_type": "qemu",
					"node":          "pve1",
					"name":          "web-server",
				},
			},
		}
		res, err := registeredPrompts["provision_workload_planner"].Handler(context.Background(), req)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(res.Messages) == 0 {
			t.Fatalf("expected messages in result")
		}
	})

	t.Run("provision_workload_planner with lxc without node", func(t *testing.T) {
		req := mcp.GetPromptRequest{
			Params: mcp.GetPromptParams{
				Name: "provision_workload_planner",
				Arguments: map[string]string{
					"workload_type": "lxc",
				},
			},
		}
		res, err := registeredPrompts["provision_workload_planner"].Handler(context.Background(), req)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(res.Messages) == 0 {
			t.Fatalf("expected messages in result")
		}
	})

	t.Run("provision_workload_planner invalid type", func(t *testing.T) {
		req := mcp.GetPromptRequest{
			Params: mcp.GetPromptParams{
				Name: "provision_workload_planner",
				Arguments: map[string]string{
					"workload_type": "docker",
				},
			},
		}
		_, err := registeredPrompts["provision_workload_planner"].Handler(context.Background(), req)
		if err == nil {
			t.Fatalf("expected error for invalid workload_type")
		}
	})

	t.Run("storage_cleanup_advisor with node", func(t *testing.T) {
		req := mcp.GetPromptRequest{
			Params: mcp.GetPromptParams{
				Name:      "storage_cleanup_advisor",
				Arguments: map[string]string{"node": "pve1"},
			},
		}
		res, err := registeredPrompts["storage_cleanup_advisor"].Handler(context.Background(), req)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(res.Messages) == 0 {
			t.Fatalf("expected messages in result")
		}
	})

	t.Run("storage_cleanup_advisor without node", func(t *testing.T) {
		req := mcp.GetPromptRequest{
			Params: mcp.GetPromptParams{
				Name:      "storage_cleanup_advisor",
				Arguments: map[string]string{},
			},
		}
		res, err := registeredPrompts["storage_cleanup_advisor"].Handler(context.Background(), req)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(res.Messages) == 0 {
			t.Fatalf("expected messages in result")
		}
	})
}
