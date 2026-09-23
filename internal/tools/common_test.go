package tools_test

import (
	"strings"
	"testing"

	"github.com/aaronburt/pve-mcp/internal/tools"
	"github.com/mark3labs/mcp-go/mcp"
)

func TestValidateNode(t *testing.T) {
	valid := []string{"pve", "node-1", "node_2", "PVE-01"}
	for _, n := range valid {
		res, err := tools.ValidateNode(n)
		if err != nil || res != n {
			t.Errorf("expected %s to be valid, got err: %v", n, err)
		}
	}

	invalid := []string{"", "../etc", "node/pve", "node 1", "node?query=1", "node#fragment"}
	for _, n := range invalid {
		_, err := tools.ValidateNode(n)
		if err == nil {
			t.Errorf("expected %s to be invalid, got nil error", n)
		}
	}
}

func TestValidateStorage(t *testing.T) {
	valid := []string{"local", "local-lvm", "nfs_backup", "ceph.pool-1"}
	for _, s := range valid {
		res, err := tools.ValidateStorage(s)
		if err != nil || res != s {
			t.Errorf("expected %s to be valid, got err: %v", s, err)
		}
	}

	invalid := []string{"", "../local", "storage/path", "pool;rm", "storage space"}
	for _, s := range invalid {
		_, err := tools.ValidateStorage(s)
		if err == nil {
			t.Errorf("expected %s to be invalid, got nil error", s)
		}
	}
}

func TestValidateVMID(t *testing.T) {
	valid := []int{100, 101, 999999, 999999999}
	for _, v := range valid {
		res, err := tools.ValidateVMID(v)
		if err != nil || res != v {
			t.Errorf("expected vmid %d to be valid, got err: %v", v, err)
		}
	}

	invalid := []int{-1, 0, 99, 1000000000}
	for _, v := range invalid {
		_, err := tools.ValidateVMID(v)
		if err == nil {
			t.Errorf("expected vmid %d to be invalid, got nil error", v)
		}
	}
}

func TestParseRequiredString(t *testing.T) {
	args := map[string]any{
		"node": "pve1",
		"bad":  123,
	}

	val, err := tools.ParseRequiredString(args, "node")
	if err != nil || val != "pve1" {
		t.Fatalf("unexpected val %s, err: %v", val, err)
	}

	_, err = tools.ParseRequiredString(args, "missing")
	if err == nil {
		t.Fatal("expected error for missing key")
	}

	_, err = tools.ParseRequiredString(args, "bad")
	if err == nil {
		t.Fatal("expected error for non-string key")
	}
}

func TestParseInt(t *testing.T) {
	args := map[string]any{
		"intVal":   100,
		"floatVal": float64(200),
		"strVal":   "300",
		"badVal":   "abc",
	}

	v1, err := tools.ParseRequiredInt(args, "intVal")
	if err != nil || v1 != 100 {
		t.Fatalf("failed to parse intVal: %d, err: %v", v1, err)
	}

	v2, err := tools.ParseRequiredInt(args, "floatVal")
	if err != nil || v2 != 200 {
		t.Fatalf("failed to parse floatVal: %d, err: %v", v2, err)
	}

	v3, err := tools.ParseRequiredInt(args, "strVal")
	if err != nil || v3 != 300 {
		t.Fatalf("failed to parse strVal: %d, err: %v", v3, err)
	}

	_, err = tools.ParseRequiredInt(args, "badVal")
	if err == nil {
		t.Fatal("expected error for badVal")
	}

	optVal, err := tools.ParseOptionalInt(args, "intVal", 50)
	if err != nil || optVal != 100 {
		t.Fatalf("failed optional parse: %d", optVal)
	}

	optFallback, err := tools.ParseOptionalInt(args, "notPresent", 50)
	if err != nil || optFallback != 50 {
		t.Fatalf("failed fallback optional: %d", optFallback)
	}
}

func TestTruncateList(t *testing.T) {
	items := []int{1, 2, 3, 4, 5}
	truncated, total, isTruncated := tools.TruncateList(items, 3)
	if !isTruncated || total != 5 || len(truncated) != 3 {
		t.Fatalf("unexpected truncate: %v, total %d, isTrunc %v", truncated, total, isTruncated)
	}

	notTruncated, total2, isTruncated2 := tools.TruncateList(items, 10)
	if isTruncated2 || total2 != 5 || len(notTruncated) != 5 {
		t.Fatalf("unexpected truncate: %v, total %d, isTrunc %v", notTruncated, total2, isTruncated2)
	}
}

func TestJSONAndErrorResult(t *testing.T) {
	type Sample struct {
		Name string `json:"name"`
	}

	res, err := tools.JSONResult(Sample{Name: "test"})
	if err != nil {
		t.Fatalf("unexpected JSONResult error: %v", err)
	}
	if res.IsError {
		t.Fatal("expected isError false")
	}

	errRes, err := tools.ErrorResult("something failed")
	if err != nil {
		t.Fatalf("unexpected ErrorResult error: %v", err)
	}
	if !errRes.IsError {
		t.Fatal("expected isError true")
	}
	tc, ok := mcp.AsTextContent(errRes.Content[0])
	if !ok || !strings.Contains(tc.Text, "something failed") {
		t.Fatalf("unexpected error text: %v", errRes.Content[0])
	}
}
