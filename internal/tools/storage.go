package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	"github.com/aaronburt/pve-mcp/internal/pve"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func RegisterStorageTools(s *server.MCPServer, client *pve.Client) {
	s.AddTool(
		mcp.NewTool(
			"pve_storage_list",
			mcp.WithDescription("List storage pools accessible from a node"),
			mcp.WithReadOnlyHintAnnotation(true),
			mcp.WithString("node", mcp.Required(), mcp.Description("Node name")),
			mcp.WithString("mode", mcp.Description("Output mode ('compact' for tabular text, 'full' for raw API output)"), mcp.Enum("compact", "full")),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			rawNode, err := ParseRequiredString(req.Params.Arguments, "node")
			if err != nil {
				return ErrorResult(err.Error())
			}
			node, err := ValidateNode(rawNode)
			if err != nil {
				return ErrorResult(err.Error())
			}
			path := fmt.Sprintf("/nodes/%s/storage", url.PathEscape(node))

			if !IsCompactMode(req.Params.Arguments) {
				var data json.RawMessage
				if err := client.Get(ctx, path, nil, &data); err != nil {
					return ErrorResult(err.Error())
				}
				return JSONResult(data)
			}

			var storages []pve.StorageItem
			if err := client.Get(ctx, path, nil, &storages); err != nil {
				return ErrorResult(err.Error())
			}

			var sb strings.Builder
			sb.WriteString("STORAGE\tTYPE\tTOTAL_GB\tUSED_GB\tAVAIL_GB\tSHARED\tCONTENT\n")
			for _, st := range storages {
				fmt.Fprintf(&sb, "%s\t%s\t%d\t%d\t%d\t%d\t%s\n",
					st.Storage, st.Type,
					st.Total/(1024*1024*1024), st.Used/(1024*1024*1024),
					st.Avail/(1024*1024*1024), st.Shared, st.Content,
				)
			}
			return mcp.NewToolResultText(sb.String()), nil
		},
	)

	s.AddTool(
		mcp.NewTool(
			"pve_storage_status",
			mcp.WithDescription("Get the status of a specific storage pool on a node"),
			mcp.WithReadOnlyHintAnnotation(true),
			mcp.WithString("node", mcp.Required(), mcp.Description("Node name")),
			mcp.WithString("storage", mcp.Required(), mcp.Description("Storage pool name")),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			rawNode, err := ParseRequiredString(req.Params.Arguments, "node")
			if err != nil {
				return ErrorResult(err.Error())
			}
			node, err := ValidateNode(rawNode)
			if err != nil {
				return ErrorResult(err.Error())
			}

			rawStorage, err := ParseRequiredString(req.Params.Arguments, "storage")
			if err != nil {
				return ErrorResult(err.Error())
			}
			storage, err := ValidateStorage(rawStorage)
			if err != nil {
				return ErrorResult(err.Error())
			}

			path := fmt.Sprintf("/nodes/%s/storage/%s/status", url.PathEscape(node), url.PathEscape(storage))
			var data json.RawMessage
			if err := client.Get(ctx, path, nil, &data); err != nil {
				return ErrorResult(err.Error())
			}
			return JSONResult(data)
		},
	)

	s.AddTool(
		mcp.NewTool(
			"pve_storage_content",
			mcp.WithDescription("List volume content inside a storage pool"),
			mcp.WithReadOnlyHintAnnotation(true),
			mcp.WithString("node", mcp.Required(), mcp.Description("Node name")),
			mcp.WithString("storage", mcp.Required(), mcp.Description("Storage pool name")),
			mcp.WithString("content", mcp.Description("Filter by content type (iso, vztmpl, backup, images, rootdir, snippets)")),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			rawNode, err := ParseRequiredString(req.Params.Arguments, "node")
			if err != nil {
				return ErrorResult(err.Error())
			}
			node, err := ValidateNode(rawNode)
			if err != nil {
				return ErrorResult(err.Error())
			}

			rawStorage, err := ParseRequiredString(req.Params.Arguments, "storage")
			if err != nil {
				return ErrorResult(err.Error())
			}
			storage, err := ValidateStorage(rawStorage)
			if err != nil {
				return ErrorResult(err.Error())
			}

			query := url.Values{}
			if content := ParseOptionalString(req.Params.Arguments, "content", ""); content != "" {
				query.Set("content", content)
			}

			path := fmt.Sprintf("/nodes/%s/storage/%s/content", url.PathEscape(node), url.PathEscape(storage))
			var data json.RawMessage
			if err := client.Get(ctx, path, query, &data); err != nil {
				return ErrorResult(err.Error())
			}
			return JSONResult(data)
		},
	)
}
