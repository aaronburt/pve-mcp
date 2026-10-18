package webui

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"runtime/metrics"
	"strings"
	"sync"
	"time"
)

//go:embed static
var staticFS embed.FS

const (
	cookieName       = "pve_mcp_ui"
	sessionTTL       = 12 * time.Hour
	csrfHeader       = "X-Requested-With"
	csrfValue        = "pve-mcp-ui"
	failedLoginDelay = 750 * time.Millisecond
)

type Options struct {
	BindAddress       string
	Port              string
	Token             string
	AllowInsecureBind bool
	AllowedHosts      []string
	Transport         string
	Buffer            *Buffer
	LogLevel          interface {
		Level() slog.Level
		Set(slog.Level)
	}
	Ping      func(ctx context.Context) error
	ToolCount func() int
}

type Server struct {
	opts    Options
	http    *http.Server
	started time.Time

	mu       sync.Mutex
	sessions map[string]time.Time
}

func isLoopback(host string) bool {
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(strings.Trim(host, "[]"))
	return ip != nil && ip.IsLoopback()
}

func New(opts Options) (*Server, error) {
	if opts.Token == "" {
		return nil, errors.New("MCP_UI_TOKEN is required to start the web UI")
	}
	if opts.Buffer == nil {
		return nil, errors.New("web ui requires a log buffer")
	}
	if !isLoopback(opts.BindAddress) && !opts.AllowInsecureBind {
		return nil, fmt.Errorf("refusing to bind the web ui to non-loopback address %q: the token and session cookie would travel unencrypted; set MCP_UI_ALLOW_INSECURE_BIND=true to allow it", opts.BindAddress)
	}

	s := &Server{
		opts:     opts,
		started:  time.Now(),
		sessions: make(map[string]time.Time),
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", s.static("index.html", "text/html; charset=utf-8"))
	mux.HandleFunc("GET /app.js", s.static("app.js", "text/javascript; charset=utf-8"))
	mux.HandleFunc("GET /style.css", s.static("style.css", "text/css; charset=utf-8"))
	mux.HandleFunc("POST /api/auth/login", s.handleLogin)
	mux.HandleFunc("POST /api/auth/logout", s.handleLogout)
	mux.HandleFunc("GET /api/status", s.authed(s.handleStatus))
	mux.HandleFunc("GET /api/logs/stream", s.authed(s.handleStream))
	mux.HandleFunc("POST /api/logs/clear", s.authed(s.handleClear))
	mux.HandleFunc("PATCH /api/loglevel", s.authed(s.handleLogLevel))

	s.http = &http.Server{
		Addr:              net.JoinHostPort(opts.BindAddress, opts.Port),
		Handler:           s.guard(mux),
		ReadHeaderTimeout: 5 * time.Second,
		IdleTimeout:       120 * time.Second,
	}
	return s, nil
}

func (s *Server) Addr() string          { return s.http.Addr }
func (s *Server) Handler() http.Handler { return s.http.Handler }

func (s *Server) ListenAndServe() error { return s.http.ListenAndServe() }

func (s *Server) Shutdown(ctx context.Context) error {
	s.opts.Buffer.Close()
	return s.http.Shutdown(ctx)
}

func (s *Server) hostAllowed(host string) bool {
	host = strings.ToLower(host)
	if host == "" {
		return false
	}
	name := host
	if h, _, err := net.SplitHostPort(host); err == nil {
		name = h
	}
	name = strings.Trim(name, "[]")
	for _, a := range s.opts.AllowedHosts {
		if a == host || a == name {
			return true
		}
	}
	if len(s.opts.AllowedHosts) > 0 {
		return isLoopback(name)
	}
	if isLoopback(s.opts.BindAddress) {
		return isLoopback(name)
	}
	return true
}

func (s *Server) guard(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("Cache-Control", "no-store")
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Referrer-Policy", "no-referrer")
		h.Set("Content-Security-Policy", "default-src 'self'; frame-ancestors 'none'")

		if !s.hostAllowed(r.Host) {
			writeError(w, http.StatusForbidden, "forbidden host")
			return
		}
		if origin := r.Header.Get("Origin"); origin != "" {
			u, err := url.Parse(origin)
			if err != nil || !strings.EqualFold(u.Host, r.Host) {
				writeError(w, http.StatusForbidden, "forbidden origin")
				return
			}
		}
		switch r.Method {
		case http.MethodGet, http.MethodHead, http.MethodOptions:
		default:
			if r.Header.Get(csrfHeader) != csrfValue {
				writeError(w, http.StatusForbidden, "missing "+csrfHeader+" header")
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

func tokenEqual(a, b string) bool {
	ha, hb := sha256.Sum256([]byte(a)), sha256.Sum256([]byte(b))
	return subtle.ConstantTimeCompare(ha[:], hb[:]) == 1
}

func bearer(r *http.Request) string {
	parts := strings.SplitN(r.Header.Get("Authorization"), " ", 2)
	if len(parts) == 2 && strings.EqualFold(parts[0], "Bearer") {
		return strings.TrimSpace(parts[1])
	}
	return ""
}

func (s *Server) validSession(id string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	exp, ok := s.sessions[id]
	if !ok {
		return false
	}
	if time.Now().After(exp) {
		delete(s.sessions, id)
		return false
	}
	return true
}

func (s *Server) authed(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if t := bearer(r); t != "" && tokenEqual(t, s.opts.Token) {
			next(w, r)
			return
		}
		if c, err := r.Cookie(cookieName); err == nil && s.validSession(c.Value) {
			next(w, r)
			return
		}
		writeError(w, http.StatusUnauthorized, "unauthorized")
	}
}

func (s *Server) static(name, contentType string) http.HandlerFunc {
	data, err := fs.ReadFile(staticFS, "static/"+name)
	if err != nil {
		panic(err)
	}
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", contentType)
		w.Write(data)
	}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Token string `json:"token"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&body); err != nil || !tokenEqual(body.Token, s.opts.Token) {
		time.Sleep(failedLoginDelay)
		slog.Warn("web ui login failed", "remote", r.RemoteAddr)
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create session")
		return
	}
	id := hex.EncodeToString(raw)

	now := time.Now()
	s.mu.Lock()
	for k, exp := range s.sessions {
		if now.After(exp) {
			delete(s.sessions, k)
		}
	}
	s.sessions[id] = now.Add(sessionTTL)
	s.mu.Unlock()

	http.SetCookie(w, &http.Cookie{
		Name:     cookieName,
		Value:    id,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
		MaxAge:   int(sessionTTL.Seconds()),
	})
	slog.Info("web ui login", "remote", r.RemoteAddr)
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie(cookieName); err == nil {
		s.mu.Lock()
		delete(s.sessions, c.Value)
		s.mu.Unlock()
	}
	http.SetCookie(w, &http.Cookie{Name: cookieName, Value: "", Path: "/", MaxAge: -1, HttpOnly: true, SameSite: http.SameSiteStrictMode})
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func heapBytes() uint64 {
	sample := []metrics.Sample{{Name: "/memory/classes/heap/objects:bytes"}}
	metrics.Read(sample)
	if sample[0].Value.Kind() == metrics.KindUint64 {
		return sample[0].Value.Uint64()
	}
	return 0
}

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	pve := map[string]any{"ok": false}
	if s.opts.Ping != nil {
		ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
		start := time.Now()
		err := s.opts.Ping(ctx)
		cancel()
		pve["latency_ms"] = time.Since(start).Milliseconds()
		if err != nil {
			pve["error"] = err.Error()
		} else {
			pve["ok"] = true
		}
	}
	tools := 0
	if s.opts.ToolCount != nil {
		tools = s.opts.ToolCount()
	}
	level := ""
	if s.opts.LogLevel != nil {
		level = strings.ToLower(s.opts.LogLevel.Level().String())
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"transport":      s.opts.Transport,
		"uptime_seconds": int64(time.Since(s.started).Seconds()),
		"pve":            pve,
		"tools":          tools,
		"heap_bytes":     heapBytes(),
		"log_level":      level,
		"buffer":         map[string]int{"capacity": BufferCapacity, "count": len(s.opts.Buffer.Snapshot())},
	})
}

func (s *Server) handleStream(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, "streaming unsupported")
		return
	}
	history, live, cancel := s.opts.Buffer.Subscribe()
	defer cancel()

	h := w.Header()
	h.Set("Content-Type", "text/event-stream")
	h.Set("Connection", "keep-alive")
	w.WriteHeader(http.StatusOK)

	send := func(e Entry) bool {
		b, err := json.Marshal(e)
		if err != nil {
			return true
		}
		_, err = fmt.Fprintf(w, "id: %d\ndata: %s\n\n", e.ID, b)
		return err == nil
	}
	for _, e := range history {
		if !send(e) {
			return
		}
	}
	flusher.Flush()

	heartbeat := time.NewTicker(20 * time.Second)
	defer heartbeat.Stop()
	for {
		select {
		case <-r.Context().Done():
			return
		case e, open := <-live:
			if !open {
				return
			}
			if !send(e) {
				return
			}
			flusher.Flush()
		case <-heartbeat.C:
			if _, err := fmt.Fprint(w, ": keepalive\n\n"); err != nil {
				return
			}
			flusher.Flush()
		}
	}
}

func (s *Server) handleClear(w http.ResponseWriter, r *http.Request) {
	s.opts.Buffer.Clear()
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) handleLogLevel(w http.ResponseWriter, r *http.Request) {
	if s.opts.LogLevel == nil {
		writeError(w, http.StatusNotImplemented, "log level is not adjustable")
		return
	}
	var body struct {
		Level string `json:"level"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1024)).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	var lvl slog.Level
	switch strings.ToLower(body.Level) {
	case "debug":
		lvl = slog.LevelDebug
	case "info":
		lvl = slog.LevelInfo
	case "warn", "warning":
		lvl = slog.LevelWarn
	case "error":
		lvl = slog.LevelError
	default:
		writeError(w, http.StatusBadRequest, "level must be one of debug, info, warn, error")
		return
	}
	slog.Warn("web ui log level change", "from", strings.ToLower(s.opts.LogLevel.Level().String()), "to", strings.ToLower(lvl.String()), "remote", r.RemoteAddr)
	s.opts.LogLevel.Set(lvl)
	writeJSON(w, http.StatusOK, map[string]string{"log_level": strings.ToLower(lvl.String())})
}
