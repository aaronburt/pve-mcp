package webui

import (
	"bufio"
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	mcpserver "github.com/mark3labs/mcp-go/server"
)

func TestBufferBoundsAndOrder(t *testing.T) {
	b := NewBuffer()
	for i := 0; i < BufferCapacity+50; i++ {
		b.Add(Entry{Kind: KindServerLog, Message: "m"})
	}
	snap := b.Snapshot()
	if len(snap) != BufferCapacity {
		t.Fatalf("expected %d entries, got %d", BufferCapacity, len(snap))
	}
	if snap[0].ID != 51 || snap[len(snap)-1].ID != uint64(BufferCapacity+50) {
		t.Fatalf("expected oldest 51 and newest %d, got %d..%d", BufferCapacity+50, snap[0].ID, snap[len(snap)-1].ID)
	}
}

func TestBufferTruncatesDetail(t *testing.T) {
	b := NewBuffer()
	b.Add(Entry{Detail: strings.Repeat("é", MaxDetailBytes)})
	d := b.Snapshot()[0].Detail
	if len(d) > MaxDetailBytes+len("…[truncated]") {
		t.Fatalf("detail not truncated: %d bytes", len(d))
	}
	if !strings.HasSuffix(d, "…[truncated]") {
		t.Fatal("expected truncation marker")
	}
}

func TestBufferCloseFreesAndClosesSubscribers(t *testing.T) {
	b := NewBuffer()
	b.Add(Entry{Message: "x"})
	_, live, _ := b.Subscribe()
	b.Close()
	if _, open := <-live; open {
		t.Fatal("subscriber channel should be closed")
	}
	if b.ring != nil {
		t.Fatal("ring should be nil after Close")
	}
	b.Add(Entry{Message: "ignored"})
	if len(b.Snapshot()) != 0 {
		t.Fatal("Add after Close should be a no-op")
	}
	_, live2, cancel := b.Subscribe()
	defer cancel()
	if _, open := <-live2; open {
		t.Fatal("subscribe after Close should return a closed channel")
	}
}

func TestBufferDropsSlowSubscriber(t *testing.T) {
	b := NewBuffer()
	_, live, _ := b.Subscribe()
	for i := 0; i < subscriberQueue+5; i++ {
		b.Add(Entry{Message: "x"})
	}
	n := 0
	for range live {
		n++
	}
	if n != subscriberQueue {
		t.Fatalf("expected %d queued entries before close, got %d", subscriberQueue, n)
	}
	b.Add(Entry{Message: "still works"})
}

func TestBufferClear(t *testing.T) {
	b := NewBuffer()
	b.Add(Entry{Message: "x"})
	b.Clear()
	if len(b.Snapshot()) != 0 {
		t.Fatal("expected empty after Clear")
	}
	b.Add(Entry{Message: "y"})
	if got := b.Snapshot(); len(got) != 1 || got[0].Message != "y" {
		t.Fatalf("unexpected after clear+add: %+v", got)
	}
}

func TestClassifyTool(t *testing.T) {
	ok := &mcp.CallToolResult{}
	bad := &mcp.CallToolResult{IsError: true}
	confirm := map[string]any{"confirm": true}
	noConfirm := map[string]any{"confirm": false}
	cases := []struct {
		name     string
		readOnly bool
		args     any
		res      *mcp.CallToolResult
		err      error
		want     string
	}{
		{"read only", true, nil, ok, nil, KindToolRead},
		{"read only never dry run", true, noConfirm, ok, nil, KindToolRead},
		{"dry run", false, noConfirm, ok, nil, KindToolDryRun},
		{"dry run omitted", false, map[string]any{}, ok, nil, KindToolDryRun},
		{"mutation", false, confirm, ok, nil, KindToolMutation},
		{"result error", false, confirm, bad, nil, KindToolError},
		{"returned error", true, nil, nil, context.Canceled, KindToolError},
	}
	for _, c := range cases {
		if got := classifyTool(c.readOnly, c.args, c.res, c.err); got != c.want {
			t.Errorf("%s: got %s want %s", c.name, got, c.want)
		}
	}
}

func TestObserverMiddlewareClassifiesAndSanitizes(t *testing.T) {
	buf := NewBuffer()
	obs := NewObserver(buf, func(s string) string { return strings.ReplaceAll(s, "SECRET", "[REDACTED]") })
	s := mcpserver.NewMCPServer("t", "1", mcpserver.WithToolHandlerMiddleware(obs.Middleware()), mcpserver.WithHooks(obs.Hooks()))
	obs.SetServer(s)
	s.AddTool(mcp.NewTool("ro", mcp.WithReadOnlyHintAnnotation(true)), func(ctx context.Context, r mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return mcp.NewToolResultText("data SECRET"), nil
	})
	s.AddTool(mcp.NewTool("mut", mcp.WithReadOnlyHintAnnotation(false), mcp.WithBoolean("confirm")), func(ctx context.Context, r mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return mcp.NewToolResultText("done"), nil
	})

	call := func(name, args string) {
		msg := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"` + name + `","arguments":` + args + `}}`
		s.HandleMessage(context.Background(), []byte(msg))
	}
	call("ro", `{}`)
	call("mut", `{"confirm":false}`)
	call("mut", `{"confirm":true}`)

	got := buf.Snapshot()
	if len(got) != 3 {
		t.Fatalf("expected 3 entries, got %d: %+v", len(got), got)
	}
	want := []string{KindToolRead, KindToolDryRun, KindToolMutation}
	for i, w := range want {
		if got[i].Kind != w {
			t.Errorf("entry %d: got %s want %s", i, got[i].Kind, w)
		}
	}
	if strings.Contains(got[0].Detail, "SECRET") || !strings.Contains(got[0].Detail, "[REDACTED]") {
		t.Fatalf("detail not sanitized: %q", got[0].Detail)
	}
}

func TestLogTee(t *testing.T) {
	buf := NewBuffer()
	obs := NewObserver(buf, func(s string) string { return strings.ReplaceAll(s, "SECRET", "[REDACTED]") })
	var out strings.Builder
	lv := new(slog.LevelVar)
	lv.Set(slog.LevelWarn)
	l := slog.New(obs.LogHandler(slog.NewJSONHandler(&out, &slog.HandlerOptions{Level: lv}))).With("a", "SECRET")

	l.Info("hidden")
	l.Warn("shown", "k", "v")
	got := buf.Snapshot()
	if len(got) != 1 || got[0].Message != "shown" || got[0].Kind != KindServerLog || got[0].Level != "WARN" {
		t.Fatalf("unexpected: %+v", got)
	}
	if !strings.Contains(got[0].Detail, "k=v") || strings.Contains(got[0].Detail, "SECRET") {
		t.Fatalf("detail: %q", got[0].Detail)
	}
	if !strings.Contains(out.String(), "shown") {
		t.Fatal("record not forwarded to wrapped handler")
	}
}

func newTestServer(t *testing.T, mutate func(*Options)) (*Server, *Buffer) {
	t.Helper()
	buf := NewBuffer()
	lv := new(slog.LevelVar)
	opts := Options{
		BindAddress: "127.0.0.1",
		Port:        "8081",
		Token:       "ui-token",
		Transport:   "http",
		Buffer:      buf,
		LogLevel:    lv,
		Ping:        func(ctx context.Context) error { return nil },
		ToolCount:   func() int { return 7 },
	}
	if mutate != nil {
		mutate(&opts)
	}
	s, err := New(opts)
	if err != nil {
		t.Fatal(err)
	}
	return s, buf
}

func do(s *Server, method, path, host string, hdr map[string]string, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Host = host
	for k, v := range hdr {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	return rec
}

const lh = "127.0.0.1:8081"

func TestNewRequiresToken(t *testing.T) {
	if _, err := New(Options{BindAddress: "127.0.0.1", Port: "1", Buffer: NewBuffer()}); err == nil {
		t.Fatal("expected error without token")
	}
}

func TestNewRefusesNonLoopbackBind(t *testing.T) {
	_, err := New(Options{BindAddress: "0.0.0.0", Port: "1", Token: "t", Buffer: NewBuffer()})
	if err == nil {
		t.Fatal("expected refusal for non-loopback bind")
	}
	if _, err := New(Options{BindAddress: "0.0.0.0", Port: "1", Token: "t", Buffer: NewBuffer(), AllowInsecureBind: true}); err != nil {
		t.Fatalf("explicit opt-in should be allowed: %v", err)
	}
}

func TestStaticServedWithoutAuth(t *testing.T) {
	s, _ := newTestServer(t, nil)
	for _, p := range []string{"/", "/app.js", "/style.css"} {
		if rec := do(s, http.MethodGet, p, lh, nil, ""); rec.Code != http.StatusOK {
			t.Errorf("%s: got %d", p, rec.Code)
		}
	}
}

func TestAPIRequiresAuth(t *testing.T) {
	s, _ := newTestServer(t, nil)
	for _, p := range []string{"/api/status", "/api/logs/stream"} {
		if rec := do(s, http.MethodGet, p, lh, nil, ""); rec.Code != http.StatusUnauthorized {
			t.Errorf("%s: expected 401, got %d", p, rec.Code)
		}
	}
	csrf := map[string]string{csrfHeader: csrfValue}
	for _, c := range []struct{ m, p string }{{http.MethodPost, "/api/logs/clear"}, {http.MethodPatch, "/api/loglevel"}} {
		if rec := do(s, c.m, c.p, lh, csrf, "{}"); rec.Code != http.StatusUnauthorized {
			t.Errorf("%s %s: expected 401, got %d", c.m, c.p, rec.Code)
		}
	}
	bad := map[string]string{"Authorization": "Bearer nope"}
	if rec := do(s, http.MethodGet, "/api/status", lh, bad, ""); rec.Code != http.StatusUnauthorized {
		t.Errorf("wrong bearer: got %d", rec.Code)
	}
}

func TestBearerStatus(t *testing.T) {
	s, _ := newTestServer(t, nil)
	rec := do(s, http.MethodGet, "/api/status", lh, map[string]string{"Authorization": "Bearer ui-token"}, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("got %d: %s", rec.Code, rec.Body)
	}
	for _, want := range []string{`"transport":"http"`, `"tools":7`, `"ok":true`} {
		if !strings.Contains(rec.Body.String(), want) {
			t.Errorf("status missing %s: %s", want, rec.Body)
		}
	}
}

func TestLoginCookieFlow(t *testing.T) {
	s, _ := newTestServer(t, nil)
	csrf := map[string]string{csrfHeader: csrfValue}

	rec := do(s, http.MethodPost, "/api/auth/login", lh, csrf, `{"token":"ui-token"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("login: %d %s", rec.Code, rec.Body)
	}
	cookies := rec.Result().Cookies()
	if len(cookies) != 1 || !cookies[0].HttpOnly || cookies[0].SameSite != http.SameSiteStrictMode {
		t.Fatalf("expected one HttpOnly SameSite=Strict cookie, got %+v", cookies)
	}
	if cookies[0].Value == "ui-token" {
		t.Fatal("cookie must not contain the token")
	}

	req := httptest.NewRequest(http.MethodGet, "/api/status", nil)
	req.Host = lh
	req.AddCookie(cookies[0])
	r2 := httptest.NewRecorder()
	s.Handler().ServeHTTP(r2, req)
	if r2.Code != http.StatusOK {
		t.Fatalf("cookie auth failed: %d", r2.Code)
	}

	lo := httptest.NewRequest(http.MethodPost, "/api/auth/logout", nil)
	lo.Host = lh
	lo.Header.Set(csrfHeader, csrfValue)
	lo.AddCookie(cookies[0])
	s.Handler().ServeHTTP(httptest.NewRecorder(), lo)
	req2 := httptest.NewRequest(http.MethodGet, "/api/status", nil)
	req2.Host = lh
	req2.AddCookie(cookies[0])
	r3 := httptest.NewRecorder()
	s.Handler().ServeHTTP(r3, req2)
	if r3.Code != http.StatusUnauthorized {
		t.Fatalf("session should be invalid after logout, got %d", r3.Code)
	}
}

func TestLoginRejectsBadToken(t *testing.T) {
	s, _ := newTestServer(t, nil)
	start := time.Now()
	rec := do(s, http.MethodPost, "/api/auth/login", lh, map[string]string{csrfHeader: csrfValue}, `{"token":"wrong"}`)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("got %d", rec.Code)
	}
	if time.Since(start) < failedLoginDelay {
		t.Fatal("failed login should be delayed")
	}
	if len(rec.Result().Cookies()) != 0 {
		t.Fatal("no cookie expected")
	}
}

func TestHostOriginAndCSRFGuards(t *testing.T) {
	s, _ := newTestServer(t, nil)
	bearer := map[string]string{"Authorization": "Bearer ui-token"}

	if rec := do(s, http.MethodGet, "/api/status", "evil.example:8081", bearer, ""); rec.Code != http.StatusForbidden {
		t.Errorf("rebinding host: expected 403, got %d", rec.Code)
	}
	for _, h := range []string{"localhost:8081", "[::1]:8081", "127.0.0.1:8081"} {
		if rec := do(s, http.MethodGet, "/api/status", h, bearer, ""); rec.Code != http.StatusOK {
			t.Errorf("host %s: got %d", h, rec.Code)
		}
	}

	cross := map[string]string{"Authorization": "Bearer ui-token", "Origin": "http://evil.example"}
	if rec := do(s, http.MethodGet, "/api/status", lh, cross, ""); rec.Code != http.StatusForbidden {
		t.Errorf("cross origin: expected 403, got %d", rec.Code)
	}
	same := map[string]string{"Authorization": "Bearer ui-token", "Origin": "http://" + lh}
	if rec := do(s, http.MethodGet, "/api/status", lh, same, ""); rec.Code != http.StatusOK {
		t.Errorf("same origin: got %d", rec.Code)
	}

	if rec := do(s, http.MethodPost, "/api/logs/clear", lh, bearer, ""); rec.Code != http.StatusForbidden {
		t.Errorf("missing csrf header: expected 403, got %d", rec.Code)
	}
	withCSRF := map[string]string{"Authorization": "Bearer ui-token", csrfHeader: csrfValue}
	if rec := do(s, http.MethodPost, "/api/logs/clear", lh, withCSRF, ""); rec.Code != http.StatusOK {
		t.Errorf("with csrf header: got %d", rec.Code)
	}
}

func TestAllowedHosts(t *testing.T) {
	s, _ := newTestServer(t, func(o *Options) {
		o.BindAddress = "0.0.0.0"
		o.AllowInsecureBind = true
		o.AllowedHosts = []string{"pve-mcp.lan"}
	})
	bearer := map[string]string{"Authorization": "Bearer ui-token"}
	if rec := do(s, http.MethodGet, "/api/status", "pve-mcp.lan:8081", bearer, ""); rec.Code != http.StatusOK {
		t.Errorf("allowed host: got %d", rec.Code)
	}
	if rec := do(s, http.MethodGet, "/api/status", "other.lan:8081", bearer, ""); rec.Code != http.StatusForbidden {
		t.Errorf("other host: expected 403, got %d", rec.Code)
	}
}

func TestClearAndLogLevel(t *testing.T) {
	s, buf := newTestServer(t, nil)
	h := map[string]string{"Authorization": "Bearer ui-token", csrfHeader: csrfValue}
	buf.Add(Entry{Message: "x"})
	do(s, http.MethodPost, "/api/logs/clear", lh, h, "")
	if len(buf.Snapshot()) != 0 {
		t.Fatal("buffer not cleared")
	}

	if rec := do(s, http.MethodPatch, "/api/loglevel", lh, h, `{"level":"debug"}`); rec.Code != http.StatusOK {
		t.Fatalf("got %d: %s", rec.Code, rec.Body)
	}
	if s.opts.LogLevel.Level() != slog.LevelDebug {
		t.Fatal("level not applied")
	}
	if rec := do(s, http.MethodPatch, "/api/loglevel", lh, h, `{"level":"loud"}`); rec.Code != http.StatusBadRequest {
		t.Fatalf("bad level: got %d", rec.Code)
	}
}

func TestStreamReplaysHistoryThenLive(t *testing.T) {
	s, buf := newTestServer(t, nil)
	buf.Add(Entry{Kind: KindServerLog, Message: "old"})
	ts := httptest.NewServer(s.Handler())
	defer ts.Close()

	host := strings.TrimPrefix(ts.URL, "http://")
	s.opts.AllowedHosts = []string{host}
	req, _ := http.NewRequest(http.MethodGet, ts.URL+"/api/logs/stream", nil)
	req.Header.Set("Authorization", "Bearer ui-token")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if ct := resp.Header.Get("Content-Type"); ct != "text/event-stream" {
		t.Fatalf("content type %q", ct)
	}

	lines := make(chan string, 16)
	go func() {
		sc := bufio.NewScanner(resp.Body)
		for sc.Scan() {
			if strings.HasPrefix(sc.Text(), "data: ") {
				lines <- sc.Text()
			}
		}
		close(lines)
	}()
	wait := func() string {
		select {
		case l := <-lines:
			return l
		case <-time.After(3 * time.Second):
			t.Fatal("timed out waiting for event")
			return ""
		}
	}
	if l := wait(); !strings.Contains(l, `"old"`) {
		t.Fatalf("expected history first, got %s", l)
	}
	buf.Add(Entry{Kind: KindServerLog, Message: "fresh"})
	if l := wait(); !strings.Contains(l, `"fresh"`) {
		t.Fatalf("expected live event, got %s", l)
	}
}

func TestShutdownClosesStreamsAndFreesBuffer(t *testing.T) {
	s, buf := newTestServer(t, nil)
	_, live, _ := buf.Subscribe()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := s.Shutdown(ctx); err != nil {
		t.Fatal(err)
	}
	if _, open := <-live; open {
		t.Fatal("stream should be closed on shutdown")
	}
	if buf.ring != nil {
		t.Fatal("buffer should be freed on shutdown")
	}
}
