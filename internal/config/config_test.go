package config_test

import (
	"os"
	"testing"
	"time"

	"github.com/aaronburt/pve-mcp-go/internal/config"
)

func TestLoadFromEnv_MissingRequired(t *testing.T) {
	t.Setenv("PVE_HOST", "")
	t.Setenv("PVE_TOKEN_ID", "")
	t.Setenv("PVE_TOKEN_SECRET", "")

	_, err := config.LoadFromEnv()
	if err == nil {
		t.Fatal("expected error when required environment variables are missing")
	}
}

func TestLoadFromEnv_InvalidURL(t *testing.T) {
	t.Setenv("PVE_HOST", "not-a-valid-url")
	t.Setenv("PVE_TOKEN_ID", "root@pam!token")
	t.Setenv("PVE_TOKEN_SECRET", "00000000-0000-0000-0000-000000000000")

	_, err := config.LoadFromEnv()
	if err == nil {
		t.Fatal("expected error for invalid PVE_HOST URL scheme")
	}
}

func TestLoadFromEnv_DefaultsAndHardening(t *testing.T) {
	t.Setenv("PVE_HOST", "https://pve.example.com:8006")
	t.Setenv("PVE_TOKEN_ID", "root@pam!token")
	t.Setenv("PVE_TOKEN_SECRET", "secret-uuid")
	t.Setenv("MCP_AUTH_TOKEN", "super-secret-mcp-bearer")

	cfg, err := config.LoadFromEnv()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.BindAddress != "127.0.0.1" {
		t.Errorf("got bind %s, want default 127.0.0.1", cfg.BindAddress)
	}
	if cfg.Port != "8080" {
		t.Errorf("got port %s, want default 8080", cfg.Port)
	}
	if cfg.MCPAuthToken != "super-secret-mcp-bearer" {
		t.Errorf("got auth token %s, want super-secret-mcp-bearer", cfg.MCPAuthToken)
	}
	if cfg.Timeout != 30*time.Second {
		t.Errorf("got timeout %v, want 30s", cfg.Timeout)
	}
	if !cfg.VerifySSL {
		t.Errorf("got verifySSL false, want default true")
	}
	if cfg.EnablePower {
		t.Errorf("got EnablePower true, want default false")
	}
	if cfg.EnableConfig {
		t.Errorf("got EnableConfig true, want default false")
	}
	if cfg.EnableStorage {
		t.Errorf("got EnableStorage true, want default false")
	}
	if cfg.EnableDestroy {
		t.Errorf("got EnableDestroy true, want default false")
	}
	if cfg.EnableNodeOps {
		t.Errorf("got EnableNodeOps true, want default false")
	}
	if cfg.EnableNodeShutdown {
		t.Errorf("got EnableNodeShutdown true, want default false")
	}
	if len(cfg.AllowedNodes) != 0 {
		t.Errorf("got %d allowed nodes, want 0", len(cfg.AllowedNodes))
	}
}

func TestLoadFromEnv_CustomOverrides(t *testing.T) {
	t.Setenv("PVE_HOST", "https://192.168.1.100:8006/")
	t.Setenv("PVE_TOKEN_ID", "auditor@pve!mcp")
	t.Setenv("PVE_TOKEN_SECRET", "custom-token-secret")
	t.Setenv("PVE_VERIFY_SSL", "false")
	t.Setenv("MCP_BIND_ADDRESS", "0.0.0.0")
	t.Setenv("PORT", ":9090")
	t.Setenv("PVE_TIMEOUT_SECONDS", "45")
	t.Setenv("LOG_LEVEL", "DEBUG")
	t.Setenv("PVE_CA_CERT", "/etc/ssl/pve-ca.pem")
	t.Setenv("PVE_FINGERPRINT", "AA:BB:CC:DD:EE:FF:11:22:33:44:55:66:77:88:99:00:AA:BB:CC:DD:EE:FF:11:22:33:44:55:66:77:88:99:00")
	t.Setenv("MCP_ALLOWED_ORIGINS", "http://localhost:3000,http://127.0.0.1:3000")
	t.Setenv("PVE_ENABLE_POWER", "true")
	t.Setenv("PVE_ENABLE_CONFIG", "1")
	t.Setenv("PVE_ENABLE_STORAGE", "yes")
	t.Setenv("PVE_ENABLE_DESTROY", "on")
	t.Setenv("PVE_ENABLE_NODE_OPS", "true")
	t.Setenv("PVE_ENABLE_NODE_SHUTDOWN", "true")
	t.Setenv("PVE_ALLOWED_NODES", "pve1, pve2")

	cfg, err := config.LoadFromEnv()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Host != "https://192.168.1.100:8006" {
		t.Errorf("got host %s, want trailing slash stripped", cfg.Host)
	}
	if cfg.VerifySSL {
		t.Errorf("got verifySSL true, want false")
	}
	if !cfg.EnablePower {
		t.Errorf("got EnablePower false, want true")
	}
	if !cfg.EnableConfig {
		t.Errorf("got EnableConfig false, want true")
	}
	if !cfg.EnableStorage {
		t.Errorf("got EnableStorage false, want true")
	}
	if !cfg.EnableDestroy {
		t.Errorf("got EnableDestroy false, want true")
	}
	if !cfg.EnableNodeOps {
		t.Errorf("got EnableNodeOps false, want true")
	}
	if !cfg.EnableNodeShutdown {
		t.Errorf("got EnableNodeShutdown false, want true")
	}
	if len(cfg.AllowedNodes) != 2 || cfg.AllowedNodes[0] != "pve1" || cfg.AllowedNodes[1] != "pve2" {
		t.Errorf("got allowed nodes %v, want [pve1 pve2]", cfg.AllowedNodes)
	}
	if cfg.BindAddress != "0.0.0.0" {
		t.Errorf("got bind %s, want 0.0.0.0", cfg.BindAddress)
	}
	if cfg.Port != "9090" {
		t.Errorf("got port %s, want 9090", cfg.Port)
	}
	if cfg.Timeout != 45*time.Second {
		t.Errorf("got timeout %v, want 45s", cfg.Timeout)
	}
	if cfg.LogLevel != "debug" {
		t.Errorf("got log level %s, want debug", cfg.LogLevel)
	}
	if cfg.CACertPath != "/etc/ssl/pve-ca.pem" {
		t.Errorf("got ca cert %s, want /etc/ssl/pve-ca.pem", cfg.CACertPath)
	}
	if cfg.Fingerprint != "aabbccddeeff11223344556677889900aabbccddeeff11223344556677889900" {
		t.Errorf("got fingerprint %s, want normalized hex", cfg.Fingerprint)
	}
	if len(cfg.AllowedOrigins) != 2 {
		t.Errorf("got %d allowed origins, want 2", len(cfg.AllowedOrigins))
	}
}

func TestLoadFromEnv_InvalidTimeout(t *testing.T) {
	t.Setenv("PVE_HOST", "https://pve.example.com")
	t.Setenv("PVE_TOKEN_ID", "root@pam!token")
	t.Setenv("PVE_TOKEN_SECRET", "secret")
	t.Setenv("PVE_TIMEOUT_SECONDS", "not-a-number")

	_, err := config.LoadFromEnv()
	if err == nil {
		t.Fatal("expected error for non-numeric timeout")
	}
}

func TestLoadDotEnv(t *testing.T) {
	config.LoadDotEnv("non_existent_file_path.env")

	tmp := t.TempDir() + "/test.env"
	content := "VAR1=hello\nVAR2=\"world\"\nVAR3='antigravity'\nVAR4=simple\nEMPTY=\nNO_EQUALS\n"
	if err := os.WriteFile(tmp, []byte(content), 0600); err != nil {
		t.Fatalf("failed to write temp env: %v", err)
	}

	config.LoadDotEnv(tmp)

	if os.Getenv("VAR1") != "hello" {
		t.Errorf("got %s, want hello", os.Getenv("VAR1"))
	}
	if os.Getenv("VAR2") != "world" {
		t.Errorf("got %s, want world", os.Getenv("VAR2"))
	}
	if os.Getenv("VAR3") != "antigravity" {
		t.Errorf("got %s, want antigravity", os.Getenv("VAR3"))
	}
}

func TestLoadFromEnv_WebUIDefaultsAndOverrides(t *testing.T) {
	t.Setenv("PVE_HOST", "https://pve.local:8006")
	t.Setenv("PVE_TOKEN_ID", "root@pam!token")
	t.Setenv("PVE_TOKEN_SECRET", "00000000-0000-0000-0000-000000000000")
	t.Setenv("MCP_UI_ENABLED", "")
	t.Setenv("MCP_UI_BIND_ADDRESS", "")
	t.Setenv("MCP_UI_PORT", "")
	t.Setenv("MCP_UI_TOKEN", "")
	t.Setenv("MCP_UI_ALLOW_INSECURE_BIND", "")
	t.Setenv("MCP_UI_ALLOWED_HOSTS", "")

	cfg, err := config.LoadFromEnv()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.UIEnabled || cfg.UIBindAddress != "127.0.0.1" || cfg.UIPort != "8081" || cfg.UIToken != "" || cfg.UIAllowInsecure {
		t.Fatalf("unexpected UI defaults: %+v", cfg)
	}

	t.Setenv("MCP_UI_ENABLED", "true")
	t.Setenv("MCP_UI_BIND_ADDRESS", "10.0.0.5")
	t.Setenv("MCP_UI_PORT", ":9000")
	t.Setenv("MCP_UI_TOKEN", " ui-secret ")
	t.Setenv("MCP_UI_ALLOW_INSECURE_BIND", "1")
	t.Setenv("MCP_UI_ALLOWED_HOSTS", "A.lan, b.lan ,")
	cfg, err = config.LoadFromEnv()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !cfg.UIEnabled || cfg.UIBindAddress != "10.0.0.5" || cfg.UIPort != "9000" || cfg.UIToken != "ui-secret" || !cfg.UIAllowInsecure {
		t.Fatalf("unexpected UI overrides: %+v", cfg)
	}
	if len(cfg.UIAllowedHosts) != 2 || cfg.UIAllowedHosts[0] != "a.lan" || cfg.UIAllowedHosts[1] != "b.lan" {
		t.Fatalf("unexpected allowed hosts: %v", cfg.UIAllowedHosts)
	}

	t.Setenv("MCP_UI_PORT", "99999")
	if _, err := config.LoadFromEnv(); err == nil {
		t.Fatal("expected error for invalid MCP_UI_PORT")
	}
}
