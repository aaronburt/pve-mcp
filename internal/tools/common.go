package tools

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/aaronburt/pve-mcp/internal/config"
	"github.com/aaronburt/pve-mcp/internal/pve"
	"github.com/mark3labs/mcp-go/mcp"
	mcpserver "github.com/mark3labs/mcp-go/server"
)

var (
	nodeRegex        = regexp.MustCompile(`^[a-zA-Z0-9_\-]+$`)
	storageRegex     = regexp.MustCompile(`^[a-zA-Z0-9_\.\-]+$`)
	vmDiskRegex      = regexp.MustCompile(`^(scsi|virtio|ide|sata)[0-9]+$`)
	lxcDiskRegex     = regexp.MustCompile(`^(rootfs|mp[0-9]+)$`)
	diskSizeRegex    = regexp.MustCompile(`^\+?[0-9]+[KMGTkmgt]$`)
	netIDRegex       = regexp.MustCompile(`^net[0-9]+$`)
	powerActionRegex = regexp.MustCompile(`^(start|stop|shutdown|reboot|suspend|resume)$`)
)

func ValidateNode(node string) (string, error) {
	trimmed := strings.TrimSpace(node)
	if trimmed == "" || !nodeRegex.MatchString(trimmed) {
		return "", fmt.Errorf("invalid node name: %q", node)
	}
	return trimmed, nil
}

func ValidateStorage(storage string) (string, error) {
	trimmed := strings.TrimSpace(storage)
	if trimmed == "" || !storageRegex.MatchString(trimmed) {
		return "", fmt.Errorf("invalid storage name: %q", storage)
	}
	return trimmed, nil
}

func ValidateVMID(vmid int) (int, error) {
	if vmid < 100 || vmid > 999999999 {
		return 0, fmt.Errorf("vmid %d out of valid range (100-999999999)", vmid)
	}
	return vmid, nil
}

func extractArgs(args any) map[string]any {
	if args == nil {
		return nil
	}
	if m, ok := args.(map[string]any); ok {
		return m
	}
	return nil
}

func ParseRequiredString(args any, key string) (string, error) {
	m := extractArgs(args)
	if m == nil {
		return "", fmt.Errorf("missing required parameter: %s", key)
	}
	raw, ok := m[key]
	if !ok || raw == nil {
		return "", fmt.Errorf("missing required parameter: %s", key)
	}
	str, ok := raw.(string)
	if !ok {
		return "", fmt.Errorf("parameter %s must be a string", key)
	}
	trimmed := strings.TrimSpace(str)
	if trimmed == "" {
		return "", fmt.Errorf("parameter %s cannot be empty", key)
	}
	return trimmed, nil
}

func ParseOptionalString(args any, key string, fallback string) string {
	m := extractArgs(args)
	if m == nil {
		return fallback
	}
	raw, ok := m[key]
	if !ok || raw == nil {
		return fallback
	}
	str, ok := raw.(string)
	if !ok {
		return fallback
	}
	trimmed := strings.TrimSpace(str)
	if trimmed == "" {
		return fallback
	}
	return trimmed
}

func ParseRequiredInt(args any, key string) (int, error) {
	m := extractArgs(args)
	if m == nil {
		return 0, fmt.Errorf("missing required parameter: %s", key)
	}
	raw, ok := m[key]
	if !ok || raw == nil {
		return 0, fmt.Errorf("missing required parameter: %s", key)
	}

	switch v := raw.(type) {
	case int:
		return v, nil
	case int32:
		return int(v), nil
	case int64:
		return int(v), nil
	case float64:
		return int(v), nil
	case float32:
		return int(v), nil
	case json.Number:
		i, err := v.Int64()
		if err != nil {
			return 0, fmt.Errorf("invalid integer for parameter %s: %w", key, err)
		}
		return int(i), nil
	case string:
		i, err := strconv.Atoi(strings.TrimSpace(v))
		if err != nil {
			return 0, fmt.Errorf("invalid integer string for parameter %s: %w", key, err)
		}
		return i, nil
	default:
		return 0, fmt.Errorf("parameter %s must be an integer", key)
	}
}

func ParseOptionalInt(args any, key string, fallback int) (int, error) {
	m := extractArgs(args)
	if m == nil {
		return fallback, nil
	}
	raw, ok := m[key]
	if !ok || raw == nil {
		return fallback, nil
	}
	return ParseRequiredInt(args, key)
}

func TruncateList[T any](items []T, max int) ([]T, int, bool) {
	total := len(items)
	if max > 0 && total > max {
		return items[:max], total, true
	}
	return items, total, false
}

func JSONResult(data any) (*mcp.CallToolResult, error) {
	if data == nil {
		return mcp.NewToolResultText("null"), nil
	}

	switch v := data.(type) {
	case string:
		return mcp.NewToolResultText(v), nil
	case []byte:
		return mcp.NewToolResultText(string(v)), nil
	case json.RawMessage:
		return mcp.NewToolResultText(string(v)), nil
	default:
		b, err := json.Marshal(data)
		if err != nil {
			return nil, errors.New("failed to marshal json result")
		}
		return mcp.NewToolResultText(string(b)), nil
	}
}

func ErrorResult(msg string) (*mcp.CallToolResult, error) {
	return mcp.NewToolResultError(msg), nil
}

func ParseOptionalBool(args any, key string, fallback bool) bool {
	argMap := extractArgs(args)
	if argMap == nil {
		return fallback
	}
	raw, ok := argMap[key]
	if !ok || raw == nil {
		return fallback
	}
	switch val := raw.(type) {
	case bool:
		return val
	case string:
		lower := strings.ToLower(strings.TrimSpace(val))
		return lower == "true" || lower == "1" || lower == "yes"
	default:
		return fallback
	}
}

func ParseRequiredBool(args any, key string) (bool, error) {
	argMap := extractArgs(args)
	if argMap == nil {
		return false, fmt.Errorf("missing required parameter: %s", key)
	}
	raw, ok := argMap[key]
	if !ok || raw == nil {
		return false, fmt.Errorf("missing required parameter: %s", key)
	}
	switch val := raw.(type) {
	case bool:
		return val, nil
	case string:
		lower := strings.ToLower(strings.TrimSpace(val))
		if lower == "true" || lower == "1" || lower == "yes" {
			return true, nil
		}
		if lower == "false" || lower == "0" || lower == "no" {
			return false, nil
		}
		return false, fmt.Errorf("parameter %s must be a boolean", key)
	default:
		return false, fmt.Errorf("parameter %s must be a boolean", key)
	}
}

func IsConfirmed(args any) bool {
	return ParseOptionalBool(args, "confirm", false)
}

func IsForce(args any) bool {
	return ParseOptionalBool(args, "force", false)
}

func EnsureMutationsAllowed(cfg *config.Config) error {
	if cfg == nil || !cfg.AllowMutations {
		return errors.New("mutations are disabled. Set PVE_ALLOW_MUTATIONS=true in server environment to enable state-changing operations")
	}
	return nil
}

func EnsureDestroyAllowed(cfg *config.Config) error {
	if cfg == nil || !cfg.AllowDestroy {
		return errors.New("machine destruction is disabled. Set PVE_ALLOW_DESTROY=true in server environment to enable permanent deletion of VMs and containers")
	}
	return nil
}

func EmitProgress(ctx context.Context, meta *mcp.Meta, progress, total float64) {
	if meta == nil || meta.ProgressToken == nil {
		return
	}
	mcpServer := mcpserver.ServerFromContext(ctx)
	if mcpServer == nil {
		return
	}
	_ = mcpServer.SendNotificationToClient(ctx, "notifications/progress", map[string]any{
		"progressToken": meta.ProgressToken,
		"progress":      progress,
		"total":         total,
	})
}

func WaitForTask(ctx context.Context, client *pve.Client, node string, upid string, timeoutSec int, meta *mcp.Meta) (*pve.TaskStatus, error) {
	if timeoutSec <= 0 {
		timeoutSec = 30
	}
	startTime := time.Now()
	deadline := startTime.Add(time.Duration(timeoutSec) * time.Second)
	EmitProgress(ctx, meta, 0, float64(timeoutSec))

	for {
		status, err := client.GetTaskStatus(ctx, node, upid)
		if err != nil {
			return nil, err
		}
		if status.Status == "stopped" {
			EmitProgress(ctx, meta, float64(timeoutSec), float64(timeoutSec))
			return status, nil
		}

		if time.Now().After(deadline) {
			return status, nil
		}

		elapsed := time.Since(startTime).Seconds()
		EmitProgress(ctx, meta, elapsed, float64(timeoutSec))

		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(1 * time.Second):
		}
	}
}

func ValidatePowerAction(action string) (string, error) {
	trimmed := strings.ToLower(strings.TrimSpace(action))
	if !powerActionRegex.MatchString(trimmed) {
		return "", fmt.Errorf("invalid power action %q (supported: start, stop, shutdown, reboot, suspend, resume)", action)
	}
	return trimmed, nil
}

func ValidateDiskIdentifier(disk string, isVM bool) (string, error) {
	trimmed := strings.ToLower(strings.TrimSpace(disk))
	if isVM {
		if !vmDiskRegex.MatchString(trimmed) {
			return "", fmt.Errorf("invalid vm disk identifier %q (expected format like scsi0, virtio0, ide0, sata0)", disk)
		}
	} else {
		if !lxcDiskRegex.MatchString(trimmed) {
			return "", fmt.Errorf("invalid lxc disk identifier %q (expected rootfs or mp0, mp1, etc.)", disk)
		}
	}
	return trimmed, nil
}

func ValidateDiskSize(size string) (string, error) {
	trimmed := strings.TrimSpace(size)
	if !diskSizeRegex.MatchString(trimmed) {
		return "", fmt.Errorf("invalid disk size %q (expected format like +10G, 50G, +500M)", size)
	}
	return trimmed, nil
}

func ParseSizeBytes(sizeStr string) (int64, error) {
	trimmed := strings.TrimSpace(sizeStr)
	if len(trimmed) < 2 {
		return 0, fmt.Errorf("invalid size %q", sizeStr)
	}
	unit := strings.ToUpper(trimmed[len(trimmed)-1:])
	numStr := strings.TrimPrefix(trimmed[:len(trimmed)-1], "+")
	val, err := strconv.ParseInt(numStr, 10, 64)
	if err != nil || val <= 0 {
		return 0, fmt.Errorf("invalid size value in %q", sizeStr)
	}
	switch unit {
	case "K":
		return val * 1024, nil
	case "M":
		return val * 1024 * 1024, nil
	case "G":
		return val * 1024 * 1024 * 1024, nil
	case "T":
		return val * 1024 * 1024 * 1024 * 1024, nil
	default:
		return 0, fmt.Errorf("unknown size unit in %q", sizeStr)
	}
}

var diskConfigSizeRegex = regexp.MustCompile(`size=([0-9]+[KMGTkmgt])`)

func ExtractDiskSizeBytes(configStr string) int64 {
	matches := diskConfigSizeRegex.FindStringSubmatch(configStr)
	if len(matches) < 2 {
		return 0
	}
	bytes, err := ParseSizeBytes(matches[1])
	if err != nil {
		return 0
	}
	return bytes
}

func ValidateDiskGrowth(currentBytes int64, sizeParam string) error {
	trimmed := strings.TrimSpace(sizeParam)
	if strings.HasPrefix(trimmed, "+") {
		return nil
	}
	targetBytes, err := ParseSizeBytes(trimmed)
	if err != nil {
		return err
	}
	if currentBytes > 0 && targetBytes <= currentBytes {
		return fmt.Errorf("target disk size (%s) must be greater than current size (%d bytes); shrinking virtual disks is forbidden", sizeParam, currentBytes)
	}
	return nil
}

func ValidateNetID(netID string) (string, error) {
	trimmed := strings.ToLower(strings.TrimSpace(netID))
	if !netIDRegex.MatchString(trimmed) {
		return "", fmt.Errorf("invalid network interface id %q (expected net0, net1, etc.)", netID)
	}
	return trimmed, nil
}

func MergeNetworkConfig(current string, updates map[string]string) string {
	parts := strings.Split(strings.TrimSpace(current), ",")
	order := make([]string, 0, len(parts))
	kvMap := make(map[string]string)

	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed == "" {
			continue
		}
		eqIdx := strings.Index(trimmed, "=")
		if eqIdx == -1 {
			continue
		}
		key := strings.TrimSpace(trimmed[:eqIdx])
		val := strings.TrimSpace(trimmed[eqIdx+1:])
		order = append(order, key)
		kvMap[key] = val
	}

	for k, v := range updates {
		if _, exists := kvMap[k]; !exists {
			order = append(order, k)
		}
		kvMap[k] = v
	}

	resultParts := make([]string, 0, len(order))
	for _, k := range order {
		resultParts = append(resultParts, fmt.Sprintf("%s=%s", k, kvMap[k]))
	}
	return strings.Join(resultParts, ",")
}

func FormatDryRunPreview(title string, diffs []string) string {
	var sb strings.Builder
	sb.WriteString("[DRY RUN PREVIEW - USER CONFIRMATION REQUIRED]\n")
	sb.WriteString(title)
	sb.WriteString("\n\nProposed Changes:\n")
	for _, diff := range diffs {
		sb.WriteString("  - ")
		sb.WriteString(diff)
		sb.WriteString("\n")
	}
	sb.WriteString("\nTo apply these changes:\n")
	sb.WriteString("1. Present this exact plan to the user and obtain their explicit approval.\n")
	sb.WriteString("2. Re-run this tool with parameter \"confirm\": true.\n")
	return sb.String()
}

func VerifyTargetIdentity(args map[string]any, liveName string, required bool) error {
	rawName, ok := args["expected_name"]
	if !ok || rawName == nil || strings.TrimSpace(fmt.Sprintf("%v", rawName)) == "" {
		if required {
			return errors.New("expected_name is required to confirm target machine identity before proceeding")
		}
		return nil
	}
	expectedName := strings.TrimSpace(fmt.Sprintf("%v", rawName))
	if expectedName != liveName {
		return fmt.Errorf("target identity mismatch: live machine name is %q, but expected_name was %q", liveName, expectedName)
	}
	return nil
}

