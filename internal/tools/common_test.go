package tools_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/aaronburt/pve-mcp/internal/config"
	"github.com/aaronburt/pve-mcp/internal/pve"
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

func TestEnsureMutationsAllowed(t *testing.T) {
	if err := tools.EnsureMutationsAllowed(nil); err == nil {
		t.Fatal("expected error when cfg is nil")
	}

	cfgDisabled := &config.Config{AllowMutations: false}
	if err := tools.EnsureMutationsAllowed(cfgDisabled); err == nil {
		t.Fatal("expected error when AllowMutations is false")
	}

	cfgEnabled := &config.Config{AllowMutations: true}
	if err := tools.EnsureMutationsAllowed(cfgEnabled); err != nil {
		t.Fatalf("unexpected error when AllowMutations is true: %v", err)
	}
}

func TestParseOptionalBoolAndConfirmed(t *testing.T) {
	argsTrue := map[string]any{"confirm": true, "force": "true"}
	if !tools.IsConfirmed(argsTrue) {
		t.Fatal("expected IsConfirmed true")
	}
	if !tools.IsForce(argsTrue) {
		t.Fatal("expected IsForce true")
	}

	argsFalse := map[string]any{"confirm": false, "force": "0"}
	if tools.IsConfirmed(argsFalse) {
		t.Fatal("expected IsConfirmed false")
	}
	if tools.IsForce(argsFalse) {
		t.Fatal("expected IsForce false")
	}

	if tools.IsConfirmed(nil) {
		t.Fatal("expected IsConfirmed false for nil")
	}
}

func TestParseRequiredBool(t *testing.T) {
	val, err := tools.ParseRequiredBool(map[string]any{"flag": true}, "flag")
	if err != nil || !val {
		t.Fatalf("expected true, got %v, err: %v", val, err)
	}

	valStr, err := tools.ParseRequiredBool(map[string]any{"flag": "true"}, "flag")
	if err != nil || !valStr {
		t.Fatalf("expected true for string, got %v, err: %v", valStr, err)
	}

	valFalse, err := tools.ParseRequiredBool(map[string]any{"flag": false}, "flag")
	if err != nil || valFalse {
		t.Fatalf("expected false, got %v, err: %v", valFalse, err)
	}

	valFalseStr, err := tools.ParseRequiredBool(map[string]any{"flag": "0"}, "flag")
	if err != nil || valFalseStr {
		t.Fatalf("expected false for string '0', got %v, err: %v", valFalseStr, err)
	}

	_, errMissing := tools.ParseRequiredBool(map[string]any{}, "flag")
	if errMissing == nil {
		t.Fatal("expected error for missing key")
	}

	_, errInvalid := tools.ParseRequiredBool(map[string]any{"flag": "invalid"}, "flag")
	if errInvalid == nil {
		t.Fatal("expected error for invalid bool string")
	}

	_, errNil := tools.ParseRequiredBool(nil, "flag")
	if errNil == nil {
		t.Fatal("expected error for nil args")
	}
}

func TestValidatePowerAction(t *testing.T) {
	valid := []string{"start", "stop", "shutdown", "reboot", "suspend", "resume", "START", " Stop "}
	for _, a := range valid {
		act, err := tools.ValidatePowerAction(a)
		if err != nil {
			t.Fatalf("expected action %q to be valid: %v", a, err)
		}
		if act == "" {
			t.Fatal("expected non-empty action")
		}
	}

	invalid := []string{"", "destroy", "kill", "hibernate", "pause"}
	for _, a := range invalid {
		if _, err := tools.ValidatePowerAction(a); err == nil {
			t.Fatalf("expected action %q to be invalid", a)
		}
	}
}

func TestValidateDiskIdentifier(t *testing.T) {
	validVM := []string{"scsi0", "scsi30", "virtio0", "ide1", "sata2"}
	for _, d := range validVM {
		if _, err := tools.ValidateDiskIdentifier(d, true); err != nil {
			t.Fatalf("expected vm disk %q to be valid: %v", d, err)
		}
	}

	invalidVM := []string{"rootfs", "mp0", "sd0", "scsi", "../scsi0"}
	for _, d := range invalidVM {
		if _, err := tools.ValidateDiskIdentifier(d, true); err == nil {
			t.Fatalf("expected vm disk %q to be invalid", d)
		}
	}

	validLXC := []string{"rootfs", "mp0", "mp1", "mp9"}
	for _, d := range validLXC {
		if _, err := tools.ValidateDiskIdentifier(d, false); err != nil {
			t.Fatalf("expected lxc disk %q to be valid: %v", d, err)
		}
	}

	invalidLXC := []string{"scsi0", "virtio0", "mp", "root"}
	for _, d := range invalidLXC {
		if _, err := tools.ValidateDiskIdentifier(d, false); err == nil {
			t.Fatalf("expected lxc disk %q to be invalid", d)
		}
	}
}

func TestValidateDiskSizeAndGrowth(t *testing.T) {
	validSizes := []string{"+10G", "50G", "+500M", "1024M", "+1T"}
	for _, s := range validSizes {
		if _, err := tools.ValidateDiskSize(s); err != nil {
			t.Fatalf("expected size %q to be valid: %v", s, err)
		}
	}

	invalidSizes := []string{"", "10", "+", "G", "50GB", "-5G"}
	for _, s := range invalidSizes {
		if _, err := tools.ValidateDiskSize(s); err == nil {
			t.Fatalf("expected size %q to be invalid", s)
		}
	}

	if err := tools.ValidateDiskGrowth(10*1024*1024*1024, "+5G"); err != nil {
		t.Fatalf("expected +5G growth to be valid: %v", err)
	}

	if err := tools.ValidateDiskGrowth(10*1024*1024*1024, "20G"); err != nil {
		t.Fatalf("expected 20G growth over 10G to be valid: %v", err)
	}

	if err := tools.ValidateDiskGrowth(20*1024*1024*1024, "10G"); err == nil {
		t.Fatal("expected shrink (10G target over 20G current) to fail")
	}

	if err := tools.ValidateDiskGrowth(10*1024*1024*1024, "10G"); err == nil {
		t.Fatal("expected equal size (10G target over 10G current) to fail")
	}

	diskStr := "local-lvm:vm-100-disk-0,size=32G"
	if bytes := tools.ExtractDiskSizeBytes(diskStr); bytes != 32*1024*1024*1024 {
		t.Fatalf("expected 32GB in bytes, got %d", bytes)
	}
	if bytes := tools.ExtractDiskSizeBytes("no-size-field"); bytes != 0 {
		t.Fatalf("expected 0 bytes for missing size, got %d", bytes)
	}
}

func TestValidateNetID(t *testing.T) {
	valid := []string{"net0", "net1", "net31"}
	for _, n := range valid {
		if _, err := tools.ValidateNetID(n); err != nil {
			t.Fatalf("expected %q to be valid netID: %v", n, err)
		}
	}

	invalid := []string{"eth0", "net", "net-0", "0net"}
	for _, n := range invalid {
		if _, err := tools.ValidateNetID(n); err == nil {
			t.Fatalf("expected %q to be invalid netID", n)
		}
	}
}

func TestMergeNetworkConfig(t *testing.T) {
	current := "virtio=BC:24:11:22:33:44,bridge=vmbr0,firewall=1"
	updates := map[string]string{
		"bridge": "vmbr1",
		"tag":    "100",
	}
	merged := tools.MergeNetworkConfig(current, updates)
	if !strings.Contains(merged, "virtio=BC:24:11:22:33:44") {
		t.Fatalf("expected MAC preserved in: %s", merged)
	}
	if !strings.Contains(merged, "bridge=vmbr1") {
		t.Fatalf("expected bridge updated in: %s", merged)
	}
	if !strings.Contains(merged, "tag=100") {
		t.Fatalf("expected tag added in: %s", merged)
	}
	if !strings.Contains(merged, "firewall=1") {
		t.Fatalf("expected firewall preserved in: %s", merged)
	}
}

func TestFormatDryRunPreview(t *testing.T) {
	preview := tools.FormatDryRunPreview("Update VM 100", []string{"cores: 2 -> 4"})
	if !strings.Contains(preview, "[DRY RUN PREVIEW - USER CONFIRMATION REQUIRED]") {
		t.Fatal("missing header in preview")
	}
	if !strings.Contains(preview, "cores: 2 -> 4") {
		t.Fatal("missing diff in preview")
	}
	if !strings.Contains(preview, "confirm") {
		t.Fatal("missing confirm instructions in preview")
	}
}

func TestEnsureDestroyAllowed(t *testing.T) {
	if err := tools.EnsureDestroyAllowed(nil); err == nil {
		t.Fatal("expected error for nil config")
	}

	cfg := &config.Config{AllowDestroy: false}
	if err := tools.EnsureDestroyAllowed(cfg); err == nil {
		t.Fatal("expected error when AllowDestroy is false")
	}

	cfg.AllowDestroy = true
	if err := tools.EnsureDestroyAllowed(cfg); err != nil {
		t.Fatalf("unexpected error when AllowDestroy is true: %v", err)
	}
}

func TestWaitForTask(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"data":{"status":"stopped","exitstatus":"OK"}}`))
	}))
	defer server.Close()

	cfg := &config.Config{
		Host:        server.URL,
		TokenID:     "root@pam!token",
		TokenSecret: "secret",
		VerifySSL:   false,
	}
	client, err := pve.NewClient(cfg)
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	status, err := tools.WaitForTask(context.Background(), client, "pve1", "UPID:pve1:123", 5, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if status.Status != "stopped" || status.ExitStatus != "OK" {
		t.Errorf("unexpected status: %+v", status)
	}

	meta := &mcp.Meta{ProgressToken: "test-token"}
	metaStatus, err := tools.WaitForTask(context.Background(), client, "pve1", "UPID:pve1:123", 5, meta)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if metaStatus.Status != "stopped" {
		t.Errorf("unexpected status: %+v", metaStatus)
	}
}

func TestEmitProgress_EdgeCases(t *testing.T) {
	tools.EmitProgress(context.Background(), nil, 10, 30)
	tools.EmitProgress(context.Background(), &mcp.Meta{}, 10, 30)
	tools.EmitProgress(context.Background(), &mcp.Meta{ProgressToken: "token"}, 10, 30)
}

func TestCommon_EdgeCasesAndErrors(t *testing.T) {
	_, err := tools.JSONResult(make(chan int))
	if err == nil {
		t.Fatal("expected JSONResult to fail for unmarshalable type")
	}

	_, err = tools.ParseRequiredInt(nil, "key")
	if err == nil {
		t.Fatal("expected ParseRequiredInt to fail on nil")
	}

	_, err = tools.ParseRequiredInt(map[string]any{}, "missing")
	if err == nil {
		t.Fatal("expected ParseRequiredInt to fail on missing key")
	}

	_, err = tools.ParseRequiredInt(map[string]any{"bad": struct{}{}}, "bad")
	if err == nil {
		t.Fatal("expected ParseRequiredInt to fail on non-number/non-string")
	}

	_, err = tools.ParseOptionalInt(map[string]any{"bad": struct{}{}}, "bad", 10)
	if err == nil {
		t.Fatal("expected ParseOptionalInt to fail on struct")
	}

	vOptStr, err := tools.ParseOptionalInt(map[string]any{"str": "42"}, "str", 10)
	if err != nil || vOptStr != 42 {
		t.Fatalf("expected ParseOptionalInt string to parse, got %d, %v", vOptStr, err)
	}

	_, err = tools.ParseOptionalInt(map[string]any{"str": "invalid"}, "str", 10)
	if err == nil {
		t.Fatal("expected ParseOptionalInt to fail on invalid numeric string")
	}

	optStr := tools.ParseOptionalString(map[string]any{"text": "hello"}, "text", "default")
	if optStr != "hello" {
		t.Fatalf("expected 'hello', got %s", optStr)
	}

	optFallback := tools.ParseOptionalString(map[string]any{}, "missing", "default")
	if optFallback != "default" {
		t.Fatalf("expected 'default', got %s", optFallback)
	}

	optBad := tools.ParseOptionalString(map[string]any{"bad": 123}, "bad", "default")
	if optBad != "default" {
		t.Fatalf("expected 'default' for non-string, got %s", optBad)
	}

	invalidSizes := []string{"invalid", "100X", "G", "+", ""}
	for _, sz := range invalidSizes {
		if _, err := tools.ParseSizeBytes(sz); err == nil {
			t.Fatalf("expected ParseSizeBytes(%q) to fail", sz)
		}
	}

	serverErr := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "internal server error", http.StatusInternalServerError)
	}))
	defer serverErr.Close()

	clientErr, _ := pve.NewClient(&config.Config{
		Host:        serverErr.URL,
		TokenID:     "root@pam!token",
		TokenSecret: "secret",
		VerifySSL:   false,
	})

	_, err = tools.WaitForTask(context.Background(), clientErr, "pve1", "UPID:pve1:123", 1, nil)
	if err == nil {
		t.Fatal("expected WaitForTask to fail when client returns error")
	}

	serverLoop := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"data":{"status":"running"}}`))
	}))
	defer serverLoop.Close()

	clientLoop, _ := pve.NewClient(&config.Config{
		Host:        serverLoop.URL,
		TokenID:     "root@pam!token",
		TokenSecret: "secret",
		VerifySSL:   false,
	})

	cancelCtx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = tools.WaitForTask(cancelCtx, clientLoop, "pve1", "UPID:pve1:123", 10, nil)
	if err == nil {
		t.Fatal("expected WaitForTask to fail when context canceled")
	}

	timedOutStatus, err := tools.WaitForTask(context.Background(), clientLoop, "pve1", "UPID:pve1:123", 1, nil)
	if err != nil || timedOutStatus == nil || timedOutStatus.Status != "running" {
		t.Fatalf("expected running status on timeout: %v, status: %+v", err, timedOutStatus)
	}

	vInt32, err := tools.ParseRequiredInt(map[string]any{"num": int32(10)}, "num")
	if err != nil || vInt32 != 10 {
		t.Fatalf("expected int32 10, got %d, %v", vInt32, err)
	}

	vInt64, err := tools.ParseRequiredInt(map[string]any{"num": int64(20)}, "num")
	if err != nil || vInt64 != 20 {
		t.Fatalf("expected int64 20, got %d, %v", vInt64, err)
	}

	vFloat32, err := tools.ParseRequiredInt(map[string]any{"num": float32(30)}, "num")
	if err != nil || vFloat32 != 30 {
		t.Fatalf("expected float32 30, got %d, %v", vFloat32, err)
	}

	vJsonNum, err := tools.ParseRequiredInt(map[string]any{"num": json.Number("40")}, "num")
	if err != nil || vJsonNum != 40 {
		t.Fatalf("expected json.Number 40, got %d, %v", vJsonNum, err)
	}

	_, err = tools.ParseRequiredInt(map[string]any{"num": json.Number("invalid")}, "num")
	if err == nil {
		t.Fatal("expected error on invalid json.Number")
	}

	_, err = tools.ParseRequiredString(nil, "key")
	if err == nil {
		t.Fatal("expected error on nil args for ParseRequiredString")
	}

	_, err = tools.ParseRequiredString(map[string]any{"key": "   "}, "key")
	if err == nil {
		t.Fatal("expected error on whitespace-only string for ParseRequiredString")
	}

	if opt := tools.ParseOptionalString(nil, "key", "def"); opt != "def" {
		t.Fatalf("expected def on nil args, got %s", opt)
	}

	if opt := tools.ParseOptionalString(map[string]any{"key": "   "}, "key", "def"); opt != "def" {
		t.Fatalf("expected def on whitespace string, got %s", opt)
	}

	resNull, err := tools.JSONResult(nil)
	if err != nil || resNull.IsError {
		t.Fatalf("expected JSONResult(nil) success: %v", err)
	}

	resStr, err := tools.JSONResult("raw-string")
	if err != nil || resStr.IsError {
		t.Fatalf("expected JSONResult(string) success: %v", err)
	}

	resBytes, err := tools.JSONResult([]byte("raw-bytes"))
	if err != nil || resBytes.IsError {
		t.Fatalf("expected JSONResult([]byte) success: %v", err)
	}

	resRaw, err := tools.JSONResult(json.RawMessage(`{"a":1}`))
	if err != nil || resRaw.IsError {
		t.Fatalf("expected JSONResult(RawMessage) success: %v", err)
	}

	szM, err := tools.ParseSizeBytes("100M")
	if err != nil || szM != 100*1024*1024 {
		t.Fatalf("expected 100M, got %d, %v", szM, err)
	}

	szG, err := tools.ParseSizeBytes("500G")
	if err != nil || szG != 500*1024*1024*1024 {
		t.Fatalf("expected 500G, got %d, %v", szG, err)
	}

	szT, err := tools.ParseSizeBytes("2T")
	if err != nil || szT != 2*1024*1024*1024*1024 {
		t.Fatalf("expected 2T, got %d, %v", szT, err)
	}
}

func TestVerifyTargetIdentity(t *testing.T) {
	if err := tools.VerifyTargetIdentity(map[string]any{}, "web-app", false); err != nil {
		t.Fatalf("expected nil when optional and omitted: %v", err)
	}

	if err := tools.VerifyTargetIdentity(map[string]any{}, "web-app", true); err == nil {
		t.Fatal("expected error when required but omitted")
	}

	if err := tools.VerifyTargetIdentity(nil, "web-app", true); err == nil {
		t.Fatal("expected error when required and nil args")
	}

	if err := tools.VerifyTargetIdentity(map[string]any{"expected_name": "   "}, "web-app", true); err == nil {
		t.Fatal("expected error when required and blank string")
	}

	if err := tools.VerifyTargetIdentity(map[string]any{"expected_name": "wrong-app"}, "web-app", false); err == nil {
		t.Fatal("expected mismatch error")
	}

	if err := tools.VerifyTargetIdentity(map[string]any{"expected_name": "web-app"}, "web-app", true); err != nil {
		t.Fatalf("expected success on match: %v", err)
	}
}

