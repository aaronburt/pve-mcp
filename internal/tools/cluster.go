package tools

import (
	"context"

	"github.com/aaronburt/pve-mcp/internal/pve"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func RegisterClusterTools(s *server.MCPServer, client *pve.Client) {
	s.AddTool(
		mcp.NewTool(
			"pve_task_status",
			mcp.WithDescription("Get the status, exit code, and progress of an asynchronous Proxmox background task by UPID"),
			mcp.WithReadOnlyHintAnnotation(true),
			mcp.WithString("node", mcp.Required(), mcp.Description("Node name where the task was initiated")),
			mcp.WithString("upid", mcp.Required(), mcp.Description("Proxmox Unique Process ID (UPID)")),
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
			upid, err := ParseRequiredString(req.Params.Arguments, "upid")
			if err != nil {
				return ErrorResult(err.Error())
			}

			status, err := client.GetTaskStatus(ctx, node, upid)
			if err != nil {
				return ErrorResult(err.Error())
			}
			return JSONResult(status)
		},
	)
}
