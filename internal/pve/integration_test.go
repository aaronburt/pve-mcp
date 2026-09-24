package pve_test

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/aaronburt/pve-mcp/internal/config"
	"github.com/aaronburt/pve-mcp/internal/pve"
)

func getLiveClient(t *testing.T) *pve.Client {
	t.Helper()

	host := os.Getenv("PVE_HOST")
	tokenID := os.Getenv("PVE_TOKEN_ID")
	tokenSecret := os.Getenv("PVE_TOKEN_SECRET")

	if host == "" || tokenID == "" || tokenSecret == "" {
		t.Skip("live PVE environment variables not set; skipping live integration tests")
	}

	cfg := &config.Config{
		Host:        host,
		TokenID:     tokenID,
		TokenSecret: tokenSecret,
		VerifySSL:   false,
		Timeout:     10 * time.Second,
	}

	client, err := pve.NewClient(cfg)
	if err != nil {
		t.Fatalf("failed to initialize live client: %v", err)
	}

	return client
}

func TestLive_PVE_Version(t *testing.T) {
	client := getLiveClient(t)

	var versionData struct {
		Version string `json:"version"`
		Release string `json:"release"`
		RepoID  string `json:"repoid"`
	}

	err := client.Get(context.Background(), "/version", nil, &versionData)
	if err != nil {
		t.Fatalf("failed to get live version: %v", err)
	}

	if versionData.Version == "" || versionData.Release == "" {
		t.Fatalf("unexpected version data: %+v", versionData)
	}
}

func TestLive_PVE_Nodes(t *testing.T) {
	client := getLiveClient(t)

	var nodes []pve.NodeItem
	err := client.Get(context.Background(), "/nodes", nil, &nodes)
	if err != nil {
		t.Fatalf("failed to list live nodes: %v", err)
	}

	if len(nodes) == 0 {
		t.Fatal("expected at least one node in live cluster")
	}

	for _, n := range nodes {
		if n.Node == "" {
			t.Errorf("expected node to have a name: %+v", n)
		}
	}
}

func TestLive_PVE_AuditPermissions(t *testing.T) {
	client := getLiveClient(t)

	var clusterStatus []pve.ClusterStatusItem
	err := client.Get(context.Background(), "/cluster/status", nil, &clusterStatus)
	if err != nil {
		if strings.Contains(err.Error(), "Sys.Audit") || strings.Contains(err.Error(), "Permission check failed") {
			t.Logf("Sys.Audit permission not yet assigned to token: %v", err)
			return
		}
		t.Fatalf("unexpected error fetching cluster status: %v", err)
	}

	if len(clusterStatus) == 0 {
		t.Fatal("expected cluster status entries")
	}
}
