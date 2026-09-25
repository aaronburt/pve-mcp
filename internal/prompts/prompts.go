package prompts

import (
	"context"
	"fmt"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func RegisterAll(mcpServer *server.MCPServer) {
	mcpServer.AddPrompt(
		mcp.NewPrompt(
			"cluster_health_audit",
			mcp.WithPromptDescription("Full cluster health and quorum audit inspecting nodes, resources, storage, and HA state"),
			mcp.WithArgument("check_storage", mcp.ArgumentDescription("Whether to inspect storage capacity (true/false)")),
		),
		handleClusterHealthAudit,
	)

	mcpServer.AddPrompt(
		mcp.NewPrompt(
			"vm_incident_triage",
			mcp.WithPromptDescription("Root-cause diagnosis for a malfunctioning VM or container using status, config, and node syslog"),
			mcp.WithArgument("node", mcp.RequiredArgument(), mcp.ArgumentDescription("Target node name")),
			mcp.WithArgument("vmid", mcp.RequiredArgument(), mcp.ArgumentDescription("Target VM or container ID")),
		),
		handleVMIncidentTriage,
	)

	mcpServer.AddPrompt(
		mcp.NewPrompt(
			"safe_host_evacuation",
			mcp.WithPromptDescription("Staged migration and shutdown plan for evacuating a node before host maintenance or reboot"),
			mcp.WithArgument("node", mcp.RequiredArgument(), mcp.ArgumentDescription("Node name to evacuate")),
		),
		handleSafeHostEvacuation,
	)

	mcpServer.AddPrompt(
		mcp.NewPrompt(
			"provision_workload_planner",
			mcp.WithPromptDescription("Interactive sizing and template selection for new VMs or LXC containers"),
			mcp.WithArgument("workload_type", mcp.RequiredArgument(), mcp.ArgumentDescription("Workload type ('qemu' or 'lxc')")),
			mcp.WithArgument("node", mcp.ArgumentDescription("Target node name (optional)")),
			mcp.WithArgument("name", mcp.ArgumentDescription("Guest hostname or label (optional)")),
		),
		handleProvisionWorkloadPlanner,
	)

	mcpServer.AddPrompt(
		mcp.NewPrompt(
			"storage_cleanup_advisor",
			mcp.WithPromptDescription("Scans for unreferenced disks, dangling snapshots, and oversized storage allocations"),
			mcp.WithArgument("node", mcp.ArgumentDescription("Target node name (optional)")),
		),
		handleStorageCleanupAdvisor,
	)
}

func handleClusterHealthAudit(ctx context.Context, req mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
	checkStorage := req.Params.Arguments["check_storage"] == "true"

	var instructions strings.Builder
	instructions.WriteString("Execute a comprehensive cluster health audit for Proxmox VE:\n\n")
	instructions.WriteString("1. Read `pve://cluster/status` to verify quorum, cluster member list, and overall health.\n")
	instructions.WriteString("2. Read `pve://cluster/resources` to analyze all VM, container, and node allocations.\n")
	instructions.WriteString("3. Read `pve://cluster/ha-status` to inspect high-availability services and fence state.\n")
	if checkStorage {
		instructions.WriteString("4. Read `pve://storage` to verify free space and allocation thresholds across all storage pools.\n")
	}
	instructions.WriteString("\nSynthesize findings into an operational health report highlighting warnings, degraded nodes, or offline services.")

	message := mcp.NewPromptMessage(mcp.RoleUser, mcp.NewTextContent(instructions.String()))
	return mcp.NewGetPromptResult("Cluster Health Audit Workflow", []mcp.PromptMessage{message}), nil
}

func handleVMIncidentTriage(ctx context.Context, req mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
	node := strings.TrimSpace(req.Params.Arguments["node"])
	vmid := strings.TrimSpace(req.Params.Arguments["vmid"])
	if node == "" || vmid == "" {
		return nil, fmt.Errorf("both 'node' and 'vmid' arguments are required for vm_incident_triage")
	}

	instructions := fmt.Sprintf(
		"Perform root-cause diagnosis on guest %s on node %s:\n\n"+
			"1. Read `pve://nodes/%s/qemu/%s/status` and `pve://nodes/%s/lxc/%s/status` to determine guest type and runtime state.\n"+
			"2. Read `pve://nodes/%s/qemu/%s/config` (or LXC config) to verify memory, CPU, disk attachments, and boot parameters.\n"+
			"3. Read `pve://nodes/%s/syslog` to extract recent systemd journal entries and kernel errors relating to VMID %s.\n"+
			"4. Read `pve://nodes/%s/qemu/%s/firewall` to verify network packet filter rules.\n\n"+
			"Diagnose the cause of the failure and provide concrete remediation steps.",
		vmid, node,
		node, vmid, node, vmid,
		node, vmid,
		node, vmid,
		node, vmid,
	)

	message := mcp.NewPromptMessage(mcp.RoleUser, mcp.NewTextContent(instructions))
	return mcp.NewGetPromptResult("VM Incident Triage Workflow", []mcp.PromptMessage{message}), nil
}

func handleSafeHostEvacuation(ctx context.Context, req mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
	node := strings.TrimSpace(req.Params.Arguments["node"])
	if node == "" {
		return nil, fmt.Errorf("'node' argument is required for safe_host_evacuation")
	}

	instructions := fmt.Sprintf(
		"Plan a safe evacuation and maintenance preparation for Proxmox node %s:\n\n"+
			"1. Read `pve://nodes` and `pve://cluster/resources` to identify other active cluster nodes with sufficient RAM and CPU headroom.\n"+
			"2. Read `pve://nodes/%s/qemu` and `pve://nodes/%s/lxc` to inventory all running guests on %s.\n"+
			"3. For shared-storage guests, plan live online migrations to candidate destination nodes.\n"+
			"4. For local-storage guests, draft an orderly ACPI shutdown sequence using `pve_qemu_power` or `pve_lxc_power` with dry-run verification.\n"+
			"5. Confirm no critical cluster quorum or HA leader constraints are violated before stopping node %s.",
		node,
		node, node, node,
		node,
	)

	message := mcp.NewPromptMessage(mcp.RoleUser, mcp.NewTextContent(instructions))
	return mcp.NewGetPromptResult("Safe Host Evacuation Workflow", []mcp.PromptMessage{message}), nil
}

func handleProvisionWorkloadPlanner(ctx context.Context, req mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
	workloadType := strings.ToLower(strings.TrimSpace(req.Params.Arguments["workload_type"]))
	if workloadType != "qemu" && workloadType != "lxc" {
		return nil, fmt.Errorf("argument 'workload_type' must be 'qemu' or 'lxc'")
	}
	node := strings.TrimSpace(req.Params.Arguments["node"])
	guestName := strings.TrimSpace(req.Params.Arguments["name"])

	var instructions strings.Builder
	instructions.WriteString(fmt.Sprintf("Plan provisioning for a new %s workload:\n\n", strings.ToUpper(workloadType)))
	instructions.WriteString("1. Read `pve://cluster/nextid` to obtain the next available VMID.\n")
	if node != "" {
		instructions.WriteString(fmt.Sprintf("2. Read `pve://nodes/%s/status` to verify host resource capacity.\n", node))
	} else {
		instructions.WriteString("2. Read `pve://nodes` to select the cluster node with lowest CPU and RAM commitment.\n")
	}
	instructions.WriteString("3. Read `pve://storage` to select storage pools supporting disk images and templates.\n")
	if guestName != "" {
		instructions.WriteString(fmt.Sprintf("4. Workload name requested: %q.\n", guestName))
	}
	instructions.WriteString("5. Present a proposed hardware, disk, and network configuration to the user for confirmation before executing creation.\n")

	message := mcp.NewPromptMessage(mcp.RoleUser, mcp.NewTextContent(instructions.String()))
	return mcp.NewGetPromptResult("Provision Workload Planner Workflow", []mcp.PromptMessage{message}), nil
}

func handleStorageCleanupAdvisor(ctx context.Context, req mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
	node := strings.TrimSpace(req.Params.Arguments["node"])

	var instructions strings.Builder
	instructions.WriteString("Analyze cluster storage to identify cleanup opportunities:\n\n")
	instructions.WriteString("1. Read `pve://storage` to list all configured storage backends and active pools.\n")
	instructions.WriteString("2. Read `pve://cluster/resources` to identify all actively mapped virtual disks.\n")
	if node != "" {
		instructions.WriteString(fmt.Sprintf("3. Read `pve://nodes/%s/storage` content to scan for untracked volumes on %s.\n", node, node))
	} else {
		instructions.WriteString("3. Scan storage pools for unreferenced disk volumes, obsolete template images, and old backup tarballs.\n")
	}
	instructions.WriteString("4. Output a classified cleanup recommendation tiered by risk (safe to delete vs verification required).\n")

	message := mcp.NewPromptMessage(mcp.RoleUser, mcp.NewTextContent(instructions.String()))
	return mcp.NewGetPromptResult("Storage Cleanup Advisor Workflow", []mcp.PromptMessage{message}), nil
}
