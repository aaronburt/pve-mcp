package pve_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"

	"github.com/aaronburt/pve-mcp/internal/config"
	"github.com/aaronburt/pve-mcp/internal/pve"
)

func TestClient_Get_Success(t *testing.T) {
	expectedToken := "PVEAPIToken=root@pam!token=secret-uuid"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != expectedToken {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		if r.URL.Path != "/api2/json/cluster/status" {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		if r.URL.Query().Get("format") != "summary" {
			http.Error(w, "missing query", http.StatusBadRequest)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"data":[{"id":"node/pve1","type":"node","online":1}]}`))
	}))
	defer server.Close()

	cfg := &config.Config{
		Host:        server.URL,
		TokenID:     "root@pam!token",
		TokenSecret: "secret-uuid",
		VerifySSL:   false,
	}

	client, err := pve.NewClient(cfg)
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	query := url.Values{}
	query.Set("format", "summary")

	var result []map[string]any
	err = client.Get(context.Background(), "/cluster/status", query, &result)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result) != 1 || result[0]["id"] != "node/pve1" {
		t.Fatalf("unexpected result: %v", result)
	}
}

func TestClient_Get_OversizedPayload(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		chunk := strings.Repeat("A", 1024*1024)
		for i := 0; i < 11; i++ {
			w.Write([]byte(chunk))
		}
	}))
	defer server.Close()

	cfg := &config.Config{
		Host:        server.URL,
		TokenID:     "root@pam!token",
		TokenSecret: "secret-uuid",
		VerifySSL:   false,
	}

	client, err := pve.NewClient(cfg)
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	var result json.RawMessage
	err = client.Get(context.Background(), "/oversized", nil, &result)
	if err == nil || !strings.Contains(err.Error(), "response exceeds maximum allowed size") {
		t.Fatalf("expected oversized error, got: %v", err)
	}
}

func TestClient_Sanitize(t *testing.T) {
	cfg := &config.Config{
		Host:        "https://pve.local:8006",
		TokenID:     "root@pam!test",
		TokenSecret: "super-secret-key-1234",
	}

	client, err := pve.NewClient(cfg)
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	raw := "Error connecting with Authorization: PVEAPIToken=root@pam!test=super-secret-key-1234 to host. Secret is super-secret-key-1234! Unicode test: 🚀 こんにちは 🔒"
	sanitized := client.Sanitize(raw)

	if strings.Contains(sanitized, "super-secret-key-1234") {
		t.Errorf("sanitized text contains secret: %s", sanitized)
	}
	if !strings.Contains(sanitized, "🚀 こんにちは 🔒") {
		t.Errorf("sanitized text corrupted UTF-8 runes: %s", sanitized)
	}
	if !strings.Contains(sanitized, "PVEAPIToken=[REDACTED]") {
		t.Errorf("sanitized text does not contain PVEAPIToken=[REDACTED]: %s", sanitized)
	}
}

func TestClient_TLS_Fingerprint(t *testing.T) {
	tlsServer := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"data":"secure-ok"}`))
	}))
	defer tlsServer.Close()

	cert := tlsServer.Certificate()
	hash := sha256.Sum256(cert.Raw)
	fingerprintHex := hex.EncodeToString(hash[:])

	matchingCfg := &config.Config{
		Host:        tlsServer.URL,
		TokenID:     "user@pve!token",
		TokenSecret: "secret",
		VerifySSL:   true,
		Fingerprint: fingerprintHex,
	}

	client, err := pve.NewClient(matchingCfg)
	if err != nil {
		t.Fatalf("failed to create client with matching fingerprint: %v", err)
	}

	var res string
	err = client.Get(context.Background(), "/test", nil, &res)
	if err != nil {
		t.Fatalf("expected success with matching fingerprint, got: %v", err)
	}

	mismatchCfg := &config.Config{
		Host:        tlsServer.URL,
		TokenID:     "user@pve!token",
		TokenSecret: "secret",
		VerifySSL:   true,
		Fingerprint: "0000000000000000000000000000000000000000000000000000000000000000",
	}

	mismatchClient, err := pve.NewClient(mismatchCfg)
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	err = mismatchClient.Get(context.Background(), "/test", nil, &res)
	if err == nil {
		t.Fatal("expected error with mismatched fingerprint, got nil")
	}
}

func TestClient_Get_HTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		w.Write([]byte(`{"message":"permission denied for secret-uuid"}`))
	}))
	defer server.Close()

	cfg := &config.Config{
		Host:        server.URL,
		TokenID:     "user@pve!token",
		TokenSecret: "secret-uuid",
		VerifySSL:   false,
	}

	client, err := pve.NewClient(cfg)
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	var res json.RawMessage
	err = client.Get(context.Background(), "/forbidden", nil, &res)
	if err == nil {
		t.Fatal("expected http error, got nil")
	}
	if strings.Contains(err.Error(), "secret-uuid") {
		t.Fatalf("expected error to sanitize secret, got: %s", err.Error())
	}
}

func TestClient_TLS_CACertError(t *testing.T) {
	cfg := &config.Config{
		Host:        "https://127.0.0.1:8006",
		TokenID:     "user@pve!token",
		TokenSecret: "secret",
		CACertPath:  "/nonexistent/path/to/ca.pem",
	}

	_, err := pve.NewClient(cfg)
	if err == nil {
		t.Fatal("expected error for non-existent CA cert")
	}
}

func TestClient_Post_Success(t *testing.T) {
	expectedToken := "PVEAPIToken=root@pam!token=secret-uuid"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != expectedToken {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if r.URL.Path != "/api2/json/nodes/pve1/qemu/100/status/start" {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		if r.Header.Get("Content-Type") != "application/x-www-form-urlencoded" {
			http.Error(w, "invalid content-type", http.StatusBadRequest)
			return
		}
		if err := r.ParseForm(); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if r.FormValue("skiplock") != "1" {
			http.Error(w, "missing form value", http.StatusBadRequest)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"data":"UPID:pve1:00001234:00005678:66F2ABCD:qmstart:100:root@pam:"}`))
	}))
	defer server.Close()

	cfg := &config.Config{
		Host:        server.URL,
		TokenID:     "root@pam!token",
		TokenSecret: "secret-uuid",
		VerifySSL:   false,
	}

	client, err := pve.NewClient(cfg)
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	form := url.Values{}
	form.Set("skiplock", "1")

	var taskUPID string
	err = client.Post(context.Background(), "/nodes/pve1/qemu/100/status/start", form, &taskUPID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.HasPrefix(taskUPID, "UPID:pve1") {
		t.Fatalf("unexpected task upid: %s", taskUPID)
	}
}

func TestClient_Put_Success(t *testing.T) {
	expectedToken := "PVEAPIToken=root@pam!token=secret-uuid"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != expectedToken {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		if r.Method != http.MethodPut {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if r.URL.Path != "/api2/json/nodes/pve1/lxc/200/config" {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		if err := r.ParseForm(); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if r.FormValue("memory") != "4096" {
			http.Error(w, "missing memory value", http.StatusBadRequest)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"data":null}`))
	}))
	defer server.Close()

	cfg := &config.Config{
		Host:        server.URL,
		TokenID:     "root@pam!token",
		TokenSecret: "secret-uuid",
		VerifySSL:   false,
	}

	client, err := pve.NewClient(cfg)
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	form := url.Values{}
	form.Set("memory", "4096")

	var raw json.RawMessage
	err = client.Put(context.Background(), "/nodes/pve1/lxc/200/config", form, &raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestClient_Delete_Success(t *testing.T) {
	expectedToken := "PVEAPIToken=root@pam!token=secret-uuid"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != expectedToken {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		if r.Method != http.MethodDelete {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if r.URL.Path != "/api2/json/nodes/pve1/lxc/100" {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		if r.URL.Query().Get("purge") != "1" {
			http.Error(w, "missing purge query param", http.StatusBadRequest)
			return
		}
		if r.Header.Get("Content-Type") != "" {
			http.Error(w, "DELETE must not have Content-Type", http.StatusBadRequest)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"data":"UPID:pve1:0001:0002:0003:vzdestroy:100:root@pam:"}`))
	}))
	defer server.Close()

	cfg := &config.Config{
		Host:        server.URL,
		TokenID:     "root@pam!token",
		TokenSecret: "secret-uuid",
		VerifySSL:   false,
	}

	client, err := pve.NewClient(cfg)
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	params := url.Values{}
	params.Set("purge", "1")

	var upid string
	err = client.Delete(context.Background(), "/nodes/pve1/lxc/100", params, &upid)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if upid != "UPID:pve1:0001:0002:0003:vzdestroy:100:root@pam:" {
		t.Errorf("got upid %s, want expected UPID", upid)
	}
}

func TestClient_GetTaskStatus_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if r.URL.Path != "/api2/json/nodes/pve1/tasks/UPID:pve1:123/status" {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"data":{"status":"stopped","exitstatus":"OK","id":"100","type":"vzdestroy","user":"root@pam"}}`))
	}))
	defer server.Close()

	cfg := &config.Config{
		Host:        server.URL,
		TokenID:     "root@pam!token",
		TokenSecret: "secret-uuid",
		VerifySSL:   false,
	}

	client, err := pve.NewClient(cfg)
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	status, err := client.GetTaskStatus(context.Background(), "pve1", "UPID:pve1:123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if status.Status != "stopped" || status.ExitStatus != "OK" {
		t.Errorf("unexpected status: %+v", status)
	}
}

func TestClient_BuildTLSConfig_Errors(t *testing.T) {
	cfgMissingCA := &config.Config{
		Host:        "https://127.0.0.1:8006",
		TokenID:     "root@pam!token",
		TokenSecret: "secret",
		CACertPath:  "non_existent_ca_file.pem",
	}
	_, err := pve.NewClient(cfgMissingCA)
	if err == nil {
		t.Fatal("expected error on missing CA cert file")
	}

	tmpFile := t.TempDir() + "/bad_ca.pem"
	if err := os.WriteFile(tmpFile, []byte("NOT A VALID CERTIFICATE"), 0600); err != nil {
		t.Fatalf("failed to write bad ca file: %v", err)
	}

	cfgBadCA := &config.Config{
		Host:        "https://127.0.0.1:8006",
		TokenID:     "root@pam!token",
		TokenSecret: "secret",
		CACertPath:  tmpFile,
	}
	_, err = pve.NewClient(cfgBadCA)
	if err == nil {
		t.Fatal("expected error on invalid CA cert PEM")
	}
}

func TestClient_ErrorsAndEdgeCases(t *testing.T) {
	serverErr := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/nonjson") {
			w.Header().Set("Content-Type", "text/plain")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("not valid json"))
			return
		}
		if strings.Contains(r.URL.Path, "/apierror") {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte(`{"errors":{"node":"not found"}}`))
			return
		}
		http.Error(w, "internal server error", http.StatusInternalServerError)
	}))
	defer serverErr.Close()

	cfg := &config.Config{
		Host:        serverErr.URL,
		TokenID:     "root@pam!token",
		TokenSecret: "secret",
		VerifySSL:   false,
	}
	client, err := pve.NewClient(cfg)
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	var res map[string]any
	cancelCtx, cancel := context.WithCancel(context.Background())
	cancel()
	err = client.Get(cancelCtx, "/test", nil, &res)
	if err == nil {
		t.Fatal("expected error on canceled context in Get")
	}

	err = client.Get(context.Background(), "/apierror", nil, &res)
	if err == nil {
		t.Fatal("expected error on 400 Bad Request in Get")
	}

	err = client.Get(context.Background(), "/nonjson", nil, &res)
	if err == nil {
		t.Fatal("expected error on non-json in Get")
	}

	_, err = client.GetTaskStatus(context.Background(), "pve1", "UPID:pve1:bad")
	if err == nil {
		t.Fatal("expected error in GetTaskStatus on 500 error")
	}

	var postRes string
	err = client.Post(cancelCtx, "/test", nil, &postRes)
	if err == nil {
		t.Fatal("expected error on canceled context in Post")
	}

	err = client.Post(context.Background(), "/apierror", nil, &postRes)
	if err == nil {
		t.Fatal("expected error on 400 Bad Request in Post")
	}

	err = client.Post(context.Background(), "/nonjson", nil, &postRes)
	if err == nil {
		t.Fatal("expected error on non-json in Post")
	}

	err = client.Get(context.Background(), ":invalid-url", nil, &res)
	if err == nil {
		t.Fatal("expected error on malformed URL in Get")
	}

	err = client.Post(context.Background(), ":invalid-url", nil, &postRes)
	if err == nil {
		t.Fatal("expected error on malformed URL in Post")
	}
}
