package pve_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
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
