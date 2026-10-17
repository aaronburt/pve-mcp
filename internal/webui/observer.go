package webui

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync/atomic"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	mcpserver "github.com/mark3labs/mcp-go/server"
)

// Observer feeds MCP activity and server logs into a Buffer. It is only
// created when the UI is enabled at boot, so a disabled UI installs nothing.
type Observer struct {
	buf      *Buffer
	sanitize func(string) string
	srv      atomic.Pointer[mcpserver.MCPServer]
}

func NewObserver(buf *Buffer, sanitize func(string) string) *Observer {
	if sanitize == nil {
		sanitize = func(s string) string { return s }
	}
	return &Observer{buf: buf, sanitize: sanitize}
}

// SetServer lets the observer look up tool annotations to classify calls.
func (o *Observer) SetServer(s *mcpserver.MCPServer) { o.srv.Store(s) }

func (o *Observer) add(e Entry) {
	e.Message = o.sanitize(e.Message)
	e.Detail = o.sanitize(e.Detail)
	o.buf.Add(e)
}

func (o *Observer) readOnly(name string) bool {
	s := o.srv.Load()
	if s == nil {
		return false
	}
	t := s.GetTool(name)
	return t != nil && t.Tool.Annotations.ReadOnlyHint != nil && *t.Tool.Annotations.ReadOnlyHint
}

func classifyTool(readOnly bool, args any, result *mcp.CallToolResult, err error) string {
	switch {
	case err != nil || (result != nil && result.IsError):
		return KindToolError
	case readOnly:
		return KindToolRead
	}
	if m, ok := args.(map[string]any); ok {
		if c, ok := m["confirm"].(bool); ok && c {
			return KindToolMutation
		}
	}
	return KindToolDryRun
}

func resultText(r *mcp.CallToolResult) string {
	if r == nil {
		return ""
	}
	for _, c := range r.Content {
		if t, ok := c.(mcp.TextContent); ok {
			return t.Text
		}
	}
	return ""
}

// Middleware records every tool call with its arguments, outcome and duration.
func (o *Observer) Middleware() mcpserver.ToolHandlerMiddleware {
	return func(next mcpserver.ToolHandlerFunc) mcpserver.ToolHandlerFunc {
		return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			start := time.Now()
			res, err := next(ctx, req)

			kind := classifyTool(o.readOnly(req.Params.Name), req.Params.Arguments, res, err)
			args, _ := json.Marshal(req.Params.Arguments)
			detail := "args: " + string(args)
			if err != nil {
				detail += "\nerror: " + err.Error()
			} else if txt := resultText(res); txt != "" {
				detail += "\nresult: " + txt
			}
			o.add(Entry{
				Kind:     kind,
				Message:  req.Params.Name,
				Detail:   detail,
				Duration: time.Since(start).Milliseconds(),
			})
			return res, err
		}
	}
}

// Hooks records resource reads, prompt fetches and protocol errors.
func (o *Observer) Hooks() *mcpserver.Hooks {
	h := &mcpserver.Hooks{}
	h.AddAfterReadResource(func(ctx context.Context, id any, req *mcp.ReadResourceRequest, res *mcp.ReadResourceResult) {
		o.add(Entry{Kind: KindResourceRead, Message: req.Params.URI})
	})
	h.AddAfterGetPrompt(func(ctx context.Context, id any, req *mcp.GetPromptRequest, res *mcp.GetPromptResult) {
		o.add(Entry{Kind: KindPromptGet, Message: req.Params.Name})
	})
	h.AddOnError(func(ctx context.Context, id any, method mcp.MCPMethod, message any, err error) {
		o.add(Entry{
			Kind:    KindToolError,
			Level:   "ERROR",
			Message: fmt.Sprintf("%s: %v", method, err),
		})
	})
	return h
}

// LogHandler tees records into the buffer while forwarding them to next.
func (o *Observer) LogHandler(next slog.Handler) slog.Handler {
	return &teeHandler{next: next, obs: o}
}

type teeHandler struct {
	next slog.Handler
	obs  *Observer
	pre  []slog.Attr
	grp  string
}

func (h *teeHandler) Enabled(ctx context.Context, l slog.Level) bool {
	return h.next.Enabled(ctx, l)
}

func (h *teeHandler) Handle(ctx context.Context, r slog.Record) error {
	detail := ""
	add := func(a slog.Attr) bool {
		detail += fmt.Sprintf("%s%s=%v\n", h.grp, a.Key, a.Value.Resolve().Any())
		return true
	}
	for _, a := range h.pre {
		add(a)
	}
	r.Attrs(add)
	h.obs.add(Entry{
		Time:    r.Time.UTC(),
		Kind:    KindServerLog,
		Level:   r.Level.String(),
		Message: r.Message,
		Detail:  detail,
	})
	return h.next.Handle(ctx, r)
}

func (h *teeHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	c := *h
	c.next = h.next.WithAttrs(attrs)
	c.pre = append(append([]slog.Attr(nil), h.pre...), attrs...)
	return &c
}

func (h *teeHandler) WithGroup(name string) slog.Handler {
	c := *h
	c.next = h.next.WithGroup(name)
	c.grp = h.grp + name + "."
	return &c
}
