# pve-mcp

[![Go Version](https://img.shields.io/badge/go-1.26%2B-blue.svg)](https://golang.org)
[![MCP Protocol](https://img.shields.io/badge/MCP-Resources%20%7C%20Tools%20%7C%20Prompts-blueviolet.svg)](#mcp-protocol-architecture)
[![Scope](https://img.shields.io/badge/scope-guarded_mutations-green.svg)](#security--architecture)
[![Coverage](https://img.shields.io/badge/coverage-93.0%25-success.svg)](#test-coverage--verification)
[![License](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

A lightweight, security-hardened Model Context Protocol (MCP) server for Proxmox Virtual Environment (PVE). Designed strictly per official MCP specifications: **Resources are nouns (context/state)**, **Tools are verbs (actions/mutations)**, and **Prompts are guided operational workflows**.

---

## MCP Protocol Architecture

```
                    ┌─────────────────────────────────────────────────────┐
                    │                      pve-mcp                        │
                    ├─────────────────┬─────────────────┬─────────────────┤
                    │  MCP Resources  │    MCP Tools    │   MCP Prompts   │
                    │   (Read Nouns)  │ (Action Verbs)  │   (Workflows)   │
                    ├─────────────────┼─────────────────┼─────────────────┤
                    │ 28+ URI Schemes │ 16 Active Tools │ 5 Operational   │
                    │ pve://cluster/* │ 2-Phase Confirm │ Health Audit    │
                    │ pve://nodes/*   │ UPID Tracking   │ Incident Triage │
                    │ pve://storage/* │ Progress Tokens │ Node Evacuation │
                    │ pve://access/*  │ Force Guards    │ Workload Plan   │
                    └─────────────────┴─────────────────┴─────────────────┘
```

---

## 1. MCP Resources & Resource Templates (`pve://...`)

All cluster state, telemetry, guest configurations, storage pools, and access controls are exposed as standard MCP Resources.

### Cluster & Node Resources
| URI | Description |
| :--- | :--- |
| `pve://cluster/status` | Cluster membership, quorum status, and node summary |
| `pve://cluster/resources` | Unified inventory of nodes, VMs, CTs, and storage pools |
| `pve://cluster/ha-status` | High Availability manager state, services, and failovers |
| `pve://cluster/nextid` | Next available free VMID / CTID |
| `pve://cluster/log` | Recent cluster-wide audit and operational log entries |
| `pve://nodes` | Cluster node list and operational status |
| `pve://nodes/{node}/status` | CPU, RAM, kernel, load metrics, and uptime for `{node}` |
| `pve://nodes/{node}/version` | PVE package release and repository version info |
| `pve://nodes/{node}/syslog` | Systemd journal entries for `{node}` |
| `pve://nodes/{node}/network` | Network interfaces, Linux bridges, and bonds |
| `pve://nodes/{node}/tasks/{+upid}` | Status and exit code of background task `{+upid}` |

### Virtual Machine Resources (QEMU)
| URI Template | Description |
| :--- | :--- |
| `pve://nodes/{node}/qemu` | VM inventory on `{node}` |
| `pve://nodes/{node}/qemu/{vmid}/status` | Real-time execution status and resource metrics |
| `pve://nodes/{node}/qemu/{vmid}/config` | Complete VM hardware configuration (cores, memory, disks, NICs) |
| `pve://nodes/{node}/qemu/{vmid}/snapshots` | Snapshot tree and parent-child hierarchy |
| `pve://nodes/{node}/qemu/{vmid}/firewall` | Firewall rules and security options |

### Container Resources (LXC)
| URI Template | Description |
| :--- | :--- |
| `pve://nodes/{node}/lxc` | Container inventory on `{node}` |
| `pve://nodes/{node}/lxc/{vmid}/status` | Real-time status and resource usage |
| `pve://nodes/{node}/lxc/{vmid}/config` | Container configuration (cores, memory, mount points, net) |
| `pve://nodes/{node}/lxc/{vmid}/snapshots` | Snapshot hierarchy for container `{vmid}` |
| `pve://nodes/{node}/lxc/{vmid}/firewall` | Firewall rules and options |

### Storage & Access Resources
| URI / Template | Description |
| :--- | :--- |
| `pve://storage` | Storage definitions and backend driver types |
| `pve://nodes/{node}/storage/{storage}/status` | Capacity, used bytes, and pool status |
| `pve://nodes/{node}/storage/{storage}/content` | Storage volumes, ISOs, templates, and backups |
| `pve://access/users` | Proxmox user accounts and API token metadata |
| `pve://access/groups` | User access groups |
| `pve://access/roles` | Role definitions and privilege bundles |
| `pve://access/domains` | Authentication realms (pam, pve, LDAP, OIDC) |
| `pve://access/permissions` | Effective ACL tree |

---

## 2. MCP Tools (18 Active Verbs)

Tools are reserved exclusively for state mutations, lifecycle actions, and task monitoring:

| Category | Tool | Parameters | Description |
| :--- | :--- | :--- | :--- |
| **Tasks** | `pve_task_status` | `node`, `upid` | Query Proxmox task exit status by UPID. Always registered. |
| **QEMU Power** | `pve_qemu_power` | `node`, `vmid`, `action`, `force`, `confirm` | Power control (`start`, `stop`, `shutdown`, `reboot`, `suspend`, `resume`). Gated by `PVE_ENABLE_POWER`. |
| **QEMU Config** | `pve_qemu_update_hardware` | `node`, `vmid`, `cores`, `memory`, `balloon`, `sockets`, `cpu`, `confirm` | Hotplug/update CPU cores, sockets, RAM, and ballooning. Gated by `PVE_ENABLE_CONFIG`. |
| | `pve_qemu_update_network` | `node`, `vmid`, `net_id`, `bridge`, `tag`, `firewall`, `rate`, `model`, `confirm` | Update or attach network interfaces with MAC address preservation. Gated by `PVE_ENABLE_CONFIG`. |
| **QEMU Storage** | `pve_qemu_resize_disk` | `node`, `vmid`, `disk`, `size`, `confirm` | Expand virtual disk capacity (disk shrinking is strictly rejected). Gated by `PVE_ENABLE_STORAGE`. |
| | `pve_qemu_clone` | `node`, `vmid`, `newid`, `name`, `full`, `storage`, `pool`, `wait`, `confirm` | Full or linked VM cloning with progress notification support. Gated by `PVE_ENABLE_STORAGE`. |
| **QEMU Destroy** | `pve_qemu_destroy` | `node`, `vmid`, `expected_name`, `purge`, `destroy_disks`, `unprotect`, `confirm` | Safely remove stopped VM. Gated by `PVE_ENABLE_DESTROY`. |
| | `pve_qemu_protection` | `node`, `vmid`, `protected`, `confirm` | Set or remove deletion protection flag. Gated by `PVE_ENABLE_DESTROY`. |
| **LXC Power** | `pve_lxc_power` | `node`, `vmid`, `action`, `force`, `confirm` | Container power control (`start`, `stop`, `shutdown`, `reboot`, `suspend`, `resume`). Gated by `PVE_ENABLE_POWER`. |
| **LXC Config** | `pve_lxc_update_hardware` | `node`, `vmid`, `cores`, `memory`, `swap`, `confirm` | Update container cores, RAM, and swap. Gated by `PVE_ENABLE_CONFIG`. |
| | `pve_lxc_update_network` | `node`, `vmid`, `net_id`, `bridge`, `name`, `tag`, `firewall`, `rate`, `ip`, `gw`, `confirm` | Update or attach container network interfaces. Gated by `PVE_ENABLE_CONFIG`. |
| **LXC Storage** | `pve_lxc_resize_disk` | `node`, `vmid`, `disk`, `size`, `confirm` | Expand rootfs or mount point capacity. Gated by `PVE_ENABLE_STORAGE`. |
| | `pve_lxc_clone` | `node`, `vmid`, `newid`, `hostname`, `full`, `storage`, `wait`, `confirm` | Clone container with progress notification support. Gated by `PVE_ENABLE_STORAGE`. |
| | `pve_lxc_create` | `node`, `vmid`, `ostemplate`, `hostname`, `cores`, `memory`, `disk`, `storage`, `bridge`, `ip`, `gateway`, `password`, `ssh_public_keys`, `unprivileged`, `start`, `wait`, `confirm` | Provision a brand new container from an OS template. Gated by `PVE_ENABLE_STORAGE`. |
| **LXC Destroy** | `pve_lxc_destroy` | `node`, `vmid`, `expected_name`, `purge`, `destroy_disks`, `unprotect`, `confirm` | Safely remove stopped container. Gated by `PVE_ENABLE_DESTROY`. |
| | `pve_lxc_protection` | `node`, `vmid`, `protected`, `confirm` | Set or remove deletion protection flag. Gated by `PVE_ENABLE_DESTROY`. |
| **Node Ops** | `pve_node_reboot` | `node`, `confirm` | Reboot hypervisor node with dry-run active workload preview. Gated by `PVE_ENABLE_NODE_OPS`. |
| | `pve_node_shutdown` | `node`, `confirm` | Shut down (cold power-off) hypervisor node with critical warning. Gated by `PVE_ENABLE_NODE_SHUTDOWN`. |

---

## 3. MCP Prompts (Operational Workflows)

Standard multi-step operational prompts guide the LLM through critical datacenter procedures:

1. **`cluster_health_audit`**: Inspects quorum status, node resource pressures, HA manager status, storage pools, and recent cluster logs.
2. **`vm_incident_triage`** (`node`, `vmid`): Deep diagnostic triage for an unhealthy or failed VM, inspecting runtime status, configuration, host syslog, and task logs.
3. **`safe_host_evacuation`** (`source_node`, `target_node`): Pre-maintenance evacuation planner verifying target capacity and proposing migration ordering.
4. **`provision_workload_planner`** (`workload_type`, `cores`, `memory_mb`, `disk_gb`): Evaluates cluster nodes, checks storage pool headroom, and fetches the next free VMID.
5. **`storage_cleanup_advisor`** (`node`, `storage`): Audits storage volumes, ISOs, snapshots, and backup archives to identify reclaimable space.

---

## Quickstart

### 1. Launch Server

```bash
# Environment Variables
export PVE_HOST="https://pve.example.com:8006"
export PVE_TOKEN_ID="auditor@pve!mcp"
export PVE_TOKEN_SECRET="00000000-0000-0000-0000-000000000000"
export PVE_VERIFY_SSL="false"  # if using self-signed certificates
export PVE_ENABLE_POWER="true"
export PVE_ENABLE_CONFIG="true"
export PVE_ENABLE_STORAGE="true"
export PVE_ENABLE_DESTROY="false"
export PVE_ENABLE_NODE_OPS="true"
export PVE_ENABLE_NODE_SHUTDOWN="false"
export MCP_AUTH_TOKEN="your-secure-bearer-token"

# Run HTTP/SSE Server
./pve-mcp

# Or run in Stdio mode for local agent integration
./pve-mcp --stdio

# Optional: read-only Web UI on http://127.0.0.1:8081 (needs its own token)
export MCP_UI_TOKEN="$(openssl rand -hex 32)"
./pve-mcp --ui
```

### 2. Docker & Docker Compose

Run with Docker:
```bash
# Build the minimal distroless image (~15MB)
docker build -t pve-mcp:latest .

# Run the container
docker run -d \
  --name pve-mcp \
  --restart unless-stopped \
  -p 127.0.0.1:8080:8080 \
  --env-file .env \
  -e MCP_BIND_ADDRESS="0.0.0.0" \
  pve-mcp:latest
```

Or using Docker Compose:
```bash
docker compose up -d
```

To also run the Web UI in the container, set `MCP_UI_TOKEN` in `.env` and add the UI variables and port to the `docker run` command above:
```bash
  -e MCP_UI_ENABLED=true \
  -e MCP_UI_BIND_ADDRESS="0.0.0.0" \
  -e MCP_UI_ALLOW_INSECURE_BIND="true" \
  -p 127.0.0.1:8081:8081
```
The UI speaks plain HTTP inside the container, so keep its host port on loopback or a trusted network.

### 3. Client Setup

Add to your MCP client configuration (`claude_desktop_config.json`, Cursor, or Antigravity):

```json
{
  "mcpServers": {
    "proxmox": {
      "url": "http://127.0.0.1:8080/sse",
      "headers": {
        "Authorization": "Bearer your-secure-bearer-token"
      }
    }
  }
}
```

Or for Stdio transport:

```json
{
  "mcpServers": {
    "proxmox": {
      "command": "/path/to/pve-mcp",
      "args": ["--stdio"],
      "env": {
        "PVE_HOST": "https://pve.example.com:8006",
        "PVE_TOKEN_ID": "root@pam!token",
        "PVE_TOKEN_SECRET": "00000000-0000-0000-0000-000000000000",
        "PVE_VERIFY_SSL": "false",
        "PVE_ENABLE_POWER": "true",
        "PVE_ENABLE_CONFIG": "true",
        "PVE_ENABLE_NODE_OPS": "true"
      }
    }
  }
}
```

---

## Security & Architecture

- **Fail-Closed Category Gating**: All mutating tool categories default to `false`. Tools belonging to disabled categories are **not registered** on the MCP server at startup, keeping them invisible to LLMs.
- **Two-Stage Defense**:
  1. *Stage 1 (Server Level)*: Admin enables/disables entire tool categories via environment variables (`PVE_ENABLE_*`).
  2. *Stage 2 (Client Level)*: Human operator approves individual tool calls interactively.
- **Node Whitelisting**: Set `PVE_ALLOWED_NODES` to restrict host operations exclusively to specific worker nodes.
- **Hard IAM Exclusion**: Permission modification (`Permissions.Modify`) is never exposed, preventing privilege escalation.
- **Two-Phase Confirmation**: Calling mutating tools with `confirm=false` returns an explicit dry-run preview (with running VM/CT impact warnings for node reboot/shutdown).
- **Target Binding & Digest Verification**: All guest mutation tools require `expected_digest` (matching the live SHA-1 hash of the guest `.conf` file) when `confirm=true`, eliminating mis-targeting risks and preventing modifications against stale state.
- **Protection & State Guards**: Machines must be stopped before destruction. Machines marked with `protection: 1` reject deletion requests.
- **Disk Shrinking Prevention**: Validates target disk size strictly exceeds current capacity before dispatching to Proxmox.
- **Progress Notifications**: Async operations support `notifications/progress` through client progress tokens.
- **Opt-in Web UI**: Disabled by default. When enabled it binds to loopback, requires its own `MCP_UI_TOKEN` (separate from `MCP_AUTH_TOKEN` and the PVE token), rejects unexpected `Host` and `Origin` headers, requires a custom header on state-changing requests, and cannot change guards, allowed nodes or tokens. Non-loopback binds are refused unless `MCP_UI_ALLOW_INSECURE_BIND=true` is set.
- **Transport Hardening**: TLS 1.2+ minimum, custom CA bundles (`PVE_CA_CERT`), SHA-256 fingerprint pinning (`PVE_FINGERPRINT`), and constant-time bearer authentication.

---

## Configuration Reference

| Variable | Required | Default | Description |
| :--- | :---: | :---: | :--- |
| `PVE_HOST` | **Yes** | — | Proxmox VE host URL (e.g., `https://pve.example.com:8006`). |
| `PVE_TOKEN_ID` | **Yes** | — | API Token ID (e.g., `auditor@pve!mcp`). |
| `PVE_TOKEN_SECRET` | **Yes** | — | API Token Secret UUID. |
| `PVE_ENABLE_POWER` | No | `false` | Enable VM and LXC power management (`pve_qemu_power`, `pve_lxc_power`). |
| `PVE_ENABLE_CONFIG` | No | `false` | Enable VM and LXC hardware and network reconfiguration tools. |
| `PVE_ENABLE_STORAGE` | No | `false` | Enable disk resizing, VM/CT cloning, and LXC container creation. |
| `PVE_ENABLE_DESTROY` | No | `false` | Enable permanent deletion and protection toggling (`pve_*_destroy`, `pve_*_protection`). |
| `PVE_ENABLE_NODE_OPS` | No | `false` | Enable hypervisor host node reboot (`pve_node_reboot`). |
| `PVE_ENABLE_NODE_SHUTDOWN` | No | `false` | Enable hypervisor host node cold shutdown (`pve_node_shutdown`). |
| `PVE_ALLOWED_NODES` | No | — | Optional comma-separated list of allowed nodes for host operations (e.g. `pve1,pve2`). |
| `PVE_VERIFY_SSL` | No | `true` | Set `false` for self-signed certificates. |
| `PVE_CA_CERT` | No | — | Path to custom CA PEM bundle. |
| `PVE_FINGERPRINT` | No | — | SHA-256 fingerprint of PVE SSL cert. |
| `MCP_BIND_ADDRESS` | No | `127.0.0.1` | Network interface IP to bind. |
| `PORT` | No | `8080` | Port for the HTTP/SSE listener. |
| `MCP_AUTH_TOKEN` | No | — | Optional bearer secret required to access MCP server. |
| `MCP_ALLOWED_ORIGINS` | No | — | Comma-separated allowed HTTP `Origin` headers. |
| `PVE_TIMEOUT_SECONDS` | No | `30` | HTTP request timeout in seconds. |
| `LOG_LEVEL` | No | `info` | Logging verbosity (`debug`, `info`, `warn`, `error`). |
| `MCP_UI_ENABLED` | No | `false` | Enable the opt-in Web UI (or pass `--ui`). Requires `MCP_UI_TOKEN`. |
| `MCP_UI_TOKEN` | With UI | — | Dedicated Web UI token. Separate from `MCP_AUTH_TOKEN` and the PVE token; the UI does not start without it. |
| `MCP_UI_BIND_ADDRESS` | No | `127.0.0.1` | Interface for the Web UI listener. Non-loopback addresses are refused unless `MCP_UI_ALLOW_INSECURE_BIND=true`. |
| `MCP_UI_PORT` | No | `8081` | Web UI port (or `--ui-port`). |
| `MCP_UI_ALLOW_INSECURE_BIND` | No | `false` | Allow a non-loopback UI bind. The token and session cookie then travel over plain HTTP. |
| `MCP_UI_ALLOWED_HOSTS` | No | — | Comma-separated extra `Host` header values accepted by the UI (DNS-rebinding guard). |

---

## Web UI (opt-in)

Off by default: when disabled nothing is allocated, no listener starts and no hooks are installed. Enable it with `--ui` or `MCP_UI_ENABLED=true` and set `MCP_UI_TOKEN`:

```bash
MCP_UI_TOKEN="$(openssl rand -hex 32)" ./pve-mcp --ui
# then open http://127.0.0.1:8081 and sign in with the token
```

It works in both `--stdio` and HTTP modes and currently provides (read-only):

- Health bar: PVE reachability and latency, transport, tool count, Go heap, uptime.
- Live event stream (SSE) of tool calls (read, dry-run, mutation, error), resource reads and server logs, held in a 250-entry in-memory ring buffer (not an audit log; it is lost on restart and can be cleared from the UI).
- Live log-level selector.

The UI cannot change guards (`PVE_ENABLE_*`), allowed nodes, tokens or any connection setting. The UI token should be treated like the PVE token: anyone holding it can read tool arguments and results. The token is never printed to the logs. Running in Docker requires `MCP_UI_BIND_ADDRESS=0.0.0.0`, `MCP_UI_ALLOW_INSECURE_BIND=true` and publishing the UI port only on a trusted interface (for example `-p 127.0.0.1:8081:8081`).

---

## Building from Source

```bash
# Windows
go build -trimpath -ldflags="-s -w" -o pve-mcp.exe ./cmd/pve-mcp

# Linux (amd64)
GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o pve-mcp-linux ./cmd/pve-mcp
```

---

## License

MIT
