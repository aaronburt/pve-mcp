package tools

import (
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
)

var (
	nodeRegex    = regexp.MustCompile(`^[a-zA-Z0-9_\-]+$`)
	storageRegex = regexp.MustCompile(`^[a-zA-Z0-9_\.\-]+$`)
)

type TruncatedResponse[T any] struct {
	Total     int  `json:"total"`
	Limit     int  `json:"limit"`
	Truncated bool `json:"truncated"`
	Items     []T  `json:"items"`
}

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

func ParseRequiredString(args map[string]any, key string) (string, error) {
	raw, ok := args[key]
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

func ParseOptionalString(args map[string]any, key string, fallback string) string {
	raw, ok := args[key]
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

func ParseRequiredInt(args map[string]any, key string) (int, error) {
	raw, ok := args[key]
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

func ParseOptionalInt(args map[string]any, key string, fallback int) (int, error) {
	raw, ok := args[key]
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
		b, err := json.MarshalIndent(data, "", "  ")
		if err != nil {
			return nil, errors.New("failed to marshal json result")
		}
		return mcp.NewToolResultText(string(b)), nil
	}
}

func ErrorResult(msg string) (*mcp.CallToolResult, error) {
	return mcp.NewToolResultError(msg), nil
}
