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

func RegisterLXCTools(s *server.MCPServer, client *pve.Client, cfg *config.Config) {
	s.AddTool(
		mcp.NewTool(
			"pve_lxc_power",
			mcp.WithDescription("Control the power state of an LXC container (start, stop, shutdown, reboot, suspend, resume)"),
			mcp.WithReadOnlyHintAnnotation(false),
			mcp.WithDestructiveHintAnnotation(true),
			mcp.WithString("node", mcp.Required(), mcp.Description("Node name")),
			mcp.WithInteger("vmid", mcp.Required(), mcp.Description("Container ID")),
			mcp.WithString("action", mcp.Required(), mcp.Description("Power action to perform"), mcp.Enum("start", "stop", "shutdown", "reboot", "suspend", "resume")),
			mcp.WithString("expected_name", mcp.Description("Current hostname of the container; required when action is 'stop' to prevent accidental power-off")),
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
			_ = client.Get(ctx, fmt.Sprintf("/nodes/%s/lxc/%d/status/current", url.PathEscape(node), vmid), nil, &statusData)

			if err := VerifyTargetIdentity(extractArgs(req.Params.Arguments), statusData.Name, action == "stop"); err != nil {
				return ErrorResult(err.Error())
			}

			if !IsConfirmed(req.Params.Arguments) {
				currentStatus := statusData.Status
				if currentStatus == "" {
					currentStatus = "unknown"
				}
				preview := FormatDryRunPreview(
					fmt.Sprintf("Power action %q on LXC %d (%s) (node %s)", action, vmid, statusData.Name, node),
					[]string{
						fmt.Sprintf("Current Status: %s", currentStatus),
						fmt.Sprintf("Target Action: %s", action),
					},
				)
				return mcp.NewToolResultText(preview), nil
			}

			path := fmt.Sprintf("/nodes/%s/lxc/%d/status/%s", url.PathEscape(node), vmid, action)
			form := url.Values{}
			if timeout, _ := ParseOptionalInt(req.Params.Arguments, "timeout", 0); timeout > 0 {
				form.Set("timeout", strconv.Itoa(timeout))
			}

			var taskUPID string
			if err := client.Post(ctx, path, form, &taskUPID); err != nil {
				return ErrorResult(err.Error())
			}
			return mcp.NewToolResultText(fmt.Sprintf("Power action %q dispatched successfully for LXC %d on node %s. Task UPID: %s", action, vmid, node, taskUPID)), nil
		},
	)

	s.AddTool(
		mcp.NewTool(
			"pve_lxc_update_hardware",
			mcp.WithDescription("Update CPU, memory, and swap allocations of an LXC container"),
			mcp.WithReadOnlyHintAnnotation(false),
			mcp.WithDestructiveHintAnnotation(true),
			mcp.WithString("node", mcp.Required(), mcp.Description("Node name")),
			mcp.WithInteger("vmid", mcp.Required(), mcp.Description("Container ID")),
			mcp.WithInteger("cores", mcp.Description("Number of CPU cores")),
			mcp.WithInteger("memory", mcp.Description("RAM allocation in Megabytes (MB)")),
			mcp.WithInteger("swap", mcp.Description("Swap space in Megabytes (MB)")),
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
			if err := client.Get(ctx, fmt.Sprintf("/nodes/%s/lxc/%d/config", url.PathEscape(node), vmid), nil, &currentConfig); err != nil {
				return ErrorResult(err.Error())
			}

			form := url.Values{}
			var diffs []string

			if cores, _ := ParseOptionalInt(req.Params.Arguments, "cores", 0); cores > 0 {
				form.Set("cores", strconv.Itoa(cores))
				diffs = append(diffs, fmt.Sprintf("cores: %v -> %d", currentConfig["cores"], cores))
			}
			if mem, _ := ParseOptionalInt(req.Params.Arguments, "memory", 0); mem > 0 {
				if mem < 16 {
					return ErrorResult("memory must be at least 16 MB")
				}
				form.Set("memory", strconv.Itoa(mem))
				diffs = append(diffs, fmt.Sprintf("memory: %v MB -> %d MB", currentConfig["memory"], mem))
			}
			if swap, _ := ParseOptionalInt(req.Params.Arguments, "swap", -1); swap >= 0 {
				form.Set("swap", strconv.Itoa(swap))
				diffs = append(diffs, fmt.Sprintf("swap: %v MB -> %d MB", currentConfig["swap"], swap))
			}

			if len(diffs) == 0 {
				return ErrorResult("at least one hardware attribute (cores, memory, swap) must be specified")
			}

			if !IsConfirmed(req.Params.Arguments) {
				preview := FormatDryRunPreview(
					fmt.Sprintf("Update hardware on LXC %d (node %s)", vmid, node),
					diffs,
				)
				return mcp.NewToolResultText(preview), nil
			}

			path := fmt.Sprintf("/nodes/%s/lxc/%d/config", url.PathEscape(node), vmid)
			var raw json.RawMessage
			if err := client.Put(ctx, path, form, &raw); err != nil {
				return ErrorResult(err.Error())
			}
			return mcp.NewToolResultText(fmt.Sprintf("Hardware successfully updated for LXC %d on node %s.\nApplied modifications:\n  - %s", vmid, node, strings.Join(diffs, "\n  - "))), nil
		},
	)

	s.AddTool(
		mcp.NewTool(
			"pve_lxc_resize_disk",
			mcp.WithDescription("Extend the virtual disk or mountpoint capacity of an LXC container"),
			mcp.WithReadOnlyHintAnnotation(false),
			mcp.WithDestructiveHintAnnotation(true),
			mcp.WithString("node", mcp.Required(), mcp.Description("Node name")),
			mcp.WithInteger("vmid", mcp.Required(), mcp.Description("Container ID")),
			mcp.WithString("disk", mcp.Required(), mcp.Description("Disk identifier (e.g. 'rootfs', 'mp0')")),
			mcp.WithString("size", mcp.Required(), mcp.Description("Size extension or target capacity (e.g. '+5G', '20G')")),
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
			disk, err := ValidateDiskIdentifier(rawDisk, false)
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
			if err := client.Get(ctx, fmt.Sprintf("/nodes/%s/lxc/%d/config", url.PathEscape(node), vmid), nil, &currentConfig); err != nil {
				return ErrorResult(err.Error())
			}
			diskVal, exists := currentConfig[disk]
			if !exists {
				return ErrorResult(fmt.Sprintf("disk %q not found in LXC %d configuration", disk, vmid))
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
				preview := FormatDryRunPreview(fmt.Sprintf("Resize disk %q on LXC %d (node %s)", disk, vmid, node), diffs)
				return mcp.NewToolResultText(preview), nil
			}

			form := url.Values{}
			form.Set("disk", disk)
			form.Set("size", size)

			path := fmt.Sprintf("/nodes/%s/lxc/%d/resize", url.PathEscape(node), vmid)
			var taskUPID string
			if err := client.Put(ctx, path, form, &taskUPID); err != nil {
				return ErrorResult(err.Error())
			}
			return mcp.NewToolResultText(fmt.Sprintf("Disk %s on LXC %d successfully resized (%s). Task UPID: %s", disk, vmid, size, taskUPID)), nil
		},
	)

	s.AddTool(
		mcp.NewTool(
			"pve_lxc_update_network",
			mcp.WithDescription("Update or configure a virtual network interface on an LXC container"),
			mcp.WithReadOnlyHintAnnotation(false),
			mcp.WithDestructiveHintAnnotation(true),
			mcp.WithString("node", mcp.Required(), mcp.Description("Node name")),
			mcp.WithInteger("vmid", mcp.Required(), mcp.Description("Container ID")),
			mcp.WithString("net_id", mcp.Required(), mcp.Description("Network interface ID (e.g. 'net0', 'net1')")),
			mcp.WithString("bridge", mcp.Description("Linux bridge to attach interface (e.g. 'vmbr0')")),
			mcp.WithString("name", mcp.Description("Interface name inside container (e.g. 'eth0')")),
			mcp.WithInteger("tag", mcp.Description("VLAN tag (1-4094)")),
			mcp.WithBoolean("firewall", mcp.Description("Enable PVE firewall on this interface")),
			mcp.WithNumber("rate", mcp.Description("Rate limit in MB/s")),
			mcp.WithString("ip", mcp.Description("IPv4 address with CIDR mask (e.g. '192.168.1.50/24') or 'dhcp'")),
			mcp.WithString("gw", mcp.Description("Default IPv4 gateway address")),
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
			if err := client.Get(ctx, fmt.Sprintf("/nodes/%s/lxc/%d/config", url.PathEscape(node), vmid), nil, &currentConfig); err != nil {
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
			if ifName := ParseOptionalString(req.Params.Arguments, "name", ""); ifName != "" {
				updates["name"] = ifName
			}
			if tag, _ := ParseOptionalInt(req.Params.Arguments, "tag", 0); tag > 0 && tag <= 4094 {
				updates["tag"] = strconv.Itoa(tag)
			}
			if ip := ParseOptionalString(req.Params.Arguments, "ip", ""); ip != "" {
				updates["ip"] = ip
			}
			if gw := ParseOptionalString(req.Params.Arguments, "gw", ""); gw != "" {
				updates["gw"] = gw
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

			baseString := currentNetStr
			if baseString == "" {
				ifName := updates["name"]
				if ifName == "" {
					ifName = "eth0"
					updates["name"] = ifName
				}
				baseString = fmt.Sprintf("name=%s", ifName)
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
				preview := FormatDryRunPreview(fmt.Sprintf("Update network interface %s on LXC %d (node %s)", netID, vmid, node), diffs)
				return mcp.NewToolResultText(preview), nil
			}

			form := url.Values{}
			form.Set(netID, mergedNet)

			path := fmt.Sprintf("/nodes/%s/lxc/%d/config", url.PathEscape(node), vmid)
			var raw json.RawMessage
			if err := client.Put(ctx, path, form, &raw); err != nil {
				return ErrorResult(err.Error())
			}
			return mcp.NewToolResultText(fmt.Sprintf("Network interface %s updated on LXC %d:\n%s", netID, vmid, mergedNet)), nil
		},
	)

	s.AddTool(
		mcp.NewTool(
			"pve_lxc_clone",
			mcp.WithDescription("Clone an existing LXC container or template to a new VMID"),
			mcp.WithReadOnlyHintAnnotation(false),
			mcp.WithDestructiveHintAnnotation(true),
			mcp.WithString("node", mcp.Required(), mcp.Description("Node name")),
			mcp.WithInteger("vmid", mcp.Required(), mcp.Description("Source Container ID")),
			mcp.WithInteger("newid", mcp.Required(), mcp.Description("Target new Container ID")),
			mcp.WithString("hostname", mcp.Description("Hostname for the new container")),
			mcp.WithString("storage", mcp.Description("Target storage pool for the container rootfs")),
			mcp.WithBoolean("full", mcp.Description("Full clone (default true) creates independent disk; false creates linked clone")),
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

			hostname := ParseOptionalString(req.Params.Arguments, "hostname", "")
			storage := ParseOptionalString(req.Params.Arguments, "storage", "")
			full := ParseOptionalBool(req.Params.Arguments, "full", true)
			wait := ParseOptionalBool(req.Params.Arguments, "wait", true)

			cloneType := "full clone"
			if !full {
				cloneType = "linked clone"
			}

			diffs := []string{
				fmt.Sprintf("Source Container ID: %d", vmid),
				fmt.Sprintf("Target Container ID: %d", newid),
				fmt.Sprintf("Clone Mode: %s", cloneType),
			}
			if hostname != "" {
				diffs = append(diffs, fmt.Sprintf("Target Hostname: %s", hostname))
			}
			if storage != "" {
				diffs = append(diffs, fmt.Sprintf("Target Storage: %s", storage))
			}

			if !IsConfirmed(req.Params.Arguments) {
				preview := FormatDryRunPreview(fmt.Sprintf("Clone LXC %d to %d (%s) on node %s", vmid, newid, cloneType, node), diffs)
				return mcp.NewToolResultText(preview), nil
			}

			form := url.Values{}
			form.Set("newid", strconv.Itoa(newid))
			if full {
				form.Set("full", "1")
			} else {
				form.Set("full", "0")
			}
			if hostname != "" {
				form.Set("hostname", hostname)
			}
			if storage != "" {
				form.Set("storage", storage)
			}

			path := fmt.Sprintf("/nodes/%s/lxc/%d/clone", url.PathEscape(node), vmid)
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
					return mcp.NewToolResultText(fmt.Sprintf("Successfully cloned LXC %d to %d (%s) on node %s. Task UPID: %s", vmid, newid, hostname, node, upid)), nil
				}
				return ErrorResult(fmt.Sprintf("Clone task failed with exit status: %s (UPID: %s)", taskStatus.ExitStatus, upid))
			}
			return mcp.NewToolResultText(fmt.Sprintf("Clone task is still running after 30s. Task UPID: %s\nUse pve_task_status to check status.", upid)), nil
		},
	)

	s.AddTool(
		mcp.NewTool(
			"pve_lxc_create",
			mcp.WithDescription("Create a new LXC container from an OS appliance template"),
			mcp.WithReadOnlyHintAnnotation(false),
			mcp.WithDestructiveHintAnnotation(true),
			mcp.WithString("node", mcp.Required(), mcp.Description("Node name")),
			mcp.WithInteger("vmid", mcp.Required(), mcp.Description("Target Container ID")),
			mcp.WithString("ostemplate", mcp.Required(), mcp.Description("OS template storage volume (e.g. 'local:vztmpl/debian-12-standard_12.7-1_amd64.tar.zst')")),
			mcp.WithString("hostname", mcp.Description("Hostname for the new container")),
			mcp.WithInteger("cores", mcp.Description("Number of CPU cores (default 2)")),
			mcp.WithInteger("memory", mcp.Description("RAM memory in Megabytes (default 1024)")),
			mcp.WithInteger("swap", mcp.Description("Swap space in Megabytes (default 512)")),
			mcp.WithString("disk", mcp.Description("Root disk size extension (e.g. '8G', default '8G')")),
			mcp.WithString("storage", mcp.Description("Storage pool for container rootfs (default 'local-lvm')")),
			mcp.WithString("bridge", mcp.Description("Network bridge (default 'vmbr0')")),
			mcp.WithString("ip", mcp.Description("IPv4 address with CIDR (e.g. '192.168.1.50/24') or 'dhcp' (default 'dhcp')")),
			mcp.WithString("gateway", mcp.Description("IPv4 default gateway")),
			mcp.WithString("password", mcp.Description("Root user password")),
			mcp.WithString("ssh_public_keys", mcp.Description("Public SSH authorized keys")),
			mcp.WithBoolean("unprivileged", mcp.Description("Create as unprivileged container (default true)")),
			mcp.WithBoolean("start", mcp.Description("Start container immediately after creation (default false)")),
			mcp.WithBoolean("wait", mcp.Description("Wait up to 30 seconds for creation task to complete (default true)")),
			mcp.WithBoolean("confirm", mcp.Description("Set to true after user confirmation to execute creation; if false/omitted, returns a dry-run preview")),
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
			ostemplate, err := ParseRequiredString(req.Params.Arguments, "ostemplate")
			if err != nil {
				return ErrorResult(err.Error())
			}

			hostname := ParseOptionalString(req.Params.Arguments, "hostname", fmt.Sprintf("ct-%d", vmid))
			cores, _ := ParseOptionalInt(req.Params.Arguments, "cores", 2)
			memory, _ := ParseOptionalInt(req.Params.Arguments, "memory", 1024)
			swap, _ := ParseOptionalInt(req.Params.Arguments, "swap", 512)
			diskSize := ParseOptionalString(req.Params.Arguments, "disk", "8G")
			storage := ParseOptionalString(req.Params.Arguments, "storage", "local-lvm")
			bridge := ParseOptionalString(req.Params.Arguments, "bridge", "vmbr0")
			ip := ParseOptionalString(req.Params.Arguments, "ip", "dhcp")
			gateway := ParseOptionalString(req.Params.Arguments, "gateway", "")
			password := ParseOptionalString(req.Params.Arguments, "password", "")
			sshKeys := ParseOptionalString(req.Params.Arguments, "ssh_public_keys", "")
			unprivileged := ParseOptionalBool(req.Params.Arguments, "unprivileged", true)
			startAfter := ParseOptionalBool(req.Params.Arguments, "start", false)
			wait := ParseOptionalBool(req.Params.Arguments, "wait", true)
			diskGB := strings.TrimRight(strings.ToUpper(strings.TrimSpace(diskSize)), "GIB ")
			if diskGB == "" {
				diskGB = "8"
			}
			rootfsParam := fmt.Sprintf("%s:%s", storage, diskGB)
			net0Param := fmt.Sprintf("name=eth0,bridge=%s,ip=%s", bridge, ip)
			if gateway != "" {
				net0Param = fmt.Sprintf("%s,gw=%s", net0Param, gateway)
			}

			diffs := []string{
				fmt.Sprintf("Container ID: %d", vmid),
				fmt.Sprintf("Hostname: %s", hostname),
				fmt.Sprintf("OS Template: %s", ostemplate),
				fmt.Sprintf("Cores: %d", cores),
				fmt.Sprintf("Memory: %d MB", memory),
				fmt.Sprintf("Swap: %d MB", swap),
				fmt.Sprintf("Rootfs: %s", rootfsParam),
				fmt.Sprintf("Network: %s", net0Param),
				fmt.Sprintf("Unprivileged: %v", unprivileged),
				fmt.Sprintf("Auto-Start: %v", startAfter),
			}

			if !IsConfirmed(req.Params.Arguments) {
				preview := FormatDryRunPreview(fmt.Sprintf("CREATE LXC %d (%s) on node %s", vmid, hostname, node), diffs)
				return mcp.NewToolResultText(preview), nil
			}

			form := url.Values{}
			form.Set("vmid", strconv.Itoa(vmid))
			form.Set("ostemplate", ostemplate)
			form.Set("hostname", hostname)
			form.Set("cores", strconv.Itoa(cores))
			form.Set("memory", strconv.Itoa(memory))
			form.Set("swap", strconv.Itoa(swap))
			form.Set("rootfs", rootfsParam)
			form.Set("net0", net0Param)
			if unprivileged {
				form.Set("unprivileged", "1")
			} else {
				form.Set("unprivileged", "0")
			}
			if startAfter {
				form.Set("start", "1")
			}
			if password != "" {
				form.Set("password", password)
			}
			if sshKeys != "" {
				form.Set("ssh-public-keys", sshKeys)
			}

			path := fmt.Sprintf("/nodes/%s/lxc", url.PathEscape(node))
			var upid string
			if err := client.Post(ctx, path, form, &upid); err != nil {
				return ErrorResult(err.Error())
			}

			if !wait {
				return mcp.NewToolResultText(fmt.Sprintf("Create task initiated successfully for LXC %d. UPID: %s\nUse pve_task_status to monitor progress.", vmid, upid)), nil
			}

			taskStatus, err := WaitForTask(ctx, client, node, upid, 30, req.Params.Meta)
			if err != nil {
				return mcp.NewToolResultText(fmt.Sprintf("Create task initiated (UPID: %s), but wait encountered error: %v", upid, err)), nil
			}
			if taskStatus.Status == "stopped" {
				if taskStatus.ExitStatus == "OK" {
					return mcp.NewToolResultText(fmt.Sprintf("Successfully created LXC %d (%s) on node %s. Task UPID: %s", vmid, hostname, node, upid)), nil
				}
				return ErrorResult(fmt.Sprintf("Create task failed with exit status: %s (UPID: %s)", taskStatus.ExitStatus, upid))
			}
			return mcp.NewToolResultText(fmt.Sprintf("Create task is still running after 30s. Task UPID: %s\nUse pve_task_status to check status.", upid)), nil
		},
	)

	s.AddTool(
		mcp.NewTool(
			"pve_lxc_destroy",
			mcp.WithDescription("Permanently delete an LXC container and its rootfs storage disks"),
			mcp.WithReadOnlyHintAnnotation(false),
			mcp.WithDestructiveHintAnnotation(true),
			mcp.WithString("node", mcp.Required(), mcp.Description("Node name")),
			mcp.WithInteger("vmid", mcp.Required(), mcp.Description("Container ID to permanently delete")),
			mcp.WithString("expected_name", mcp.Required(), mcp.Description("Current name or hostname of the container to confirm identity before destruction")),
			mcp.WithBoolean("purge", mcp.Description("Remove container from backup jobs and purge disk configurations (default true)")),
			mcp.WithBoolean("destroy_disks", mcp.Description("Remove and erase all associated mountpoints and disks (default true)")),
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
			if err := client.Get(ctx, fmt.Sprintf("/nodes/%s/lxc/%d/status/current", url.PathEscape(node), vmid), nil, &statusData); err != nil {
				return ErrorResult(err.Error())
			}

			if err := VerifyTargetIdentity(extractArgs(req.Params.Arguments), statusData.Name, true); err != nil {
				return ErrorResult(err.Error())
			}

			if statusData.Status != "stopped" {
				return ErrorResult(fmt.Sprintf("cannot destroy LXC %d while it is %q. Stop or shutdown the container first.", vmid, statusData.Status))
			}

			unprotect := ParseOptionalBool(req.Params.Arguments, "unprotect", false)
			var configData map[string]any
			_ = client.Get(ctx, fmt.Sprintf("/nodes/%s/lxc/%d/config", url.PathEscape(node), vmid), nil, &configData)
			if protVal, ok := configData["protection"]; ok {
				if protStr := fmt.Sprintf("%v", protVal); protStr == "1" || protStr == "true" {
					if !unprotect {
						return ErrorResult(fmt.Sprintf("LXC %d has protection flag enabled. Pass unprotect: true to confirm removing protection and destroying.", vmid))
					}
					unprotForm := url.Values{}
					unprotForm.Set("protection", "0")
					_ = client.Put(ctx, fmt.Sprintf("/nodes/%s/lxc/%d/config", url.PathEscape(node), vmid), unprotForm, nil)
				}
			}

			diffs := []string{
				fmt.Sprintf("Target Container ID: %d", vmid),
				fmt.Sprintf("Target Name: %s", statusData.Name),
				fmt.Sprintf("Verified Identity: %s", statusData.Name),
				fmt.Sprintf("Status: %s", statusData.Status),
				"Action: PERMANENT DELETION of LXC container and all rootfs/mountpoint storage",
			}

			if !IsConfirmed(req.Params.Arguments) {
				preview := FormatDryRunPreview(fmt.Sprintf("DESTROY LXC %d (%s) on node %s", vmid, statusData.Name, node), diffs)
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

			path := fmt.Sprintf("/nodes/%s/lxc/%d", url.PathEscape(node), vmid)
			var upid string
			if err := client.Delete(ctx, path, form, &upid); err != nil {
				return ErrorResult(err.Error())
			}

			return mcp.NewToolResultText(fmt.Sprintf("Destroy task initiated for LXC %d on node %s. Task UPID: %s", vmid, node, upid)), nil
		},
	)

	s.AddTool(
		mcp.NewTool(
			"pve_lxc_protection",
			mcp.WithDescription("Set or remove the deletion protection flag on an LXC container"),
			mcp.WithReadOnlyHintAnnotation(false),
			mcp.WithDestructiveHintAnnotation(true),
			mcp.WithString("node", mcp.Required(), mcp.Description("Node name")),
			mcp.WithInteger("vmid", mcp.Required(), mcp.Description("Container ID")),
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
			_ = client.Get(ctx, fmt.Sprintf("/nodes/%s/lxc/%d/config", url.PathEscape(node), vmid), nil, &configData)
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
				preview := FormatDryRunPreview(fmt.Sprintf("Set protection to %s on LXC %d (node %s)", targetProtection, vmid, node), diffs)
				return mcp.NewToolResultText(preview), nil
			}

			form := url.Values{}
			form.Set("protection", targetVal)

			path := fmt.Sprintf("/nodes/%s/lxc/%d/config", url.PathEscape(node), vmid)
			var raw json.RawMessage
			if err := client.Put(ctx, path, form, &raw); err != nil {
				return ErrorResult(err.Error())
			}

			return mcp.NewToolResultText(fmt.Sprintf("Protection successfully set to %s for LXC %d on node %s.", targetProtection, vmid, node)), nil
		},
	)
}
