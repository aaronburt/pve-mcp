package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/aaronburt/pve-mcp/internal/config"
	"github.com/aaronburt/pve-mcp/internal/pve"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func RegisterQEMUTools(s *server.MCPServer, client *pve.Client, cfg *config.Config) {
	s.AddTool(
		mcp.NewTool(
			"pve_qemu_power",
			mcp.WithDescription("Control the power state of a QEMU virtual machine (start, stop, shutdown, reboot, suspend, resume)"),
			mcp.WithReadOnlyHintAnnotation(false),
			mcp.WithDestructiveHintAnnotation(true),
			mcp.WithString("node", mcp.Required(), mcp.Description("Node name")),
			mcp.WithInteger("vmid", mcp.Required(), mcp.Description("Virtual Machine ID")),
			mcp.WithString("action", mcp.Required(), mcp.Description("Power action to perform"), mcp.Enum("start", "stop", "shutdown", "reboot", "suspend", "resume")),
			mcp.WithString("expected_name", mcp.Description("Current name of the virtual machine; required when action is 'stop' to prevent accidental power-off")),
			mcp.WithInteger("timeout", mcp.Description("Graceful shutdown timeout in seconds")),
			mcp.WithBoolean("force", mcp.Description("Required true to execute an ungraceful hard power-off ('stop')")),
			mcp.WithBoolean("confirm", mcp.Description("Set to true after user confirmation to execute the power change; if false/omitted, returns a dry-run preview")),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			if err := EnsureMutationsAllowed(cfg); err != nil {
				return ErrorResult(err.Error())
			}
			rawNode, err := ParseRequiredString(req.Params.Arguments, "node")
			if err != nil {
				return ErrorResult(err.Error())
			}
			node, err := ValidateNode(rawNode)
			if err != nil {
				return ErrorResult(err.Error())
			}
			rawVMID, err := ParseRequiredInt(req.Params.Arguments, "vmid")
			if err != nil {
				return ErrorResult(err.Error())
			}
			vmid, err := ValidateVMID(rawVMID)
			if err != nil {
				return ErrorResult(err.Error())
			}
			rawAction, err := ParseRequiredString(req.Params.Arguments, "action")
			if err != nil {
				return ErrorResult(err.Error())
			}
			action, err := ValidatePowerAction(rawAction)
			if err != nil {
				return ErrorResult(err.Error())
			}

			if action == "stop" && !IsForce(req.Params.Arguments) {
				return ErrorResult("force power-off ('stop') can cause filesystem corruption. Use 'shutdown' for graceful ACPI power-off, or pass force: true to confirm immediate power cut.")
			}

			var statusData struct {
				Status string `json:"status"`
				Name   string `json:"name"`
			}
			_ = client.Get(ctx, fmt.Sprintf("/nodes/%s/qemu/%d/status/current", url.PathEscape(node), vmid), nil, &statusData)

			if err := VerifyTargetIdentity(extractArgs(req.Params.Arguments), statusData.Name, action == "stop"); err != nil {
				return ErrorResult(err.Error())
			}

			if !IsConfirmed(req.Params.Arguments) {
				currentStatus := statusData.Status
				if currentStatus == "" {
					currentStatus = "unknown"
				}
				preview := FormatDryRunPreview(
					fmt.Sprintf("Power action %q on VM %d (%s) (node %s)", action, vmid, statusData.Name, node),
					[]string{
						fmt.Sprintf("Current Status: %s", currentStatus),
						fmt.Sprintf("Target Action: %s", action),
					},
				)
				return mcp.NewToolResultText(preview), nil
			}

			path := fmt.Sprintf("/nodes/%s/qemu/%d/status/%s", url.PathEscape(node), vmid, action)
			form := url.Values{}
			if timeout, _ := ParseOptionalInt(req.Params.Arguments, "timeout", 0); timeout > 0 {
				form.Set("timeout", strconv.Itoa(timeout))
			}

			var taskUPID string
			if err := client.Post(ctx, path, form, &taskUPID); err != nil {
				return ErrorResult(err.Error())
			}
			return mcp.NewToolResultText(fmt.Sprintf("Power action %q dispatched successfully for VM %d on node %s. Task UPID: %s", action, vmid, node, taskUPID)), nil
		},
	)

	s.AddTool(
		mcp.NewTool(
			"pve_qemu_update_hardware",
			mcp.WithDescription("Update CPU, memory, and ballooning parameters of a QEMU virtual machine"),
			mcp.WithReadOnlyHintAnnotation(false),
			mcp.WithDestructiveHintAnnotation(true),
			mcp.WithString("node", mcp.Required(), mcp.Description("Node name")),
			mcp.WithInteger("vmid", mcp.Required(), mcp.Description("Virtual Machine ID")),
			mcp.WithInteger("cores", mcp.Description("Number of CPU cores per socket")),
			mcp.WithInteger("sockets", mcp.Description("Number of CPU sockets")),
			mcp.WithString("cpu", mcp.Description("Emulated CPU type (e.g. 'host', 'kvm64', 'x86-64-v2-AES')")),
			mcp.WithInteger("memory", mcp.Description("RAM allocation in Megabytes (MB)")),
			mcp.WithInteger("balloon", mcp.Description("Dynamically allocated balloon RAM in Megabytes (MB)")),
			mcp.WithBoolean("confirm", mcp.Description("Set to true after user confirmation to apply hardware updates; if false/omitted, returns a dry-run preview")),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			if err := EnsureMutationsAllowed(cfg); err != nil {
				return ErrorResult(err.Error())
			}
			rawNode, err := ParseRequiredString(req.Params.Arguments, "node")
			if err != nil {
				return ErrorResult(err.Error())
			}
			node, err := ValidateNode(rawNode)
			if err != nil {
				return ErrorResult(err.Error())
			}
			rawVMID, err := ParseRequiredInt(req.Params.Arguments, "vmid")
			if err != nil {
				return ErrorResult(err.Error())
			}
			vmid, err := ValidateVMID(rawVMID)
			if err != nil {
				return ErrorResult(err.Error())
			}

			var currentConfig map[string]any
			if err := client.Get(ctx, fmt.Sprintf("/nodes/%s/qemu/%d/config", url.PathEscape(node), vmid), nil, &currentConfig); err != nil {
				return ErrorResult(err.Error())
			}

			form := url.Values{}
			var diffs []string

			if cores, _ := ParseOptionalInt(req.Params.Arguments, "cores", 0); cores > 0 {
				form.Set("cores", strconv.Itoa(cores))
				diffs = append(diffs, fmt.Sprintf("cores: %v -> %d", currentConfig["cores"], cores))
			}
			if sockets, _ := ParseOptionalInt(req.Params.Arguments, "sockets", 0); sockets > 0 {
				form.Set("sockets", strconv.Itoa(sockets))
				diffs = append(diffs, fmt.Sprintf("sockets: %v -> %d", currentConfig["sockets"], sockets))
			}
			if cpu := ParseOptionalString(req.Params.Arguments, "cpu", ""); cpu != "" {
				form.Set("cpu", cpu)
				diffs = append(diffs, fmt.Sprintf("cpu: %v -> %s", currentConfig["cpu"], cpu))
			}
			if mem, _ := ParseOptionalInt(req.Params.Arguments, "memory", 0); mem > 0 {
				if mem < 16 {
					return ErrorResult("memory must be at least 16 MB")
				}
				form.Set("memory", strconv.Itoa(mem))
				diffs = append(diffs, fmt.Sprintf("memory: %v MB -> %d MB", currentConfig["memory"], mem))
			}
			if balloon, _ := ParseOptionalInt(req.Params.Arguments, "balloon", -1); balloon >= 0 {
				form.Set("balloon", strconv.Itoa(balloon))
				diffs = append(diffs, fmt.Sprintf("balloon: %v MB -> %d MB", currentConfig["balloon"], balloon))
			}

			if len(diffs) == 0 {
				return ErrorResult("at least one hardware attribute (cores, sockets, cpu, memory, balloon) must be specified")
			}

			if !IsConfirmed(req.Params.Arguments) {
				preview := FormatDryRunPreview(
					fmt.Sprintf("Update hardware on VM %d (node %s)", vmid, node),
					diffs,
				)
				return mcp.NewToolResultText(preview), nil
			}

			path := fmt.Sprintf("/nodes/%s/qemu/%d/config", url.PathEscape(node), vmid)
			var raw json.RawMessage
			if err := client.Post(ctx, path, form, &raw); err != nil {
				return ErrorResult(err.Error())
			}
			return mcp.NewToolResultText(fmt.Sprintf("Hardware successfully updated for VM %d on node %s.\nApplied modifications:\n  - %s", vmid, node, strings.Join(diffs, "\n  - "))), nil
		},
	)

	s.AddTool(
		mcp.NewTool(
			"pve_qemu_resize_disk",
			mcp.WithDescription("Extend the virtual disk capacity of a QEMU virtual machine"),
			mcp.WithReadOnlyHintAnnotation(false),
			mcp.WithDestructiveHintAnnotation(true),
			mcp.WithString("node", mcp.Required(), mcp.Description("Node name")),
			mcp.WithInteger("vmid", mcp.Required(), mcp.Description("Virtual Machine ID")),
			mcp.WithString("disk", mcp.Required(), mcp.Description("Disk identifier (e.g. 'scsi0', 'virtio0', 'sata0')")),
			mcp.WithString("size", mcp.Required(), mcp.Description("Size extension or target capacity (e.g. '+10G', '50G')")),
			mcp.WithBoolean("confirm", mcp.Description("Set to true after user confirmation to execute disk resize; if false/omitted, returns a dry-run preview")),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			if err := EnsureMutationsAllowed(cfg); err != nil {
				return ErrorResult(err.Error())
			}
			rawNode, err := ParseRequiredString(req.Params.Arguments, "node")
			if err != nil {
				return ErrorResult(err.Error())
			}
			node, err := ValidateNode(rawNode)
			if err != nil {
				return ErrorResult(err.Error())
			}
			rawVMID, err := ParseRequiredInt(req.Params.Arguments, "vmid")
			if err != nil {
				return ErrorResult(err.Error())
			}
			vmid, err := ValidateVMID(rawVMID)
			if err != nil {
				return ErrorResult(err.Error())
			}
			rawDisk, err := ParseRequiredString(req.Params.Arguments, "disk")
			if err != nil {
				return ErrorResult(err.Error())
			}
			disk, err := ValidateDiskIdentifier(rawDisk, true)
			if err != nil {
				return ErrorResult(err.Error())
			}
			rawSize, err := ParseRequiredString(req.Params.Arguments, "size")
			if err != nil {
				return ErrorResult(err.Error())
			}
			size, err := ValidateDiskSize(rawSize)
			if err != nil {
				return ErrorResult(err.Error())
			}

			var currentConfig map[string]any
			if err := client.Get(ctx, fmt.Sprintf("/nodes/%s/qemu/%d/config", url.PathEscape(node), vmid), nil, &currentConfig); err != nil {
				return ErrorResult(err.Error())
			}
			diskVal, exists := currentConfig[disk]
			if !exists {
				return ErrorResult(fmt.Sprintf("disk %q not found in VM %d configuration", disk, vmid))
			}
			diskConfigStr, _ := diskVal.(string)
			currentBytes := ExtractDiskSizeBytes(diskConfigStr)
			if err := ValidateDiskGrowth(currentBytes, size); err != nil {
				return ErrorResult(err.Error())
			}

			if !IsConfirmed(req.Params.Arguments) {
				diffs := []string{
					fmt.Sprintf("Disk Device: %s", disk),
					fmt.Sprintf("Current Volume Configuration: %s", diskConfigStr),
					fmt.Sprintf("Requested Size Increase: %s", size),
				}
				preview := FormatDryRunPreview(fmt.Sprintf("Resize disk %q on VM %d (node %s)", disk, vmid, node), diffs)
				return mcp.NewToolResultText(preview), nil
			}

			form := url.Values{}
			form.Set("disk", disk)
			form.Set("size", size)

			path := fmt.Sprintf("/nodes/%s/qemu/%d/resize", url.PathEscape(node), vmid)
			var taskUPID string
			if err := client.Put(ctx, path, form, &taskUPID); err != nil {
				return ErrorResult(err.Error())
			}
			return mcp.NewToolResultText(fmt.Sprintf("Disk %s on VM %d successfully resized (%s). Task UPID: %s", disk, vmid, size, taskUPID)), nil
		},
	)

	s.AddTool(
		mcp.NewTool(
			"pve_qemu_update_network",
			mcp.WithDescription("Update or configure a virtual network interface on a QEMU virtual machine"),
			mcp.WithReadOnlyHintAnnotation(false),
			mcp.WithDestructiveHintAnnotation(true),
			mcp.WithString("node", mcp.Required(), mcp.Description("Node name")),
			mcp.WithInteger("vmid", mcp.Required(), mcp.Description("Virtual Machine ID")),
			mcp.WithString("net_id", mcp.Required(), mcp.Description("Network interface ID (e.g. 'net0', 'net1')")),
			mcp.WithString("bridge", mcp.Description("Linux bridge to attach interface (e.g. 'vmbr0')")),
			mcp.WithString("model", mcp.Description("Network device model (e.g. 'virtio', 'e1000')")),
			mcp.WithInteger("tag", mcp.Description("VLAN tag (1-4094)")),
			mcp.WithBoolean("firewall", mcp.Description("Enable PVE firewall on this interface")),
			mcp.WithNumber("rate", mcp.Description("Rate limit in MB/s")),
			mcp.WithBoolean("confirm", mcp.Description("Set to true after user confirmation to apply network changes; if false/omitted, returns a dry-run preview")),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			if err := EnsureMutationsAllowed(cfg); err != nil {
				return ErrorResult(err.Error())
			}
			rawNode, err := ParseRequiredString(req.Params.Arguments, "node")
			if err != nil {
				return ErrorResult(err.Error())
			}
			node, err := ValidateNode(rawNode)
			if err != nil {
				return ErrorResult(err.Error())
			}
			rawVMID, err := ParseRequiredInt(req.Params.Arguments, "vmid")
			if err != nil {
				return ErrorResult(err.Error())
			}
			vmid, err := ValidateVMID(rawVMID)
			if err != nil {
				return ErrorResult(err.Error())
			}
			rawNetID, err := ParseRequiredString(req.Params.Arguments, "net_id")
			if err != nil {
				return ErrorResult(err.Error())
			}
			netID, err := ValidateNetID(rawNetID)
			if err != nil {
				return ErrorResult(err.Error())
			}

			var currentConfig map[string]any
			if err := client.Get(ctx, fmt.Sprintf("/nodes/%s/qemu/%d/config", url.PathEscape(node), vmid), nil, &currentConfig); err != nil {
				return ErrorResult(err.Error())
			}

			currentNetStr := ""
			if val, ok := currentConfig[netID]; ok {
				currentNetStr, _ = val.(string)
			}

			updates := make(map[string]string)
			if bridge := ParseOptionalString(req.Params.Arguments, "bridge", ""); bridge != "" {
				updates["bridge"] = bridge
			}
			if tag, _ := ParseOptionalInt(req.Params.Arguments, "tag", 0); tag > 0 && tag <= 4094 {
				updates["tag"] = strconv.Itoa(tag)
			}
			if argMap := extractArgs(req.Params.Arguments); argMap != nil {
				if fw, ok := argMap["firewall"]; ok && fw != nil {
					if ParseOptionalBool(req.Params.Arguments, "firewall", false) {
						updates["firewall"] = "1"
					} else {
						updates["firewall"] = "0"
					}
				}
				if rVal, ok := argMap["rate"]; ok && rVal != nil {
					updates["rate"] = fmt.Sprintf("%v", rVal)
				}
			}

			model := ParseOptionalString(req.Params.Arguments, "model", "")
			baseString := currentNetStr
			if baseString == "" {
				if model == "" {
					model = "virtio"
				}
				baseString = fmt.Sprintf("%s=auto", model)
			} else if model != "" {
				parts := strings.Split(baseString, ",")
				first := parts[0]
				eqIdx := strings.Index(first, "=")
				if eqIdx != -1 {
					mac := first[eqIdx+1:]
					parts[0] = fmt.Sprintf("%s=%s", model, mac)
					baseString = strings.Join(parts, ",")
				}
			}

			mergedNet := MergeNetworkConfig(baseString, updates)
			if currentNetStr == "" && !strings.Contains(mergedNet, "bridge=") {
				return ErrorResult("bridge parameter is required when creating a new network interface")
			}

			diffs := []string{
				fmt.Sprintf("Interface: %s", netID),
				fmt.Sprintf("Current Configuration: %s", currentNetStr),
				fmt.Sprintf("Proposed Configuration: %s", mergedNet),
			}

			if !IsConfirmed(req.Params.Arguments) {
				preview := FormatDryRunPreview(fmt.Sprintf("Update network interface %s on VM %d (node %s)", netID, vmid, node), diffs)
				return mcp.NewToolResultText(preview), nil
			}

			form := url.Values{}
			form.Set(netID, mergedNet)

			path := fmt.Sprintf("/nodes/%s/qemu/%d/config", url.PathEscape(node), vmid)
			var raw json.RawMessage
			if err := client.Post(ctx, path, form, &raw); err != nil {
				return ErrorResult(err.Error())
			}
			return mcp.NewToolResultText(fmt.Sprintf("Network interface %s updated on VM %d:\n%s", netID, vmid, mergedNet)), nil
		},
	)

	s.AddTool(
		mcp.NewTool(
			"pve_qemu_clone",
			mcp.WithDescription("Clone an existing QEMU virtual machine or template to a new VMID"),
			mcp.WithReadOnlyHintAnnotation(false),
			mcp.WithDestructiveHintAnnotation(true),
			mcp.WithString("node", mcp.Required(), mcp.Description("Node name")),
			mcp.WithInteger("vmid", mcp.Required(), mcp.Description("Source Virtual Machine ID or template ID")),
			mcp.WithInteger("newid", mcp.Required(), mcp.Description("Target new Virtual Machine ID")),
			mcp.WithString("name", mcp.Description("Name for the new virtual machine")),
			mcp.WithString("storage", mcp.Description("Target storage pool for full disk clone")),
			mcp.WithBoolean("full", mcp.Description("Full clone (default true) creates independent disks; false creates linked clone")),
			mcp.WithString("pool", mcp.Description("Add the cloned VM to a specified resource pool")),
			mcp.WithBoolean("wait", mcp.Description("Wait up to 30 seconds for the clone task to complete (default true)")),
			mcp.WithBoolean("confirm", mcp.Description("Set to true after user confirmation to execute the clone; if false/omitted, returns a dry-run preview")),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			if err := EnsureMutationsAllowed(cfg); err != nil {
				return ErrorResult(err.Error())
			}
			rawNode, err := ParseRequiredString(req.Params.Arguments, "node")
			if err != nil {
				return ErrorResult(err.Error())
			}
			node, err := ValidateNode(rawNode)
			if err != nil {
				return ErrorResult(err.Error())
			}
			rawVMID, err := ParseRequiredInt(req.Params.Arguments, "vmid")
			if err != nil {
				return ErrorResult(err.Error())
			}
			vmid, err := ValidateVMID(rawVMID)
			if err != nil {
				return ErrorResult(err.Error())
			}
			rawNewID, err := ParseRequiredInt(req.Params.Arguments, "newid")
			if err != nil {
				return ErrorResult(err.Error())
			}
			newid, err := ValidateVMID(rawNewID)
			if err != nil {
				return ErrorResult(err.Error())
			}
			if vmid == newid {
				return ErrorResult("newid must be different from source vmid")
			}

			name := ParseOptionalString(req.Params.Arguments, "name", "")
			storage := ParseOptionalString(req.Params.Arguments, "storage", "")
			pool := ParseOptionalString(req.Params.Arguments, "pool", "")
			full := ParseOptionalBool(req.Params.Arguments, "full", true)
			wait := ParseOptionalBool(req.Params.Arguments, "wait", true)

			cloneType := "full clone"
			if !full {
				cloneType = "linked clone"
			}

			diffs := []string{
				fmt.Sprintf("Source VMID: %d", vmid),
				fmt.Sprintf("Target VMID: %d", newid),
				fmt.Sprintf("Clone Mode: %s", cloneType),
			}
			if name != "" {
				diffs = append(diffs, fmt.Sprintf("Target Name: %s", name))
			}
			if storage != "" {
				diffs = append(diffs, fmt.Sprintf("Target Storage: %s", storage))
			}
			if pool != "" {
				diffs = append(diffs, fmt.Sprintf("Target Pool: %s", pool))
			}

			if !IsConfirmed(req.Params.Arguments) {
				preview := FormatDryRunPreview(fmt.Sprintf("Clone VM %d to %d (%s) on node %s", vmid, newid, cloneType, node), diffs)
				return mcp.NewToolResultText(preview), nil
			}

			form := url.Values{}
			form.Set("newid", strconv.Itoa(newid))
			if full {
				form.Set("full", "1")
			} else {
				form.Set("full", "0")
			}
			if name != "" {
				form.Set("name", name)
			}
			if storage != "" {
				form.Set("storage", storage)
			}
			if pool != "" {
				form.Set("pool", pool)
			}

			path := fmt.Sprintf("/nodes/%s/qemu/%d/clone", url.PathEscape(node), vmid)
			var upid string
			if err := client.Post(ctx, path, form, &upid); err != nil {
				return ErrorResult(err.Error())
			}

			if !wait {
				return mcp.NewToolResultText(fmt.Sprintf("Clone task initiated successfully. UPID: %s\nUse pve_task_status to monitor progress.", upid)), nil
			}

			taskStatus, err := WaitForTask(ctx, client, node, upid, 30, req.Params.Meta)
			if err != nil {
				return mcp.NewToolResultText(fmt.Sprintf("Clone task initiated (UPID: %s), but wait encountered error: %v", upid, err)), nil
			}
			if taskStatus.Status == "stopped" {
				if taskStatus.ExitStatus == "OK" {
					return mcp.NewToolResultText(fmt.Sprintf("Successfully cloned VM %d to %d (%s) on node %s. Task UPID: %s", vmid, newid, name, node, upid)), nil
				}
				return ErrorResult(fmt.Sprintf("Clone task failed with exit status: %s (UPID: %s)", taskStatus.ExitStatus, upid))
			}
			return mcp.NewToolResultText(fmt.Sprintf("Clone task is still running after 30s. Task UPID: %s\nUse pve_task_status to check status.", upid)), nil
		},
	)

	s.AddTool(
		mcp.NewTool(
			"pve_qemu_destroy",
			mcp.WithDescription("Permanently delete a QEMU virtual machine and its associated virtual disks"),
			mcp.WithReadOnlyHintAnnotation(false),
			mcp.WithDestructiveHintAnnotation(true),
			mcp.WithString("node", mcp.Required(), mcp.Description("Node name")),
			mcp.WithInteger("vmid", mcp.Required(), mcp.Description("Virtual Machine ID to permanently delete")),
			mcp.WithString("expected_name", mcp.Required(), mcp.Description("Current name of the virtual machine to confirm identity before destruction")),
			mcp.WithBoolean("purge", mcp.Description("Remove VM from backup jobs and purge disk configurations (default true)")),
			mcp.WithBoolean("destroy_disks", mcp.Description("Remove and erase all associated virtual disks (default true)")),
			mcp.WithBoolean("unprotect", mcp.Description("Explicitly remove protection flag before deletion if enabled")),
			mcp.WithBoolean("confirm", mcp.Description("Set to true after user confirmation to execute deletion; if false/omitted, returns a dry-run preview")),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			if err := EnsureMutationsAllowed(cfg); err != nil {
				return ErrorResult(err.Error())
			}
			if err := EnsureDestroyAllowed(cfg); err != nil {
				return ErrorResult(err.Error())
			}

			rawNode, err := ParseRequiredString(req.Params.Arguments, "node")
			if err != nil {
				return ErrorResult(err.Error())
			}
			node, err := ValidateNode(rawNode)
			if err != nil {
				return ErrorResult(err.Error())
			}
			rawVMID, err := ParseRequiredInt(req.Params.Arguments, "vmid")
			if err != nil {
				return ErrorResult(err.Error())
			}
			vmid, err := ValidateVMID(rawVMID)
			if err != nil {
				return ErrorResult(err.Error())
			}

			var statusData struct {
				Status string `json:"status"`
				Name   string `json:"name"`
			}
			if err := client.Get(ctx, fmt.Sprintf("/nodes/%s/qemu/%d/status/current", url.PathEscape(node), vmid), nil, &statusData); err != nil {
				return ErrorResult(err.Error())
			}

			if err := VerifyTargetIdentity(extractArgs(req.Params.Arguments), statusData.Name, true); err != nil {
				return ErrorResult(err.Error())
			}

			if statusData.Status != "stopped" {
				return ErrorResult(fmt.Sprintf("cannot destroy VM %d while it is %q. Stop or shutdown the VM first.", vmid, statusData.Status))
			}

			unprotect := ParseOptionalBool(req.Params.Arguments, "unprotect", false)
			var configData map[string]any
			_ = client.Get(ctx, fmt.Sprintf("/nodes/%s/qemu/%d/config", url.PathEscape(node), vmid), nil, &configData)
			if protVal, ok := configData["protection"]; ok {
				if protStr := fmt.Sprintf("%v", protVal); protStr == "1" || protStr == "true" {
					if !unprotect {
						return ErrorResult(fmt.Sprintf("VM %d has protection flag enabled. Pass unprotect: true to confirm removing protection and destroying.", vmid))
					}
					unprotForm := url.Values{}
					unprotForm.Set("protection", "0")
					_ = client.Put(ctx, fmt.Sprintf("/nodes/%s/qemu/%d/config", url.PathEscape(node), vmid), unprotForm, nil)
				}
			}

			diffs := []string{
				fmt.Sprintf("Target VMID: %d", vmid),
				fmt.Sprintf("Target Name: %s", statusData.Name),
				fmt.Sprintf("Verified Identity: %s", statusData.Name),
				fmt.Sprintf("Status: %s", statusData.Status),
				"Action: PERMANENT DELETION of virtual machine and all attached storage disks",
			}

			if !IsConfirmed(req.Params.Arguments) {
				preview := FormatDryRunPreview(fmt.Sprintf("DESTROY VM %d (%s) on node %s", vmid, statusData.Name, node), diffs)
				return mcp.NewToolResultText(preview), nil
			}

			purge := ParseOptionalBool(req.Params.Arguments, "purge", true)
			destroyDisks := ParseOptionalBool(req.Params.Arguments, "destroy_disks", true)

			form := url.Values{}
			if purge {
				form.Set("purge", "1")
			}
			if destroyDisks {
				form.Set("destroy-unreferenced-disks", "1")
			}

			path := fmt.Sprintf("/nodes/%s/qemu/%d", url.PathEscape(node), vmid)
			var upid string
			if err := client.Delete(ctx, path, form, &upid); err != nil {
				return ErrorResult(err.Error())
			}

			return mcp.NewToolResultText(fmt.Sprintf("Destroy task initiated for VM %d on node %s. Task UPID: %s", vmid, node, upid)), nil
		},
	)

	s.AddTool(
		mcp.NewTool(
			"pve_qemu_protection",
			mcp.WithDescription("Set or remove the deletion protection flag on a QEMU virtual machine"),
			mcp.WithReadOnlyHintAnnotation(false),
			mcp.WithDestructiveHintAnnotation(true),
			mcp.WithString("node", mcp.Required(), mcp.Description("Node name")),
			mcp.WithInteger("vmid", mcp.Required(), mcp.Description("Virtual Machine ID")),
			mcp.WithBoolean("protected", mcp.Required(), mcp.Description("Set to true to enable deletion protection; false to unprotect")),
			mcp.WithBoolean("confirm", mcp.Description("Set to true after user confirmation to execute protection change; if false/omitted, returns a dry-run preview")),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			if err := EnsureMutationsAllowed(cfg); err != nil {
				return ErrorResult(err.Error())
			}

			rawNode, err := ParseRequiredString(req.Params.Arguments, "node")
			if err != nil {
				return ErrorResult(err.Error())
			}
			node, err := ValidateNode(rawNode)
			if err != nil {
				return ErrorResult(err.Error())
			}
			rawVMID, err := ParseRequiredInt(req.Params.Arguments, "vmid")
			if err != nil {
				return ErrorResult(err.Error())
			}
			vmid, err := ValidateVMID(rawVMID)
			if err != nil {
				return ErrorResult(err.Error())
			}
			protected, err := ParseRequiredBool(req.Params.Arguments, "protected")
			if err != nil {
				return ErrorResult(err.Error())
			}

			var configData map[string]any
			_ = client.Get(ctx, fmt.Sprintf("/nodes/%s/qemu/%d/config", url.PathEscape(node), vmid), nil, &configData)
			currentProtection := "disabled"
			if protVal, ok := configData["protection"]; ok {
				if protStr := fmt.Sprintf("%v", protVal); protStr == "1" || protStr == "true" {
					currentProtection = "enabled"
				}
			}

			targetProtection := "disabled"
			targetVal := "0"
			if protected {
				targetProtection = "enabled"
				targetVal = "1"
			}

			diffs := []string{
				fmt.Sprintf("Current Protection: %s", currentProtection),
				fmt.Sprintf("Proposed Protection: %s", targetProtection),
			}

			if !IsConfirmed(req.Params.Arguments) {
				preview := FormatDryRunPreview(fmt.Sprintf("Set protection to %s on VM %d (node %s)", targetProtection, vmid, node), diffs)
				return mcp.NewToolResultText(preview), nil
			}

			form := url.Values{}
			form.Set("protection", targetVal)

			path := fmt.Sprintf("/nodes/%s/qemu/%d/config", url.PathEscape(node), vmid)
			var raw json.RawMessage
			if err := client.Post(ctx, path, form, &raw); err != nil {
				return ErrorResult(err.Error())
			}

			return mcp.NewToolResultText(fmt.Sprintf("Protection successfully set to %s for VM %d on node %s.", targetProtection, vmid, node)), nil
		},
	)
}
