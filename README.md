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

## 2. MCP Tools (16 Active Verbs)

Tools are reserved exclusively for state mutations and task monitoring:

| Category | Tool | Parameters | Description |
| :--- | :--- | :--- | :--- |
| **Tasks** | `pve_task_status` | `node`, `upid` | Query Proxmox task exit status by UPID. |
| **QEMU** | `pve_qemu_power` | `node`, `vmid`, `action`, `force`, `confirm` | Power control (`start`, `stop`, `shutdown`, `reboot`, `suspend`, `resume`). Hard stop requires `force: true`. |
| | `pve_qemu_update_hardware` | `node`, `vmid`, `cores`, `memory_mb`, `balloon_mb`, `name`, `description`, `onboot`, `confirm` | Hotplug/update CPU cores, RAM, ballooning, description, and onboot. |
| | `pve_qemu_resize_disk` | `node`, `vmid`, `disk`, `size`, `confirm` | Expand virtual disk capacity (disk shrinking is strictly rejected). |
| | `pve_qemu_update_network` | `node`, `vmid`, `net_id`, `bridge`, `tag`, `firewall`, `rate`, `confirm` | Update or attach network interfaces with MAC address preservation. |
| | `pve_qemu_clone` | `node`, `vmid`, `newid`, `name`, `full`, `storage`, `target`, `wait`, `confirm` | Full or linked VM cloning with progress notification support. |
| | `pve_qemu_destroy` | `node`, `vmid`, `purge`, `destroy_unreferenced_disks`, `confirm` | Safely remove stopped VM. Requires `ALLOW_DESTROY=true` on server. |
| | `pve_qemu_protection` | `node`, `vmid`, `enable` | Set or remove deletion protection flag. |
| **LXC** | `pve_lxc_power` | `node`, `vmid`, `action`, `force`, `confirm` | Container power control (`start`, `stop`, `shutdown`, `reboot`, `suspend`, `resume`). |
| | `pve_lxc_update_hardware` | `node`, `vmid`, `cores`, `memory_mb`, `swap_mb`, `description`, `onboot`, `confirm` | Update container cores, RAM, swap, and description. |
| | `pve_lxc_resize_disk` | `node`, `vmid`, `disk`, `size`, `confirm` | Expand rootfs or mount point capacity. |
| | `pve_lxc_update_network` | `node`, `vmid`, `net_id`, `bridge`, `tag`, `firewall`, `rate`, `confirm` | Update or attach container network interfaces. |
| | `pve_lxc_clone` | `node`, `vmid`, `newid`, `hostname`, `full`, `storage`, `target`, `wait`, `confirm` | Clone container with progress notification support. |
| | `pve_lxc_create` | `node`, `vmid`, `ostemplate`, `hostname`, `cores`, `memory`, `disk`, `storage`, `bridge`, `ip`, `unprivileged`, `password`, `start`, `wait`, `confirm` | Provision a brand new container from an OS template. |
| | `pve_lxc_destroy` | `node`, `vmid`, `purge`, `destroy_unreferenced_disks`, `confirm` | Safely remove stopped container. Requires `ALLOW_DESTROY=true`. |
| | `pve_lxc_protection` | `node`, `vmid`, `enable` | Set or remove deletion protection flag. |

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
export PVE_ALLOW_MUTATIONS="true"  # if mutations are desired
export MCP_AUTH_TOKEN="your-secure-bearer-token"

# Run HTTP/SSE Server
./pve-mcp

# Or run in Stdio mode for local agent integration
./pve-mcp --stdio
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
        "PVE_ALLOW_MUTATIONS": "true"
      }
    }
  }
}
```

---

## Security & Architecture

- **Two-Phase Confirmation**: Calling any mutating tool with `confirm=false` returns an explicit `[DRY RUN PREVIEW - USER CONFIRMATION REQUIRED]` with a before-and-after diff. Mutations only execute when `confirm: true` is passed.
- **Environment Gating**:
  - `PVE_ALLOW_MUTATIONS` defaults to `false`. Mutating tools reject execution immediately when disabled.
  - `PVE_ALLOW_DESTROY` defaults to `false`. Deletion tools (`pve_qemu_destroy`, `pve_lxc_destroy`) are permanently disabled until explicitly enabled.
- **Protection & State Guards**: Machines must be stopped before destruction. Machines marked with `protection: 1` reject deletion requests.
- **Disk Shrinking Prevention**: Validates target disk size strictly exceeds current capacity before dispatching to Proxmox.
- **Progress Notifications**: Async operations support `notifications/progress` through client progress tokens.
- **Transport Hardening**: TLS 1.2+ minimum, custom CA bundles (`PVE_CA_CERT`), SHA-256 fingerprint pinning (`PVE_FINGERPRINT`), and constant-time bearer authentication.

---

## Configuration Reference

| Variable | Required | Default | Description |
| :--- | :---: | :---: | :--- |
| `PVE_HOST` | **Yes** | — | Proxmox VE host URL (e.g., `https://pve.example.com:8006`). |
| `PVE_TOKEN_ID` | **Yes** | — | API Token ID (e.g., `auditor@pve!mcp`). |
| `PVE_TOKEN_SECRET` | **Yes** | — | API Token Secret UUID. |
| `PVE_ALLOW_MUTATIONS` | No | `false` | Set `true` to enable VM/LXC power, hardware, clone, create, and protection operations. |
| `PVE_ALLOW_DESTROY` | No | `false` | Set `true` to explicitly allow permanent machine deletion (`pve_qemu_destroy`, `pve_lxc_destroy`). |
| `PVE_VERIFY_SSL` | No | `true` | Set `false` for self-signed certificates. |
| `PVE_CA_CERT` | No | — | Path to custom CA PEM bundle. |
| `PVE_FINGERPRINT` | No | — | SHA-256 fingerprint of PVE SSL cert. |
| `MCP_BIND_ADDRESS` | No | `127.0.0.1` | Network interface IP to bind. |
| `PORT` | No | `8080` | Port for the HTTP/SSE listener. |
| `MCP_AUTH_TOKEN` | No | — | Optional bearer secret required to access MCP server. |
| `MCP_ALLOWED_ORIGINS` | No | — | Comma-separated allowed HTTP `Origin` headers. |
| `PVE_TIMEOUT_SECONDS` | No | `30` | HTTP request timeout in seconds. |
| `LOG_LEVEL` | No | `info` | Logging verbosity (`debug`, `info`, `warn`, `error`). |

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
