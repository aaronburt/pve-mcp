package tools_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/aaronburt/pve-mcp/internal/config"
	"github.com/aaronburt/pve-mcp/internal/pve"
	"github.com/aaronburt/pve-mcp/internal/tools"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

type toolTestCase struct {
	name        string
	validArgs   map[string]any
	invalidArgs map[string]any
}

func TestTools_RegisterAndExecuteAll(t *testing.T) {
	mockPVE := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if strings.Contains(r.URL.Path, "/tasks/") {
			w.Write([]byte(`{"data":{"status":"stopped","exitstatus":"OK"}}`))
			return
		}
		w.Write([]byte(`{"data":[{"status":"ok"}]}`))
	}))
	defer mockPVE.Close()

	cfg := &config.Config{
		Host:        mockPVE.URL,
		TokenID:     "user@pve!token",
		TokenSecret: "secret",
		VerifySSL:   false,
	}

	client, err := pve.NewClient(cfg)
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	mcpServer := server.NewMCPServer("pve-mcp", "1.0.0")
	tools.RegisterAll(mcpServer, client, cfg)

	cases := []toolTestCase{
		{name: "pve_task_status", validArgs: map[string]any{"node": "pve", "upid": "UPID:pve:123"}, invalidArgs: map[string]any{"node": "../bad", "upid": "UPID:pve:123"}},
	}

	if len(cases) != 1 {
		t.Fatalf("expected exactly 1 test case, got %d", len(cases))
	}

	allTools := mcpServer.ListTools()
	if len(allTools) != 16 {
		t.Fatalf("expected exactly 16 registered tools, got %d", len(allTools))
	}

	ctx := context.Background()

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			st := mcpServer.GetTool(tc.name)
			if st == nil {
				t.Fatalf("tool %s not registered in server", tc.name)
			}

			reqValid := mcp.CallToolRequest{
				Params: mcp.CallToolParams{
					Name:      tc.name,
					Arguments: tc.validArgs,
				},
			}
			resValid, err := st.Handler(ctx, reqValid)
			if err != nil || resValid == nil || resValid.IsError {
				t.Fatalf("tool %s failed valid execution: err=%v, res=%+v", tc.name, err, resValid)
			}

			if tc.invalidArgs != nil {
				reqInvalid := mcp.CallToolRequest{
					Params: mcp.CallToolParams{
						Name:      tc.name,
						Arguments: tc.invalidArgs,
					},
				}
				resInvalid, err := st.Handler(ctx, reqInvalid)
				if err != nil || resInvalid == nil || !resInvalid.IsError {
					t.Fatalf("tool %s expected validation error on invalid args: err=%v, res=%+v", tc.name, err, resInvalid)
				}
			}
		})
	}

	errPVE := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "simulated pve failure", http.StatusInternalServerError)
	}))
	defer errPVE.Close()

	errClient, err := pve.NewClient(&config.Config{
		Host:        errPVE.URL,
		TokenID:     "user@pve!token",
		TokenSecret: "secret",
		VerifySSL:   false,
	})
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	errServer := server.NewMCPServer("pve-mcp", "1.0.0")
	tools.RegisterAll(errServer, errClient, cfg)

	for _, tc := range cases {
		st := errServer.GetTool(tc.name)
		res, _ := st.Handler(ctx, mcp.CallToolRequest{
			Params: mcp.CallToolParams{
				Name:      tc.name,
				Arguments: tc.validArgs,
			},
		})
		if res == nil || !res.IsError {
			t.Fatalf("expected error result from %s when PVE fails, got %+v", tc.name, res)
		}
	}
}

func TestMutatingTools_GuardrailsAndExecution(t *testing.T) {
	mockPVE := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api2/json/nodes/pve/qemu/100/status/current":
			w.Write([]byte(`{"data":{"status":"running","name":"vm100"}}`))
		case "/api2/json/nodes/pve/qemu/100/config":
			if r.Method == http.MethodGet {
				w.Write([]byte(`{"data":{"cores":2,"sockets":1,"memory":2048,"scsi0":"local-lvm:vm-100-disk-0,size=32G","net0":"virtio=AA:BB:CC:DD:EE:FF,bridge=vmbr0,firewall=1"}}`))
			} else {
				w.Write([]byte(`{"data":null}`))
			}
		case "/api2/json/nodes/pve/qemu/100/status/start":
			w.Write([]byte(`{"data":"UPID:pve:0001:0002:66F2:qmstart:100:root@pam:"}`))
		case "/api2/json/nodes/pve/qemu/100/status/stop":
			w.Write([]byte(`{"data":"UPID:pve:0001:0002:66F2:qmstop:100:root@pam:"}`))
		case "/api2/json/nodes/pve/qemu/100/resize":
			w.Write([]byte(`{"data":"UPID:pve:0001:0002:66F2:qmresize:100:root@pam:"}`))
		case "/api2/json/nodes/pve/lxc/200/status/current":
			w.Write([]byte(`{"data":{"status":"stopped","name":"ct200"}}`))
		case "/api2/json/nodes/pve/lxc/200/config":
			if r.Method == http.MethodGet {
				w.Write([]byte(`{"data":{"cores":2,"memory":1024,"swap":512,"rootfs":"local-zfs:subvol-200-disk-0,size=8G","net0":"name=eth0,bridge=vmbr0,ip=dhcp"}}`))
			} else {
				w.Write([]byte(`{"data":null}`))
			}
		case "/api2/json/nodes/pve/lxc/200/status/start":
			w.Write([]byte(`{"data":"UPID:pve:0001:0002:66F2:vzstart:200:root@pam:"}`))
		case "/api2/json/nodes/pve/lxc/200/resize":
			w.Write([]byte(`{"data":"UPID:pve:0001:0002:66F2:vzresize:200:root@pam:"}`))
		case "/api2/json/nodes/pve/qemu/100/clone":
			w.Write([]byte(`{"data":"UPID:pve:0001:0002:66F2:qmdclone:100:root@pam:"}`))
		case "/api2/json/nodes/pve/qemu/101/status/current":
			w.Write([]byte(`{"data":{"status":"stopped","name":"vm101"}}`))
		case "/api2/json/nodes/pve/qemu/101/config":
			w.Write([]byte(`{"data":{}}`))
		case "/api2/json/nodes/pve/qemu/101":
			w.Write([]byte(`{"data":"UPID:pve:0001:0002:66F2:qmdestroy:101:root@pam:"}`))
		case "/api2/json/nodes/pve/lxc/200/clone":
			w.Write([]byte(`{"data":"UPID:pve:0001:0002:66F2:vzclone:200:root@pam:"}`))
		case "/api2/json/nodes/pve/lxc":
			w.Write([]byte(`{"data":"UPID:pve:0001:0002:66F2:vzcreate:300:root@pam:"}`))
		case "/api2/json/nodes/pve/lxc/200":
			w.Write([]byte(`{"data":"UPID:pve:0001:0002:66F2:vzdestroy:200:root@pam:"}`))
		case "/api2/json/nodes/pve/tasks/UPID:pve:0001:0002:66F2:qmdclone:100:root@pam:/status",
			"/api2/json/nodes/pve/tasks/UPID:pve:0001:0002:66F2:vzclone:200:root@pam:/status",
			"/api2/json/nodes/pve/tasks/UPID:pve:0001:0002:66F2:vzcreate:300:root@pam:/status":
			w.Write([]byte(`{"data":{"status":"stopped","exitstatus":"OK"}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer mockPVE.Close()

	ctx := context.Background()

	cfgDisabled := &config.Config{
		Host:           mockPVE.URL,
		TokenID:        "user@pve!token",
		TokenSecret:    "secret",
		AllowMutations: false,
	}
	clientDisabled, _ := pve.NewClient(cfgDisabled)
	serverDisabled := server.NewMCPServer("pve-mcp", "1.0.0")
	tools.RegisterAll(serverDisabled, clientDisabled, cfgDisabled)

	mutatingToolNames := []string{
		"pve_qemu_power", "pve_qemu_update_hardware", "pve_qemu_resize_disk", "pve_qemu_update_network",
		"pve_lxc_power", "pve_lxc_update_hardware", "pve_lxc_resize_disk", "pve_lxc_update_network",
		"pve_qemu_clone", "pve_qemu_destroy",
		"pve_lxc_clone", "pve_lxc_create", "pve_lxc_destroy",
		"pve_qemu_protection", "pve_lxc_protection",
	}

	for _, name := range mutatingToolNames {
		st := serverDisabled.GetTool(name)
		if st == nil {
			t.Fatalf("mutating tool %s not registered", name)
		}
		res, err := st.Handler(ctx, mcp.CallToolRequest{
			Params: mcp.CallToolParams{
				Name:      name,
				Arguments: map[string]any{"node": "pve", "vmid": 100},
			},
		})
		if err != nil || res == nil || !res.IsError {
			t.Fatalf("expected mutation guardrail error for %s, got %+v", name, res)
		}
		tc, _ := mcp.AsTextContent(res.Content[0])
		if !strings.Contains(tc.Text, "mutations are disabled") {
			t.Fatalf("expected 'mutations are disabled' message, got: %s", tc.Text)
		}
	}

	cfgEnabled := &config.Config{
		Host:           mockPVE.URL,
		TokenID:        "user@pve!token",
		TokenSecret:    "secret",
		AllowMutations: true,
		AllowDestroy:   true,
	}
	clientEnabled, _ := pve.NewClient(cfgEnabled)
	serverEnabled := server.NewMCPServer("pve-mcp", "1.0.0")
	tools.RegisterAll(serverEnabled, clientEnabled, cfgEnabled)

	t.Run("qemu_power_dry_run_and_confirm", func(t *testing.T) {
		st := serverEnabled.GetTool("pve_qemu_power")
		res, _ := st.Handler(ctx, mcp.CallToolRequest{
			Params: mcp.CallToolParams{
				Arguments: map[string]any{"node": "pve", "vmid": 100, "action": "start"},
			},
		})
		tc, _ := mcp.AsTextContent(res.Content[0])
		if !strings.Contains(tc.Text, "[DRY RUN PREVIEW - USER CONFIRMATION REQUIRED]") {
			t.Fatalf("expected dry-run preview, got: %s", tc.Text)
		}

		resExec, _ := st.Handler(ctx, mcp.CallToolRequest{
			Params: mcp.CallToolParams{
				Arguments: map[string]any{"node": "pve", "vmid": 100, "action": "start", "confirm": true},
			},
		})
		if resExec.IsError {
			t.Fatalf("unexpected error executing power start: %+v", resExec)
		}
		tcExec, _ := mcp.AsTextContent(resExec.Content[0])
		if !strings.Contains(tcExec.Text, "UPID:pve") {
			t.Fatalf("expected UPID in execution result, got: %s", tcExec.Text)
		}
	})

	t.Run("qemu_power_stop_requires_force", func(t *testing.T) {
		st := serverEnabled.GetTool("pve_qemu_power")
		res, _ := st.Handler(ctx, mcp.CallToolRequest{
			Params: mcp.CallToolParams{
				Arguments: map[string]any{"node": "pve", "vmid": 100, "action": "stop", "confirm": true},
			},
		})
		if !res.IsError {
			t.Fatal("expected error when stop action is called without force")
		}
		tc, _ := mcp.AsTextContent(res.Content[0])
		if !strings.Contains(tc.Text, "force power-off") {
			t.Fatalf("expected force warning, got: %s", tc.Text)
		}

		resMissingName, _ := st.Handler(ctx, mcp.CallToolRequest{
			Params: mcp.CallToolParams{
				Arguments: map[string]any{"node": "pve", "vmid": 100, "action": "stop", "force": true, "confirm": true},
			},
		})
		if !resMissingName.IsError {
			t.Fatal("expected error when stop action is called without expected_name")
		}

		resMismatchName, _ := st.Handler(ctx, mcp.CallToolRequest{
			Params: mcp.CallToolParams{
				Arguments: map[string]any{"node": "pve", "vmid": 100, "action": "stop", "force": true, "confirm": true, "expected_name": "wrong-name"},
			},
		})
		if !resMismatchName.IsError {
			t.Fatal("expected error when stop action is called with mismatched expected_name")
		}

		resForce, _ := st.Handler(ctx, mcp.CallToolRequest{
			Params: mcp.CallToolParams{
				Arguments: map[string]any{"node": "pve", "vmid": 100, "action": "stop", "force": true, "confirm": true, "expected_name": "vm100"},
			},
		})
		if resForce.IsError {
			t.Fatalf("unexpected error executing force stop: %+v", resForce)
		}
	})

	t.Run("qemu_update_hardware_dry_run_and_confirm", func(t *testing.T) {
		st := serverEnabled.GetTool("pve_qemu_update_hardware")
		res, _ := st.Handler(ctx, mcp.CallToolRequest{
			Params: mcp.CallToolParams{
				Arguments: map[string]any{"node": "pve", "vmid": 100, "cores": 4, "memory": 4096},
			},
		})
		tc, _ := mcp.AsTextContent(res.Content[0])
		if !strings.Contains(tc.Text, "cores: 2 -> 4") || !strings.Contains(tc.Text, "memory: 2048 MB -> 4096 MB") {
			t.Fatalf("expected diff preview, got: %s", tc.Text)
		}

		resExec, _ := st.Handler(ctx, mcp.CallToolRequest{
			Params: mcp.CallToolParams{
				Arguments: map[string]any{"node": "pve", "vmid": 100, "cores": 4, "memory": 4096, "confirm": true},
			},
		})
		if resExec.IsError {
			t.Fatalf("unexpected error executing hardware update: %+v", resExec)
		}
	})

	t.Run("qemu_resize_disk_growth_and_shrink_rejection", func(t *testing.T) {
		st := serverEnabled.GetTool("pve_qemu_resize_disk")
		resShrink, _ := st.Handler(ctx, mcp.CallToolRequest{
			Params: mcp.CallToolParams{
				Arguments: map[string]any{"node": "pve", "vmid": 100, "disk": "scsi0", "size": "16G"},
			},
		})
		if !resShrink.IsError {
			t.Fatal("expected shrink error")
		}

		resGrow, _ := st.Handler(ctx, mcp.CallToolRequest{
			Params: mcp.CallToolParams{
				Arguments: map[string]any{"node": "pve", "vmid": 100, "disk": "scsi0", "size": "+10G", "confirm": true},
			},
		})
		if resGrow.IsError {
			t.Fatalf("unexpected error on disk growth: %+v", resGrow)
		}
	})

	t.Run("qemu_update_network_merge_preservation", func(t *testing.T) {
		st := serverEnabled.GetTool("pve_qemu_update_network")
		resPreview, _ := st.Handler(ctx, mcp.CallToolRequest{
			Params: mcp.CallToolParams{
				Arguments: map[string]any{"node": "pve", "vmid": 100, "net_id": "net0", "bridge": "vmbr1", "tag": 100},
			},
		})
		tc, _ := mcp.AsTextContent(resPreview.Content[0])
		if !strings.Contains(tc.Text, "AA:BB:CC:DD:EE:FF") {
			t.Fatalf("expected MAC preserved in network preview, got: %s", tc.Text)
		}

		resExec, _ := st.Handler(ctx, mcp.CallToolRequest{
			Params: mcp.CallToolParams{
				Arguments: map[string]any{"node": "pve", "vmid": 100, "net_id": "net0", "bridge": "vmbr1", "confirm": true},
			},
		})
		if resExec.IsError {
			t.Fatalf("unexpected error on network update: %+v", resExec)
		}
	})

	t.Run("lxc_power_and_hardware_and_resize", func(t *testing.T) {
		pwr := serverEnabled.GetTool("pve_lxc_power")
		pwrRes, _ := pwr.Handler(ctx, mcp.CallToolRequest{
			Params: mcp.CallToolParams{
				Arguments: map[string]any{"node": "pve", "vmid": 200, "action": "start", "confirm": true},
			},
		})
		if pwrRes.IsError {
			t.Fatalf("unexpected lxc power error: %+v", pwrRes)
		}

		hw := serverEnabled.GetTool("pve_lxc_update_hardware")
		hwRes, _ := hw.Handler(ctx, mcp.CallToolRequest{
			Params: mcp.CallToolParams{
				Arguments: map[string]any{"node": "pve", "vmid": 200, "cores": 4, "memory": 2048, "confirm": true},
			},
		})
		if hwRes.IsError {
			t.Fatalf("unexpected lxc hardware update error: %+v", hwRes)
		}

		disk := serverEnabled.GetTool("pve_lxc_resize_disk")
		diskRes, _ := disk.Handler(ctx, mcp.CallToolRequest{
			Params: mcp.CallToolParams{
				Arguments: map[string]any{"node": "pve", "vmid": 200, "disk": "rootfs", "size": "+4G", "confirm": true},
			},
		})
		if diskRes.IsError {
			t.Fatalf("unexpected lxc disk resize error: %+v", diskRes)
		}

		net := serverEnabled.GetTool("pve_lxc_update_network")
		netRes, _ := net.Handler(ctx, mcp.CallToolRequest{
			Params: mcp.CallToolParams{
				Arguments: map[string]any{"node": "pve", "vmid": 200, "net_id": "net0", "bridge": "vmbr2", "confirm": true},
			},
		})
		if netRes.IsError {
			t.Fatalf("unexpected lxc network update error: %+v", netRes)
		}
	})

	cfgNoDestroy := &config.Config{
		Host:           mockPVE.URL,
		TokenID:        "user@pve!token",
		TokenSecret:    "secret",
		AllowMutations: true,
		AllowDestroy:   false,
	}
	clientNoDestroy, _ := pve.NewClient(cfgNoDestroy)
	serverNoDestroy := server.NewMCPServer("pve-mcp", "1.0.0")
	tools.RegisterAll(serverNoDestroy, clientNoDestroy, cfgNoDestroy)

	t.Run("destroy_disabled_safety_check", func(t *testing.T) {
		qemuDestroy := serverNoDestroy.GetTool("pve_qemu_destroy")
		resQ, err := qemuDestroy.Handler(ctx, mcp.CallToolRequest{
			Params: mcp.CallToolParams{
				Arguments: map[string]any{"node": "pve", "vmid": 101, "confirm": true},
			},
		})
		if err != nil || resQ == nil || !resQ.IsError {
			t.Fatalf("expected error when PVE_ALLOW_DESTROY=false, got: %+v", resQ)
		}
		tcQ, _ := mcp.AsTextContent(resQ.Content[0])
		if !strings.Contains(tcQ.Text, "machine destruction is disabled") {
			t.Fatalf("expected destroy disabled message, got: %s", tcQ.Text)
		}

		lxcDestroy := serverNoDestroy.GetTool("pve_lxc_destroy")
		resL, err := lxcDestroy.Handler(ctx, mcp.CallToolRequest{
			Params: mcp.CallToolParams{
				Arguments: map[string]any{"node": "pve", "vmid": 200, "confirm": true},
			},
		})
		if err != nil || resL == nil || !resL.IsError {
			t.Fatalf("expected error when PVE_ALLOW_DESTROY=false for lxc, got: %+v", resL)
		}
	})

	t.Run("destroy_running_machine_prevented", func(t *testing.T) {
		qemuDestroy := serverEnabled.GetTool("pve_qemu_destroy")
		res, err := qemuDestroy.Handler(ctx, mcp.CallToolRequest{
			Params: mcp.CallToolParams{
				Arguments: map[string]any{"node": "pve", "vmid": 100, "confirm": true, "expected_name": "vm100"},
			},
		})
		if err != nil || res == nil || !res.IsError {
			t.Fatalf("expected error trying to destroy running VM 100, got: %+v", res)
		}
		tc, _ := mcp.AsTextContent(res.Content[0])
		if !strings.Contains(tc.Text, "cannot destroy VM 100 while it is") {
			t.Fatalf("expected stopped error message, got: %s", tc.Text)
		}
	})

	t.Run("qemu_clone_and_destroy", func(t *testing.T) {
		cloneTool := serverEnabled.GetTool("pve_qemu_clone")
		resPreview, _ := cloneTool.Handler(ctx, mcp.CallToolRequest{
			Params: mcp.CallToolParams{
				Arguments: map[string]any{"node": "pve", "vmid": 100, "newid": 102, "name": "cloned-vm"},
			},
		})
		tcPreview, _ := mcp.AsTextContent(resPreview.Content[0])
		if !strings.Contains(tcPreview.Text, "[DRY RUN PREVIEW - USER CONFIRMATION REQUIRED]") {
			t.Fatalf("expected dry-run preview for clone, got: %s", tcPreview.Text)
		}

		resExec, _ := cloneTool.Handler(ctx, mcp.CallToolRequest{
			Params: mcp.CallToolParams{
				Arguments: map[string]any{"node": "pve", "vmid": 100, "newid": 102, "name": "cloned-vm", "confirm": true},
			},
		})
		if resExec.IsError {
			t.Fatalf("unexpected error cloning VM: %+v", resExec)
		}
		tcExec, _ := mcp.AsTextContent(resExec.Content[0])
		if !strings.Contains(tcExec.Text, "Successfully cloned VM 100 to 102") {
			t.Fatalf("expected clone success message, got: %s", tcExec.Text)
		}

		destroyTool := serverEnabled.GetTool("pve_qemu_destroy")
		resDestPreview, _ := destroyTool.Handler(ctx, mcp.CallToolRequest{
			Params: mcp.CallToolParams{
				Arguments: map[string]any{"node": "pve", "vmid": 101, "expected_name": "vm101"},
			},
		})
		tcDestPreview, _ := mcp.AsTextContent(resDestPreview.Content[0])
		if !strings.Contains(tcDestPreview.Text, "PERMANENT DELETION") {
			t.Fatalf("expected deletion warning in preview, got: %s", tcDestPreview.Text)
		}

		resDestExec, _ := destroyTool.Handler(ctx, mcp.CallToolRequest{
			Params: mcp.CallToolParams{
				Arguments: map[string]any{"node": "pve", "vmid": 101, "confirm": true, "expected_name": "vm101"},
			},
		})
		if resDestExec.IsError {
			t.Fatalf("unexpected error destroying VM: %+v", resDestExec)
		}
	})

	t.Run("lxc_clone_create_and_destroy", func(t *testing.T) {
		cloneTool := serverEnabled.GetTool("pve_lxc_clone")
		resClone, _ := cloneTool.Handler(ctx, mcp.CallToolRequest{
			Params: mcp.CallToolParams{
				Arguments: map[string]any{"node": "pve", "vmid": 200, "newid": 202, "hostname": "cloned-ct", "confirm": true},
			},
		})
		if resClone.IsError {
			t.Fatalf("unexpected error cloning CT: %+v", resClone)
		}

		createTool := serverEnabled.GetTool("pve_lxc_create")
		resCreatePreview, _ := createTool.Handler(ctx, mcp.CallToolRequest{
			Params: mcp.CallToolParams{
				Arguments: map[string]any{"node": "pve", "vmid": 300, "ostemplate": "local:vztmpl/alpine.tar.zst"},
			},
		})
		tcCreatePreview, _ := mcp.AsTextContent(resCreatePreview.Content[0])
		if !strings.Contains(tcCreatePreview.Text, "[DRY RUN PREVIEW - USER CONFIRMATION REQUIRED]") {
			t.Fatalf("expected dry-run preview for create, got: %s", tcCreatePreview.Text)
		}

		resCreate, _ := createTool.Handler(ctx, mcp.CallToolRequest{
			Params: mcp.CallToolParams{
				Arguments: map[string]any{"node": "pve", "vmid": 300, "ostemplate": "local:vztmpl/alpine.tar.zst", "confirm": true},
			},
		})
		if resCreate.IsError {
			t.Fatalf("unexpected error creating CT: %+v", resCreate)
		}

		destroyTool := serverEnabled.GetTool("pve_lxc_destroy")
		resDestroy, _ := destroyTool.Handler(ctx, mcp.CallToolRequest{
			Params: mcp.CallToolParams{
				Arguments: map[string]any{"node": "pve", "vmid": 200, "confirm": true, "expected_name": "ct200"},
			},
		})
		if resDestroy.IsError {
			t.Fatalf("unexpected error destroying CT: %+v", resDestroy)
		}
	})

	t.Run("qemu_and_lxc_protection", func(t *testing.T) {
		qProt := serverEnabled.GetTool("pve_qemu_protection")
		resQPreview, _ := qProt.Handler(ctx, mcp.CallToolRequest{
			Params: mcp.CallToolParams{
				Arguments: map[string]any{"node": "pve", "vmid": 100, "protected": true},
			},
		})
		tcQPreview, _ := mcp.AsTextContent(resQPreview.Content[0])
		if !strings.Contains(tcQPreview.Text, "[DRY RUN PREVIEW - USER CONFIRMATION REQUIRED]") {
			t.Fatalf("expected dry-run preview, got: %s", tcQPreview.Text)
		}

		resQExec, _ := qProt.Handler(ctx, mcp.CallToolRequest{
			Params: mcp.CallToolParams{
				Arguments: map[string]any{"node": "pve", "vmid": 100, "protected": true, "confirm": true},
			},
		})
		if resQExec.IsError {
			t.Fatalf("unexpected error setting qemu protection: %+v", resQExec)
		}

		lProt := serverEnabled.GetTool("pve_lxc_protection")
		resLPreview, _ := lProt.Handler(ctx, mcp.CallToolRequest{
			Params: mcp.CallToolParams{
				Arguments: map[string]any{"node": "pve", "vmid": 200, "protected": false},
			},
		})
		tcLPreview, _ := mcp.AsTextContent(resLPreview.Content[0])
		if !strings.Contains(tcLPreview.Text, "[DRY RUN PREVIEW - USER CONFIRMATION REQUIRED]") {
			t.Fatalf("expected dry-run preview, got: %s", tcLPreview.Text)
		}

		resLExec, _ := lProt.Handler(ctx, mcp.CallToolRequest{
			Params: mcp.CallToolParams{
				Arguments: map[string]any{"node": "pve", "vmid": 200, "protected": false, "confirm": true},
			},
		})
		if resLExec.IsError {
			t.Fatalf("unexpected error setting lxc protection: %+v", resLExec)
		}
	})
}

func TestMutatingTools_ValidationAndEdgeCases(t *testing.T) {
	mockPVE := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api2/json/nodes/pve/qemu/100/status/current":
			w.Write([]byte(`{"data":{"status":"running","name":"vm100"}}`))
		case "/api2/json/nodes/pve/qemu/100/config":
			w.Write([]byte(`{"data":{"cores":2,"memory":2048,"scsi0":"local:vm-100-disk-0,size=32G","protection":1,"net0":"virtio=AA:BB:CC:DD:EE:FF,bridge=vmbr0"}}`))
		case "/api2/json/nodes/pve/lxc/200/status/current":
			w.Write([]byte(`{"data":{"status":"running","name":"ct200"}}`))
		case "/api2/json/nodes/pve/lxc/200/config":
			w.Write([]byte(`{"data":{"cores":2,"memory":1024,"rootfs":"local:subvol-200-disk-0,size=8G","protection":1,"net0":"name=eth0,bridge=vmbr0"}}`))
		case "/api2/json/nodes/pve/qemu/100/clone",
			"/api2/json/nodes/pve/lxc/200/clone",
			"/api2/json/nodes/pve/lxc":
			w.Write([]byte(`{"data":"UPID:pve:0001:0002:66F2:failtask:100:root@pam:"}`))
		case "/api2/json/nodes/pve/tasks/UPID:pve:0001:0002:66F2:failtask:100:root@pam:/status":
			w.Write([]byte(`{"data":{"status":"stopped","exitstatus":"ERROR: out of space"}}`))
		default:
			w.Write([]byte(`{"data":null}`))
		}
	}))
	defer mockPVE.Close()

	cfg := &config.Config{
		Host:           mockPVE.URL,
		TokenID:        "user@pve!token",
		TokenSecret:    "secret",
		AllowMutations: true,
		AllowDestroy:   true,
	}
	client, _ := pve.NewClient(cfg)
	mcpServer := server.NewMCPServer("pve-mcp", "1.0.0")
	tools.RegisterAll(mcpServer, client, cfg)
	ctx := context.Background()

	t.Run("destroy running and protected guests", func(t *testing.T) {
		qDestroy := mcpServer.GetTool("pve_qemu_destroy")
		resRunning, _ := qDestroy.Handler(ctx, mcp.CallToolRequest{
			Params: mcp.CallToolParams{
				Arguments: map[string]any{"node": "pve", "vmid": 100, "confirm": true, "expected_name": "vm100"},
			},
		})
		if !resRunning.IsError || !strings.Contains(resRunning.Content[0].(mcp.TextContent).Text, "running") {
			t.Fatalf("expected running guest error, got: %+v", resRunning)
		}

		lDestroy := mcpServer.GetTool("pve_lxc_destroy")
		resLRunning, _ := lDestroy.Handler(ctx, mcp.CallToolRequest{
			Params: mcp.CallToolParams{
				Arguments: map[string]any{"node": "pve", "vmid": 200, "confirm": true, "expected_name": "ct200"},
			},
		})
		if !resLRunning.IsError || !strings.Contains(resLRunning.Content[0].(mcp.TextContent).Text, "running") {
			t.Fatalf("expected running container error, got: %+v", resLRunning)
		}
	})

	t.Run("destroy disallowed config", func(t *testing.T) {
		noDestroyCfg := &config.Config{
			Host:           mockPVE.URL,
			TokenID:        "user@pve!token",
			TokenSecret:    "secret",
			AllowMutations: true,
			AllowDestroy:   false,
		}
		noDestroyClient, _ := pve.NewClient(noDestroyCfg)
		noDestroyServer := server.NewMCPServer("pve-mcp", "1.0.0")
		tools.RegisterAll(noDestroyServer, noDestroyClient, noDestroyCfg)

		st := noDestroyServer.GetTool("pve_qemu_destroy")
		res, _ := st.Handler(ctx, mcp.CallToolRequest{
			Params: mcp.CallToolParams{
				Arguments: map[string]any{"node": "pve", "vmid": 100, "confirm": true},
			},
		})
		if !res.IsError || !strings.Contains(res.Content[0].(mcp.TextContent).Text, "machine destruction is disabled") {
			t.Fatalf("expected destroy disabled error, got: %+v", res)
		}
	})

	t.Run("resize validation and shrinking rejection", func(t *testing.T) {
		qResize := mcpServer.GetTool("pve_qemu_resize_disk")
		resInvalidDisk, _ := qResize.Handler(ctx, mcp.CallToolRequest{
			Params: mcp.CallToolParams{
				Arguments: map[string]any{"node": "pve", "vmid": 100, "disk": "bad_disk", "size": "64G", "confirm": true},
			},
		})
		if !resInvalidDisk.IsError {
			t.Fatalf("expected error on invalid disk name")
		}

		resShrink, _ := qResize.Handler(ctx, mcp.CallToolRequest{
			Params: mcp.CallToolParams{
				Arguments: map[string]any{"node": "pve", "vmid": 100, "disk": "scsi0", "size": "16G", "confirm": true},
			},
		})
		if !resShrink.IsError || !strings.Contains(resShrink.Content[0].(mcp.TextContent).Text, "shrinking virtual disks is forbidden") {
			t.Fatalf("expected shrink rejection error, got: %+v", resShrink)
		}

		lResize := mcpServer.GetTool("pve_lxc_resize_disk")
		resLShrink, _ := lResize.Handler(ctx, mcp.CallToolRequest{
			Params: mcp.CallToolParams{
				Arguments: map[string]any{"node": "pve", "vmid": 200, "disk": "rootfs", "size": "4G", "confirm": true},
			},
		})
		if !resLShrink.IsError || !strings.Contains(resLShrink.Content[0].(mcp.TextContent).Text, "shrinking virtual disks is forbidden") {
			t.Fatalf("expected lxc shrink rejection error, got: %+v", resLShrink)
		}
	})

	t.Run("hardware update with no changes proposed", func(t *testing.T) {
		qHw := mcpServer.GetTool("pve_qemu_update_hardware")
		resNoChange, _ := qHw.Handler(ctx, mcp.CallToolRequest{
			Params: mcp.CallToolParams{
				Arguments: map[string]any{"node": "pve", "vmid": 100, "confirm": true},
			},
		})
		if !resNoChange.IsError || !strings.Contains(resNoChange.Content[0].(mcp.TextContent).Text, "at least one hardware attribute") {
			t.Fatalf("expected no changes proposed error, got: %+v", resNoChange)
		}

		lHw := mcpServer.GetTool("pve_lxc_update_hardware")
		resLNoChange, _ := lHw.Handler(ctx, mcp.CallToolRequest{
			Params: mcp.CallToolParams{
				Arguments: map[string]any{"node": "pve", "vmid": 200, "confirm": true},
			},
		})
		if !resLNoChange.IsError || !strings.Contains(resLNoChange.Content[0].(mcp.TextContent).Text, "at least one hardware attribute") {
			t.Fatalf("expected no changes proposed error for lxc, got: %+v", resLNoChange)
		}
	})

	t.Run("clone and create with wait false and task error", func(t *testing.T) {
		qClone := mcpServer.GetTool("pve_qemu_clone")
		resNoWait, _ := qClone.Handler(ctx, mcp.CallToolRequest{
			Params: mcp.CallToolParams{
				Arguments: map[string]any{"node": "pve", "vmid": 100, "newid": 102, "confirm": true, "wait": false},
			},
		})
		if resNoWait.IsError || !strings.Contains(resNoWait.Content[0].(mcp.TextContent).Text, "Clone task initiated successfully") {
			t.Fatalf("expected wait=false success, got: %+v", resNoWait)
		}

		resTaskFail, _ := qClone.Handler(ctx, mcp.CallToolRequest{
			Params: mcp.CallToolParams{
				Arguments: map[string]any{"node": "pve", "vmid": 100, "newid": 102, "confirm": true, "wait": true},
			},
		})
		if !resTaskFail.IsError || !strings.Contains(resTaskFail.Content[0].(mcp.TextContent).Text, "Clone task failed") {
			t.Fatalf("expected task failure error, got: %+v", resTaskFail)
		}

		lCreate := mcpServer.GetTool("pve_lxc_create")
		resCreateNoWait, _ := lCreate.Handler(ctx, mcp.CallToolRequest{
			Params: mcp.CallToolParams{
				Arguments: map[string]any{"node": "pve", "vmid": 300, "ostemplate": "local:vztmpl/alpine.tar.zst", "confirm": true, "wait": false},
			},
		})
		if resCreateNoWait.IsError || !strings.Contains(resCreateNoWait.Content[0].(mcp.TextContent).Text, "Create task initiated successfully") {
			t.Fatalf("expected create wait=false success, got: %+v", resCreateNoWait)
		}
	})
}

func TestMutatingTools_ValidationParamPermutationsAndFailures(t *testing.T) {
	failServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "upstream failure", http.StatusInternalServerError)
	}))
	defer failServer.Close()

	cfg := &config.Config{
		Host:           failServer.URL,
		TokenID:        "user@pve!token",
		TokenSecret:    "secret",
		AllowMutations: true,
		AllowDestroy:   true,
	}
	client, _ := pve.NewClient(cfg)
	mcpServer := server.NewMCPServer("pve-mcp", "1.0.0")
	tools.RegisterAll(mcpServer, client, cfg)
	ctx := context.Background()

	toolsToTest := []struct {
		name      string
		validArgs map[string]any
	}{
		{name: "pve_qemu_power", validArgs: map[string]any{"node": "pve", "vmid": 100, "action": "start", "confirm": true}},
		{name: "pve_qemu_update_hardware", validArgs: map[string]any{"node": "pve", "vmid": 100, "cores": 4, "confirm": true}},
		{name: "pve_qemu_resize_disk", validArgs: map[string]any{"node": "pve", "vmid": 100, "disk": "scsi0", "size": "64G", "confirm": true}},
		{name: "pve_qemu_update_network", validArgs: map[string]any{"node": "pve", "vmid": 100, "net_id": "net0", "bridge": "vmbr1", "confirm": true}},
		{name: "pve_qemu_clone", validArgs: map[string]any{"node": "pve", "vmid": 100, "newid": 102, "confirm": true}},
		{name: "pve_qemu_destroy", validArgs: map[string]any{"node": "pve", "vmid": 100, "confirm": true}},
		{name: "pve_qemu_protection", validArgs: map[string]any{"node": "pve", "vmid": 100, "protected": true, "confirm": true}},
		{name: "pve_lxc_power", validArgs: map[string]any{"node": "pve", "vmid": 200, "action": "start", "confirm": true}},
		{name: "pve_lxc_update_hardware", validArgs: map[string]any{"node": "pve", "vmid": 200, "cores": 4, "confirm": true}},
		{name: "pve_lxc_resize_disk", validArgs: map[string]any{"node": "pve", "vmid": 200, "disk": "rootfs", "size": "16G", "confirm": true}},
		{name: "pve_lxc_update_network", validArgs: map[string]any{"node": "pve", "vmid": 200, "net_id": "net0", "bridge": "vmbr1", "confirm": true}},
		{name: "pve_lxc_clone", validArgs: map[string]any{"node": "pve", "vmid": 200, "newid": 202, "confirm": true}},
		{name: "pve_lxc_create", validArgs: map[string]any{"node": "pve", "vmid": 300, "ostemplate": "local:vztmpl/alpine.tar.zst", "confirm": true}},
		{name: "pve_lxc_destroy", validArgs: map[string]any{"node": "pve", "vmid": 200, "confirm": true}},
		{name: "pve_lxc_protection", validArgs: map[string]any{"node": "pve", "vmid": 200, "protected": true, "confirm": true}},
	}

	for _, tt := range toolsToTest {
		st := mcpServer.GetTool(tt.name)
		if st == nil {
			t.Fatalf("tool %s not registered", tt.name)
		}

		resMissingNode, _ := st.Handler(ctx, mcp.CallToolRequest{
			Params: mcp.CallToolParams{
				Arguments: map[string]any{"vmid": 100},
			},
		})
		if !resMissingNode.IsError {
			t.Fatalf("tool %s expected error on missing node", tt.name)
		}

		resInvalidNode, _ := st.Handler(ctx, mcp.CallToolRequest{
			Params: mcp.CallToolParams{
				Arguments: map[string]any{"node": "bad~node", "vmid": 100},
			},
		})
		if !resInvalidNode.IsError {
			t.Fatalf("tool %s expected error on invalid node", tt.name)
		}

		resMissingVMID, _ := st.Handler(ctx, mcp.CallToolRequest{
			Params: mcp.CallToolParams{
				Arguments: map[string]any{"node": "pve"},
			},
		})
		if !resMissingVMID.IsError {
			t.Fatalf("tool %s expected error on missing vmid", tt.name)
		}

		resInvalidVMID, _ := st.Handler(ctx, mcp.CallToolRequest{
			Params: mcp.CallToolParams{
				Arguments: map[string]any{"node": "pve", "vmid": 50},
			},
		})
		if !resInvalidVMID.IsError {
			t.Fatalf("tool %s expected error on invalid vmid", tt.name)
		}

		resPVEFail, _ := st.Handler(ctx, mcp.CallToolRequest{
			Params: mcp.CallToolParams{
				Arguments: tt.validArgs,
			},
		})
		if !resPVEFail.IsError {
			t.Fatalf("tool %s expected error on upstream PVE failure", tt.name)
		}
	}
}

func TestMutatingTools_FullOptionCoverage(t *testing.T) {
	mockPVE := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api2/json/nodes/pve/qemu/100/status/current":
			w.Write([]byte(`{"data":{"status":"running","name":"vm100"}}`))
		case "/api2/json/nodes/pve/lxc/200/status/current":
			w.Write([]byte(`{"data":{"status":"running","name":"ct200"}}`))
		case "/api2/json/nodes/pve/qemu/100/config":
			if r.Method == http.MethodGet {
				w.Write([]byte(`{"data":{"cores":2,"sockets":1,"memory":2048,"scsi0":"local-lvm:vm-100-disk-0,size=32G","net0":"virtio=AA:BB:CC:DD:EE:FF,bridge=vmbr0,firewall=1"}}`))
			} else {
				w.Write([]byte(`{"data":null}`))
			}
		case "/api2/json/nodes/pve/qemu/100/status/shutdown",
			"/api2/json/nodes/pve/qemu/100/status/reboot",
			"/api2/json/nodes/pve/qemu/100/status/suspend",
			"/api2/json/nodes/pve/qemu/100/status/resume",
			"/api2/json/nodes/pve/qemu/100/resize",
			"/api2/json/nodes/pve/lxc/200/status/shutdown",
			"/api2/json/nodes/pve/lxc/200/status/stop",
			"/api2/json/nodes/pve/lxc/200/status/reboot",
			"/api2/json/nodes/pve/lxc/200/status/suspend",
			"/api2/json/nodes/pve/lxc/200/status/resume",
			"/api2/json/nodes/pve/lxc/200/resize",
			"/api2/json/nodes/pve/qemu/100/clone",
			"/api2/json/nodes/pve/lxc/200/clone",
			"/api2/json/nodes/pve/lxc":
			w.Write([]byte(`{"data":"UPID:pve:0001:0002:66F2:action:100:root@pam:"}`))
		case "/api2/json/nodes/pve/lxc/200/config":
			if r.Method == http.MethodGet {
				w.Write([]byte(`{"data":{"cores":2,"memory":1024,"swap":512,"rootfs":"local-zfs:subvol-200-disk-0,size=8G","net0":"name=eth0,bridge=vmbr0,ip=dhcp"}}`))
			} else {
				w.Write([]byte(`{"data":null}`))
			}
		case "/api2/json/nodes/pve/tasks/UPID:pve:0001:0002:66F2:action:100:root@pam:/status":
			w.Write([]byte(`{"data":{"status":"stopped","exitstatus":"OK"}}`))
		default:
			w.Write([]byte(`{"data":null}`))
		}
	}))
	defer mockPVE.Close()

	cfg := &config.Config{
		Host:           mockPVE.URL,
		TokenID:        "user@pve!token",
		TokenSecret:    "secret",
		AllowMutations: true,
		AllowDestroy:   true,
	}
	client, _ := pve.NewClient(cfg)
	mcpServer := server.NewMCPServer("pve-mcp", "1.0.0")
	tools.RegisterAll(mcpServer, client, cfg)
	ctx := context.Background()

	t.Run("qemu_power_all_actions", func(t *testing.T) {
		st := mcpServer.GetTool("pve_qemu_power")
		actions := []string{"shutdown", "reboot", "suspend", "resume"}
		for _, action := range actions {
			res, err := st.Handler(ctx, mcp.CallToolRequest{
				Params: mcp.CallToolParams{
					Arguments: map[string]any{"node": "pve", "vmid": 100, "action": action, "timeout": 30, "confirm": true},
				},
			})
			if err != nil || res.IsError {
				t.Fatalf("action %s failed: err=%v, res=%+v", action, err, res)
			}
		}

		resInvalidAction, _ := st.Handler(ctx, mcp.CallToolRequest{
			Params: mcp.CallToolParams{
				Arguments: map[string]any{"node": "pve", "vmid": 100, "action": "destroy", "confirm": true},
			},
		})
		if !resInvalidAction.IsError {
			t.Fatalf("expected error on invalid power action")
		}
	})

	t.Run("qemu_hardware_full_attributes", func(t *testing.T) {
		st := mcpServer.GetTool("pve_qemu_update_hardware")
		resPreview, _ := st.Handler(ctx, mcp.CallToolRequest{
			Params: mcp.CallToolParams{
				Arguments: map[string]any{
					"node": "pve", "vmid": 100,
					"cores": 4, "sockets": 2, "cpu": "host", "memory": 4096, "balloon": 2048,
				},
			},
		})
		if resPreview.IsError {
			t.Fatalf("expected preview success: %+v", resPreview)
		}

		resExec, _ := st.Handler(ctx, mcp.CallToolRequest{
			Params: mcp.CallToolParams{
				Arguments: map[string]any{
					"node": "pve", "vmid": 100,
					"cores": 4, "sockets": 2, "cpu": "host", "memory": 4096, "balloon": 2048,
					"confirm": true,
				},
			},
		})
		if resExec.IsError {
			t.Fatalf("expected exec success: %+v", resExec)
		}
	})

	t.Run("qemu_resize_and_network_previews", func(t *testing.T) {
		resizeTool := mcpServer.GetTool("pve_qemu_resize_disk")
		resResizePreview, _ := resizeTool.Handler(ctx, mcp.CallToolRequest{
			Params: mcp.CallToolParams{
				Arguments: map[string]any{"node": "pve", "vmid": 100, "disk": "scsi0", "size": "64G"},
			},
		})
		if resResizePreview.IsError {
			t.Fatalf("expected resize preview success: %+v", resResizePreview)
		}

		resMissingSize, _ := resizeTool.Handler(ctx, mcp.CallToolRequest{
			Params: mcp.CallToolParams{
				Arguments: map[string]any{"node": "pve", "vmid": 100, "disk": "scsi0"},
			},
		})
		if !resMissingSize.IsError {
			t.Fatalf("expected error on missing size")
		}

		resDiskNotInConfig, _ := resizeTool.Handler(ctx, mcp.CallToolRequest{
			Params: mcp.CallToolParams{
				Arguments: map[string]any{"node": "pve", "vmid": 100, "disk": "scsi1", "size": "64G", "confirm": true},
			},
		})
		if !resDiskNotInConfig.IsError {
			t.Fatalf("expected error for disk not in config")
		}

		netTool := mcpServer.GetTool("pve_qemu_update_network")
		resNetPreview, _ := netTool.Handler(ctx, mcp.CallToolRequest{
			Params: mcp.CallToolParams{
				Arguments: map[string]any{"node": "pve", "vmid": 100, "net_id": "net0", "bridge": "vmbr1", "tag": 100, "firewall": true},
			},
		})
		if resNetPreview.IsError {
			t.Fatalf("expected network preview success: %+v", resNetPreview)
		}

		resNetMissingBridge, _ := netTool.Handler(ctx, mcp.CallToolRequest{
			Params: mcp.CallToolParams{
				Arguments: map[string]any{"node": "pve", "vmid": 100, "net_id": "net99"},
			},
		})
		if !resNetMissingBridge.IsError {
			t.Fatalf("expected error on missing bridge")
		}

		resNetInvalidNetID, _ := netTool.Handler(ctx, mcp.CallToolRequest{
			Params: mcp.CallToolParams{
				Arguments: map[string]any{"node": "pve", "vmid": 100, "net_id": "eth0", "bridge": "vmbr0"},
			},
		})
		if !resNetInvalidNetID.IsError {
			t.Fatalf("expected error on invalid netID")
		}
	})

	t.Run("qemu_clone_options", func(t *testing.T) {
		cloneTool := mcpServer.GetTool("pve_qemu_clone")
		resMissingNewID, _ := cloneTool.Handler(ctx, mcp.CallToolRequest{
			Params: mcp.CallToolParams{
				Arguments: map[string]any{"node": "pve", "vmid": 100, "confirm": true},
			},
		})
		if !resMissingNewID.IsError {
			t.Fatalf("expected error on missing newid")
		}

		resWithOptions, _ := cloneTool.Handler(ctx, mcp.CallToolRequest{
			Params: mcp.CallToolParams{
				Arguments: map[string]any{
					"node": "pve", "vmid": 100, "newid": 105, "name": "cloned",
					"full": true, "storage": "local-lvm", "target": "pve2", "format": "qcow2",
					"confirm": true,
				},
			},
		})
		if resWithOptions.IsError {
			t.Fatalf("expected clone with options success: %+v", resWithOptions)
		}
	})

	t.Run("lxc_power_hardware_and_network", func(t *testing.T) {
		powerTool := mcpServer.GetTool("pve_lxc_power")
		resStopForce, _ := powerTool.Handler(ctx, mcp.CallToolRequest{
			Params: mcp.CallToolParams{
				Arguments: map[string]any{"node": "pve", "vmid": 200, "action": "stop", "force": true, "confirm": true, "expected_name": "ct200"},
			},
		})
		if resStopForce.IsError {
			t.Fatalf("expected force stop success: %+v", resStopForce)
		}

		actions := []string{"shutdown", "reboot", "suspend", "resume"}
		for _, action := range actions {
			res, err := powerTool.Handler(ctx, mcp.CallToolRequest{
				Params: mcp.CallToolParams{
					Arguments: map[string]any{"node": "pve", "vmid": 200, "action": action, "timeout": 15, "confirm": true},
				},
			})
			if err != nil || res.IsError {
				t.Fatalf("lxc power action %s failed: err=%v, res=%+v", action, err, res)
			}
		}

		resStopNoForce, _ := powerTool.Handler(ctx, mcp.CallToolRequest{
			Params: mcp.CallToolParams{
				Arguments: map[string]any{"node": "pve", "vmid": 200, "action": "stop", "confirm": true},
			},
		})
		if !resStopNoForce.IsError {
			t.Fatalf("expected error on stop without force")
		}

		hwTool := mcpServer.GetTool("pve_lxc_update_hardware")
		resHwPreview, _ := hwTool.Handler(ctx, mcp.CallToolRequest{
			Params: mcp.CallToolParams{
				Arguments: map[string]any{"node": "pve", "vmid": 200, "cores": 4, "memory": 2048, "swap": 1024},
			},
		})
		if resHwPreview.IsError {
			t.Fatalf("expected lxc hw preview success: %+v", resHwPreview)
		}

		resHwExec, _ := hwTool.Handler(ctx, mcp.CallToolRequest{
			Params: mcp.CallToolParams{
				Arguments: map[string]any{"node": "pve", "vmid": 200, "cores": 4, "memory": 2048, "swap": 1024, "confirm": true},
			},
		})
		if resHwExec.IsError {
			t.Fatalf("expected lxc hw exec success: %+v", resHwExec)
		}

		resizeTool := mcpServer.GetTool("pve_lxc_resize_disk")
		resResizePreview, _ := resizeTool.Handler(ctx, mcp.CallToolRequest{
			Params: mcp.CallToolParams{
				Arguments: map[string]any{"node": "pve", "vmid": 200, "disk": "rootfs", "size": "16G"},
			},
		})
		if resResizePreview.IsError {
			t.Fatalf("expected lxc resize preview success: %+v", resResizePreview)
		}

		resInvalidLXCDisk, _ := resizeTool.Handler(ctx, mcp.CallToolRequest{
			Params: mcp.CallToolParams{
				Arguments: map[string]any{"node": "pve", "vmid": 200, "disk": "scsi0", "size": "16G", "confirm": true},
			},
		})
		if !resInvalidLXCDisk.IsError {
			t.Fatalf("expected error on invalid lxc disk format")
		}

		netTool := mcpServer.GetTool("pve_lxc_update_network")
		resNetPreview, _ := netTool.Handler(ctx, mcp.CallToolRequest{
			Params: mcp.CallToolParams{
				Arguments: map[string]any{
					"node": "pve", "vmid": 200, "net_id": "net0", "bridge": "vmbr2",
					"ip": "192.168.1.50/24", "gw": "192.168.1.1", "tag": 20,
				},
			},
		})
		if resNetPreview.IsError {
			t.Fatalf("expected lxc net preview success: %+v", resNetPreview)
		}

		resNetExec, _ := netTool.Handler(ctx, mcp.CallToolRequest{
			Params: mcp.CallToolParams{
				Arguments: map[string]any{
					"node": "pve", "vmid": 200, "net_id": "net0", "bridge": "vmbr2",
					"ip": "192.168.1.50/24", "gw": "192.168.1.1", "tag": 20, "confirm": true,
				},
			},
		})
		if resNetExec.IsError {
			t.Fatalf("expected lxc net exec success: %+v", resNetExec)
		}

		resNetMissingBridge, _ := netTool.Handler(ctx, mcp.CallToolRequest{
			Params: mcp.CallToolParams{
				Arguments: map[string]any{"node": "pve", "vmid": 200, "net_id": "net99"},
			},
		})
		if !resNetMissingBridge.IsError {
			t.Fatalf("expected error on missing bridge")
		}
	})

	t.Run("lxc_clone_and_create_options", func(t *testing.T) {
		cloneTool := mcpServer.GetTool("pve_lxc_clone")
		resMissingNewID, _ := cloneTool.Handler(ctx, mcp.CallToolRequest{
			Params: mcp.CallToolParams{
				Arguments: map[string]any{"node": "pve", "vmid": 200, "confirm": true},
			},
		})
		if !resMissingNewID.IsError {
			t.Fatalf("expected error on missing newid")
		}

		resWithOptions, _ := cloneTool.Handler(ctx, mcp.CallToolRequest{
			Params: mcp.CallToolParams{
				Arguments: map[string]any{
					"node": "pve", "vmid": 200, "newid": 205, "hostname": "cloned-ct",
					"full": true, "storage": "local-zfs", "target": "pve2",
					"confirm": true,
				},
			},
		})
		if resWithOptions.IsError {
			t.Fatalf("expected clone with options success: %+v", resWithOptions)
		}

		createTool := mcpServer.GetTool("pve_lxc_create")
		resMissingTemplate, _ := createTool.Handler(ctx, mcp.CallToolRequest{
			Params: mcp.CallToolParams{
				Arguments: map[string]any{"node": "pve", "vmid": 300, "confirm": true},
			},
		})
		if !resMissingTemplate.IsError {
			t.Fatalf("expected error on missing ostemplate")
		}

		resCreateFull, _ := createTool.Handler(ctx, mcp.CallToolRequest{
			Params: mcp.CallToolParams{
				Arguments: map[string]any{
					"node": "pve", "vmid": 305, "ostemplate": "local:vztmpl/ubuntu.tar.zst",
					"hostname": "ubuntu-ct", "cores": 2, "memory": 1024, "disk": 16,
					"storage": "local-zfs", "bridge": "vmbr0", "ip": "dhcp",
					"unprivileged": true, "password": "pass", "start": true,
					"confirm": true,
				},
			},
		})
		if resCreateFull.IsError {
			t.Fatalf("expected full create success: %+v", resCreateFull)
		}
	})
}

func TestMutatingTools_ExhaustiveBranchCoverage(t *testing.T) {
	mockPVE := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if strings.Contains(r.URL.Path, "/tasks/") {
			w.Write([]byte(`{"data":{"status":"stopped","exitstatus":"OK"}}`))
			return
		}
		if strings.Contains(r.URL.Path, "/status/current") {
			w.Write([]byte(`{"data":{"status":"stopped","protection":false,"name":"test-guest"}}`))
			return
		}
		if strings.Contains(r.URL.Path, "/config") {
			if r.Method == http.MethodGet {
				w.Write([]byte(`{"data":{"cores":2,"memory":2048,"scsi0":"local-lvm:vm-100-disk-0,size=32G","rootfs":"local-zfs:subvol-200-disk-0,size=8G","net0":"virtio=AA:BB:CC:DD:EE:FF,bridge=vmbr0"}}`))
				return
			}
			w.Write([]byte(`{"data":null}`))
			return
		}
		if strings.Contains(r.URL.Path, "/clone") || strings.Contains(r.URL.Path, "/resize") || strings.Contains(r.URL.Path, "/status/") {
			w.Write([]byte(`{"data":"UPID:pve:12345"}`))
			return
		}
		if r.Method == http.MethodDelete || r.Method == http.MethodPost || r.Method == http.MethodPut {
			w.Write([]byte(`{"data":"UPID:pve:12345"}`))
			return
		}
		w.Write([]byte(`{"data":[]}`))
	}))
	defer mockPVE.Close()

	ctx := context.Background()
	cfg := &config.Config{
		Host:           mockPVE.URL,
		TokenID:        "root@pam!token",
		TokenSecret:    "secret",
		AllowMutations: true,
		AllowDestroy:   true,
	}
	client, _ := pve.NewClient(cfg)
	mcpServer := server.NewMCPServer("pve-mcp", "1.0.0")
	tools.RegisterAll(mcpServer, client, cfg)

	mockErrServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "internal error", http.StatusInternalServerError)
	}))
	defer mockErrServer.Close()
	cfgErr := &config.Config{
		Host:           mockErrServer.URL,
		TokenID:        "root@pam!token",
		TokenSecret:    "secret",
		AllowMutations: true,
		AllowDestroy:   true,
	}
	clientErr, _ := pve.NewClient(cfgErr)
	mcpServerErr := server.NewMCPServer("pve-mcp-err", "1.0.0")
	tools.RegisterAll(mcpServerErr, clientErr, cfgErr)

	taskTool := mcpServer.GetTool("pve_task_status")
	resBadNode, _ := taskTool.Handler(ctx, mcp.CallToolRequest{
		Params: mcp.CallToolParams{Arguments: map[string]any{"node": "../bad", "upid": "UPID:pve:1"}},
	})
	if !resBadNode.IsError {
		t.Fatal("expected error on bad node in task status")
	}

	taskToolErr := mcpServerErr.GetTool("pve_task_status")
	resTaskErr, _ := taskToolErr.Handler(ctx, mcp.CallToolRequest{
		Params: mcp.CallToolParams{Arguments: map[string]any{"node": "pve", "upid": "UPID:pve:1"}},
	})
	if !resTaskErr.IsError {
		t.Fatal("expected error from API failure in task status")
	}

	toolsWithNodeAndVMID := []string{
		"pve_qemu_power", "pve_qemu_update_hardware", "pve_qemu_resize_disk", "pve_qemu_update_network",
		"pve_qemu_clone", "pve_qemu_destroy", "pve_qemu_protection",
		"pve_lxc_power", "pve_lxc_update_hardware", "pve_lxc_resize_disk", "pve_lxc_update_network",
		"pve_lxc_clone", "pve_lxc_create", "pve_lxc_destroy", "pve_lxc_protection",
	}

	for _, name := range toolsWithNodeAndVMID {
		tool := mcpServer.GetTool(name)
		resNode, _ := tool.Handler(ctx, mcp.CallToolRequest{
			Params: mcp.CallToolParams{Arguments: map[string]any{"node": "../bad", "vmid": 100}},
		})
		if !resNode.IsError {
			t.Fatalf("expected node error for %s", name)
		}

		resVMID, _ := tool.Handler(ctx, mcp.CallToolRequest{
			Params: mcp.CallToolParams{Arguments: map[string]any{"node": "pve", "vmid": -1}},
		})
		if !resVMID.IsError {
			t.Fatalf("expected vmid error for %s", name)
		}
	}

	qemuHwTool := mcpServer.GetTool("pve_qemu_update_hardware")
	resQemuHwFull, _ := qemuHwTool.Handler(ctx, mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Arguments: map[string]any{
				"node": "pve", "vmid": 100, "cores": 4, "memory_mb": 4096,
				"balloon_mb": 2048, "name": "vm-renamed", "description": "new-desc",
				"onboot": true, "confirm": true,
			},
		},
	})
	if resQemuHwFull.IsError {
		t.Fatalf("expected qemu hw full success: %+v", resQemuHwFull)
	}

	lxcHwTool := mcpServer.GetTool("pve_lxc_update_hardware")
	resLxcHwFull, _ := lxcHwTool.Handler(ctx, mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Arguments: map[string]any{
				"node": "pve", "vmid": 200, "cores": 4, "memory_mb": 2048,
				"swap_mb": 512, "description": "new-ct-desc", "onboot": true,
				"confirm": true,
			},
		},
	})
	if resLxcHwFull.IsError {
		t.Fatalf("expected lxc hw full success: %+v", resLxcHwFull)
	}

	qemuNetTool := mcpServer.GetTool("pve_qemu_update_network")
	resQemuNetNew, _ := qemuNetTool.Handler(ctx, mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Arguments: map[string]any{
				"node": "pve", "vmid": 100, "net_id": "net1", "bridge": "vmbr0",
				"tag": 100, "firewall": true, "rate": 10, "confirm": true,
			},
		},
	})
	if resQemuNetNew.IsError {
		t.Fatalf("expected qemu new net success: %+v", resQemuNetNew)
	}

	lxcNetTool := mcpServer.GetTool("pve_lxc_update_network")
	resLxcNetNew, _ := lxcNetTool.Handler(ctx, mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Arguments: map[string]any{
				"node": "pve", "vmid": 200, "net_id": "net1", "bridge": "vmbr0",
				"tag": 100, "firewall": true, "rate": 10, "confirm": true,
			},
		},
	})
	if resLxcNetNew.IsError {
		t.Fatalf("expected lxc new net success: %+v", resLxcNetNew)
	}

	qemuCloneTool := mcpServer.GetTool("pve_qemu_clone")
	resQemuCloneNoWait, _ := qemuCloneTool.Handler(ctx, mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Arguments: map[string]any{
				"node": "pve", "vmid": 100, "newid": 110, "wait": false, "confirm": true,
			},
		},
	})
	if resQemuCloneNoWait.IsError {
		t.Fatalf("expected qemu clone no-wait success: %+v", resQemuCloneNoWait)
	}

	lxcCloneTool := mcpServer.GetTool("pve_lxc_clone")
	resLxcCloneNoWait, _ := lxcCloneTool.Handler(ctx, mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Arguments: map[string]any{
				"node": "pve", "vmid": 200, "newid": 210, "wait": false, "confirm": true,
			},
		},
	})
	if resLxcCloneNoWait.IsError {
		t.Fatalf("expected lxc clone no-wait success: %+v", resLxcCloneNoWait)
	}

	lxcCreateTool := mcpServer.GetTool("pve_lxc_create")
	resLxcCreateNoWait, _ := lxcCreateTool.Handler(ctx, mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Arguments: map[string]any{
				"node": "pve", "vmid": 310, "ostemplate": "local:vztmpl/alpine.tar.zst",
				"wait": false, "confirm": true,
			},
		},
	})
	if resLxcCreateNoWait.IsError {
		t.Fatalf("expected lxc create no-wait success: %+v", resLxcCreateNoWait)
	}

	qemuDestroyTool := mcpServer.GetTool("pve_qemu_destroy")
	resQemuDestroyFull, _ := qemuDestroyTool.Handler(ctx, mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Arguments: map[string]any{
				"node": "pve", "vmid": 100, "purge": true,
				"destroy_unreferenced_disks": true, "confirm": true,
				"expected_name": "test-guest",
			},
		},
	})
	if resQemuDestroyFull.IsError {
		t.Fatalf("expected qemu destroy full success: %+v", resQemuDestroyFull)
	}

	lxcDestroyTool := mcpServer.GetTool("pve_lxc_destroy")
	resLxcDestroyFull, _ := lxcDestroyTool.Handler(ctx, mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Arguments: map[string]any{
				"node": "pve", "vmid": 200, "purge": true,
				"destroy_unreferenced_disks": true, "confirm": true,
				"expected_name": "test-guest",
			},
		},
	})
	if resLxcDestroyFull.IsError {
		t.Fatalf("expected lxc destroy full success: %+v", resLxcDestroyFull)
	}

	for _, name := range toolsWithNodeAndVMID {
		toolErr := mcpServerErr.GetTool(name)
		args := map[string]any{"node": "pve", "vmid": 100, "confirm": true, "force": true}
		switch name {
		case "pve_qemu_power", "pve_lxc_power":
			args["action"] = "start"
		case "pve_qemu_resize_disk", "pve_lxc_resize_disk":
			args["disk"] = "scsi0"
			if name == "pve_lxc_resize_disk" {
				args["disk"] = "rootfs"
			}
			args["size"] = "+5G"
		case "pve_qemu_update_network", "pve_lxc_update_network":
			args["net_id"] = "net0"
			args["bridge"] = "vmbr0"
		case "pve_qemu_clone", "pve_lxc_clone":
			args["newid"] = 199
		case "pve_lxc_create":
			args["ostemplate"] = "local:vztmpl/debian.tar.zst"
		case "pve_qemu_protection", "pve_lxc_protection":
			args["enable"] = true
		}

		resErr, _ := toolErr.Handler(ctx, mcp.CallToolRequest{
			Params: mcp.CallToolParams{Arguments: args},
		})
		if !resErr.IsError {
			t.Fatalf("expected API error for %s on mockErrServer", name)
		}
	}
}
