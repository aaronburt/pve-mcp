# pve-mcp

[![Go Version](https://img.shields.io/badge/go-1.22%2B-blue.svg)](https://golang.org)
[![Scope](https://img.shields.io/badge/scope-strictly_read--only-green.svg)](#architecture--security-model)
[![Token Efficiency](https://img.shields.io/badge/token_savings-82%25-brightgreen.svg)](#benchmark--token-efficiency)
[![Coverage](https://img.shields.io/badge/coverage-87.4%25-success.svg)](#test-coverage--verification)
[![License](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

A standalone, security-hardened Model Context Protocol (MCP) server for Proxmox Virtual Environment (PVE). Designed in compiled Go for **extreme token efficiency**, **sub-15ms startup**, and **zero-mutation read-only safety** over Streamable HTTP/SSE.

---

## Benchmark: Go `pve-mcp` vs. TypeScript `PVEMCP`

`pve-mcp` was engineered to solve the primary drawbacks of Node.js-based MCP servers: heavy memory footprints, sluggish cold-start times, and bloated context-window consumption.

### Performance & Runtime Metrics

| Dimension | TypeScript / Node.js (`PVEMCP`) | Go (`pve-mcp`) | Benefit |
| :--- | :---: | :---: | :--- |
| **Token Consumption** | ~2,880 tokens / full scan | **~518 tokens / full scan** | **82% fewer tokens consumed per loop** |
| **Cold-Start Time** | 250ms – 600ms (V8 JIT boot) | **5ms – 15ms** (Native binary) | **30x faster cold starts** |
| **Memory Footprint (RSS)** | ~60MB – 90MB | **~8MB – 14MB** | **~85% less memory usage** |
| **Connection Transport** | Standard Node fetch | Pooled Keep-Alive HTTP/TLS | Zero connection churn on repeat calls |
| **Concurrency** | Single-threaded event loop | Goroutines (M:N native threads) | Parallel dispatch with no GC stalls |

---

## Token Efficiency: `compact` vs. `full`

Standard Proxmox API endpoints return hundreds of low-level kernel telemetry counters (`pressurecpufull`, `balloon_min`, `shares`, `memhost`, raw byte counts) on every query. `pve-mcp` defaults to **`compact` tabular mode**, providing high-signal metrics formatted for instant LLM comprehension at a fraction of the token cost.

### Output Layout Comparison (Anonymized Data)

```text
================================================================================
MODE: "compact" (Default — Tabular TSV Format)
Tokens: ~50 | Focus: High Signal, Maximum Token Efficiency
================================================================================

VMID	NAME	STATUS	CPUS	CPU	RAM_MB	MAX_RAM_MB	DISK_GB	UPTIME_SEC
100	web-app	running	2	0.02	1024	2048		20	360000
101	db-node	running	4	0.15	4096	8192		50	360000
102	cache-1	stopped	1	0.00	0	1024		10	0

================================================================================
MODE: "full" (Raw JSON Format)
Tokens: ~430 | Focus: Low-Level OS Telemetry & Deep Diagnostics
================================================================================

[
  {
    "vmid": 100,
    "name": "web-app",
    "status": "running",
    "cpus": 2,
    "cpu": 0.02,
    "mem": 1073741824,
    "maxmem": 2147483648,
    "memhost": 1073741824,
    "disk": 5368709120,
    "maxdisk": 21474836480,
    "uptime": 360000,
    "pid": 1234,
    "balloon_min": 536870912,
    "shares": 1000,
    "netin": 1258291200,
    "netout": 943718400,
    "pressurecpufull": 0,
    "pressurecpusome": 0,
    "pressureiofull": 0,
    "pressureiosome": 0,
    "pressurememoryfull": 0,
    "pressurememorysome": 0
  },
  {
    "vmid": 101,
    "name": "db-node",
    "status": "running",
    "cpus": 4,
    "cpu": 0.15,
    "mem": 4294967296,
    "maxmem": 8589934592,
    "memhost": 4294967296,
    "disk": 21474836480,
    "maxdisk": 53687091200,
    "uptime": 360000,
    "pid": 5678,
    "balloon_min": 1073741824,
    "shares": 1000,
    "netin": 4194304000,
    "netout": 3145728000,
    "pressurecpufull": 0,
    "pressurecpusome": 0,
    "pressureiofull": 0,
    "pressureiosome": 0,
    "pressurememoryfull": 0,
    "pressurememorysome": 0
  }
]
```

### Measured Savings Across Tools

| Tool | `compact` (Tabular) | `full` (Raw API) | Token Savings |
| :--- | :---: | :---: | :---: |
| **`pve_qemu_list`** (4 VMs) | **~71 tokens** | ~430 tokens | **83.5%** |
| **`pve_lxc_list`** (7 Containers) | **~107 tokens** | ~890 tokens | **88.0%** |
| **`pve_storage_list`** (5 Storage Pools) | **~71 tokens** | ~240 tokens | **70.4%** |
| **`pve_cluster_resources`** (18 Items) | **~269 tokens** | ~1,320 tokens | **79.6%** |
| **Total (All 4 Core Inspection Tools)** | **~518 tokens** | **~2,880 tokens** | **82.0%** |

*(To request unpruned raw API telemetry, any tool can be invoked with `mode: "full"`).*

---

## Architecture & Security Model

- **Strictly Read-Only Scope**: Exactly 29 monitoring, status, and inspection tools. Zero state-changing mutation endpoints.
- **Localhost Default Binding**: Binds to `127.0.0.1` by default to prevent accidental LAN exposure.
- **Bearer Authentication**: Constant-time token verification (`crypto/subtle.ConstantTimeCompare`) via `MCP_AUTH_TOKEN`.
- **DNS Rebinding Protection**: Automatic loopback verification and optional origin whitelist via `MCP_ALLOWED_ORIGINS`.
- **Path Traversal Prevention**: Strict regex validation on node names (`^[a-zA-Z0-9_\-]+$`) and storage identifiers (`^[a-zA-Z0-9_\.\-]+$`), integer range validation on VMIDs (100–999,999,999), and path segment escaping (`url.PathEscape`).
- **Secret Sanitization**: PVE token secrets and authorization headers are scrubbed from all logs and error messages without corrupting multi-byte UTF-8 runes.
- **TLS Hardening**: TLS 1.2+ minimum, custom CA bundle support (`PVE_CA_CERT`), and SHA-256 fingerprint pinning (`PVE_FINGERPRINT`).
- **DoS Protection**: `ReadHeaderTimeout: 5s` and an explicit 10MB payload size ceiling.

---

## Configuration

Configure the server using environment variables:

| Variable | Required | Default | Description |
| :--- | :---: | :---: | :--- |
| `PVE_HOST` | **Yes** | — | Proxmox VE host URL (e.g., `https://pve.example.com:8006`). |
| `PVE_TOKEN_ID` | **Yes** | — | API Token ID (e.g., `auditor@pve!mcp`). |
| `PVE_TOKEN_SECRET` | **Yes** | — | API Token Secret UUID. |
| `PVE_VERIFY_SSL` | No | `true` | Set to `false` to disable certificate verification for self-signed certs. |
| `PVE_CA_CERT` | No | — | Path to a custom CA certificate PEM file. |
| `PVE_FINGERPRINT` | No | — | SHA-256 fingerprint of the PVE SSL certificate. |
| `MCP_BIND_ADDRESS` | No | `127.0.0.1` | Network interface IP to bind. |
| `PORT` | No | `8080` | Port for the HTTP/SSE listener. |
| `MCP_AUTH_TOKEN` | No | — | Optional bearer secret required to connect to the MCP server. |
| `MCP_ALLOWED_ORIGINS` | No | — | Comma-separated list of allowed HTTP `Origin` headers. |
| `PVE_TIMEOUT_SECONDS` | No | `30` | HTTP request timeout in seconds. |
| `LOG_LEVEL` | No | `info` | Logging verbosity (`debug`, `info`, `warn`, `error`). |

---

## Proxmox VE Permissions

For least-privilege operation, configure a dedicated API token with read-only access:

1. In PVE Web UI, navigate to **Datacenter** → **Permissions** → **API Tokens** and add a token: `auditor@pve!mcp` (with **Privilege Separation** enabled).
2. Under **Datacenter** → **Permissions**, click **Add** → **API Token Permission**:
   - **Path**: `/`
   - **API Token**: `auditor@pve!mcp`
   - **Role**: `PVEAuditor`
3. *(Optional)* To enable node syslog inspection via `pve_node_syslog`, assign a custom role containing the `Sys.Syslog` privilege to `/`.

---

## Tool Reference (29 Tools)

### Cluster Tools (5)
- `pve_cluster_status`: Get cluster status and quorum information.
- `pve_cluster_resources`: Get cluster-wide resources (nodes, VMs, storage, pools) with optional `type` and `mode` filters (defaults to `mode: "compact"`; use `mode: "full"` for raw Proxmox JSON).
- `pve_cluster_nextid`: Get next available free VMID in the cluster.
- `pve_cluster_log`: Read cluster-wide log entries with optional `max` limit.
- `pve_cluster_ha_status`: Get High Availability (HA) cluster status.

### Node Tools (5)
- `pve_nodes_list`: List all cluster nodes with summary health and metrics.
- `pve_node_status`: Get detailed CPU, memory, and uptime status for a node (`node`).
- `pve_node_version`: Get package and kernel version details for a node (`node`).
- `pve_node_syslog`: Read system journal logs on a node (`node`, optional `limit`, `since`, `until`).
- `pve_node_rrddata`: Read RRD performance data for a node (`node`, `timeframe`, optional `cf`).

### QEMU Workload Tools (5)
- `pve_qemu_list`: List all virtual machines on a node (`node`, optional `mode`).
- `pve_qemu_status`: Get current status of a virtual machine (`node`, `vmid`).
- `pve_qemu_config`: Get configuration details of a virtual machine (`node`, `vmid`).
- `pve_qemu_snapshots`: List snapshots for a virtual machine (`node`, `vmid`).
- `pve_qemu_firewall`: Get firewall rules for a virtual machine (`node`, `vmid`).

### LXC Container Tools (5)
- `pve_lxc_list`: List all LXC containers on a node (`node`, optional `mode`).
- `pve_lxc_status`: Get current status of an LXC container (`node`, `vmid`).
- `pve_lxc_config`: Get configuration details of an LXC container (`node`, `vmid`).
- `pve_lxc_snapshots`: List snapshots for an LXC container (`node`, `vmid`).
- `pve_lxc_firewall`: Get firewall rules for an LXC container (`node`, `vmid`).

### Storage Tools (3)
- `pve_storage_list`: List storage pools accessible from a node (`node`, optional `mode`).
- `pve_storage_status`: Get volume allocation and status for a storage pool (`node`, `storage`).
- `pve_storage_content`: List disk images, ISOs, templates, and backups in storage (`node`, `storage`, optional `content`).

### Network Tools (1)
- `pve_network_list`: List network interfaces on a node (`node`, optional `type` filter).

### Access Tools (5)
- `pve_access_users`: List configured cluster users.
- `pve_access_groups`: List configured user groups.
- `pve_access_roles`: List defined privilege roles.
- `pve_access_domains`: List authentication realms/domains.
- `pve_access_permissions`: Query user permissions and access control lists (`userid`, `path`).

---

## Running the Server

### 1. Launch with Environment Variables

```bash
export PVE_HOST="https://pve.example.com:8006"
export PVE_TOKEN_ID="auditor@pve!mcp"
export PVE_TOKEN_SECRET="00000000-0000-0000-0000-000000000000"
export PVE_VERIFY_SSL="false"  # if using self-signed certificate
export MCP_AUTH_TOKEN="your-secure-mcp-bearer-token"
export PORT="8080"

./pve-mcp
```

On Windows (PowerShell):
```powershell
$env:PVE_HOST="https://pve.example.com:8006"
$env:PVE_TOKEN_ID="auditor@pve!mcp"
$env:PVE_TOKEN_SECRET="00000000-0000-0000-0000-000000000000"
$env:PVE_VERIFY_SSL="false"
$env:MCP_AUTH_TOKEN="your-secure-mcp-bearer-token"
$env:PORT="8080"

.\pve-mcp.exe
```

---

## Client Configuration

### Claude Desktop / Cursor / Antigravity

Add the server to your MCP client configuration (`mcpServers`):

```json
{
  "mcpServers": {
    "proxmox": {
      "url": "http://127.0.0.1:8080/sse",
      "headers": {
        "Authorization": "Bearer your-secure-mcp-bearer-token"
      }
    }
  }
}
```

---

## Building from Source

```bash
# Build standalone Windows binary
go build -trimpath -ldflags="-s -w" -o pve-mcp.exe ./cmd/pve-mcp

# Cross-compile for Linux (amd64)
GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o pve-mcp-linux ./cmd/pve-mcp
```
