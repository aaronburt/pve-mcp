package config_test

import (
	"os"
	"testing"
	"time"

	"github.com/aaronburt/pve-mcp/internal/config"
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
	if cfg.AllowMutations {
		t.Errorf("got AllowMutations true, want default false")
	}
	if cfg.AllowDestroy {
		t.Errorf("got AllowDestroy true, want default false")
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
	t.Setenv("PVE_ALLOW_MUTATIONS", "true")

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
	if !cfg.AllowMutations {
		t.Errorf("got AllowMutations false, want true")
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

func TestLoadFromEnv_AllowDestroy(t *testing.T) {
	t.Setenv("PVE_HOST", "https://pve.example.com")
	t.Setenv("PVE_TOKEN_ID", "root@pam!token")
	t.Setenv("PVE_TOKEN_SECRET", "secret")

	t.Setenv("PVE_ALLOW_DESTROY", "true")
	cfg, err := config.LoadFromEnv()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !cfg.AllowDestroy {
		t.Errorf("expected AllowDestroy true when PVE_ALLOW_DESTROY=true")
	}

	t.Setenv("PVE_ALLOW_DESTROY", "0")
	cfg2, err := config.LoadFromEnv()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg2.AllowDestroy {
		t.Errorf("expected AllowDestroy false when PVE_ALLOW_DESTROY=0")
	}
}

func TestLoadDotEnv(t *testing.T) {
	config.LoadDotEnv("non_existent_file_path.env")

	tmp := t.TempDir() + "/test.env"
	content := "# comment line\nVAR1=hello\nVAR2=\"world\"\nVAR3='antigravity'\nVAR4=simple\nEMPTY=\nNO_EQUALS\n"
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
