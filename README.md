# pve-mcp

Standalone, security-hardened Model Context Protocol (MCP) server for Proxmox Virtual Environment (PVE). Provides read-only access to Proxmox VE over Streamable HTTP/SSE.

## Architecture & Security Model

- **Read-Only Scope**: Exactly 29 read-only monitoring and inspection tools. Zero state-changing mutation endpoints.
- **Localhost Default**: Binds to `127.0.0.1` by default to prevent unintended LAN exposure. Set `MCP_BIND_ADDRESS=0.0.0.0` only when behind an authenticated reverse proxy or firewall.
- **Bearer Authentication**: Constant-time token verification (`crypto/subtle.ConstantTimeCompare`) via `MCP_AUTH_TOKEN`.
- **DNS Rebinding Protection**: Automatic loopback verification and optional origin whitelist via `MCP_ALLOWED_ORIGINS`.
- **Path Traversal Prevention**: Strict regex validation on node names (`^[a-zA-Z0-9_\-]+$`) and storage identifiers (`^[a-zA-Z0-9_\.\-]+$`), integer range validation on VMIDs (100–999,999,999), and path segment escaping (`url.PathEscape`).
- **Secret Sanitization**: PVE token secrets and authorization headers are scrubbed from all logs and error messages without corrupting multi-byte UTF-8 runes.
- **Transport Hardening**: TLS 1.2+ minimum, custom CA bundle support (`PVE_CA_CERT`), and SHA-256 fingerprint pinning (`PVE_FINGERPRINT`).
- **DoS Protection**: `ReadHeaderTimeout: 5s` and an explicit 10MB payload size limit.

---

## Configuration

Configure the server using the following environment variables:

| Variable | Required | Default | Description |
| :--- | :---: | :---: | :--- |
| `PVE_HOST` | **Yes** | — | Proxmox VE host URL (e.g., `https://192.168.1.100:8006`). |
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

1. Create an API token: `auditor@pve!mcp` with **Privilege Separation** enabled.
2. Assign the built-in **`PVEAuditor`** role to `auditor@pve!mcp` at the path `/`.
3. To enable node syslog inspection via `pve_node_syslog`, assign a custom role containing the `Sys.Syslog` privilege to `/`.

---

## Tool Reference (29 Tools)

### Cluster Tools (5)
- `pve_cluster_status`: Get cluster status and quorum information.
- `pve_cluster_resources`: Get cluster-wide resources (nodes, VMs, storage, pools) with optional `type` filter.
- `pve_cluster_nextid`: Get next available VMID.
- `pve_cluster_log`: Read cluster-wide log entries with optional `max` limit.
- `pve_cluster_ha_status`: Get High Availability (HA) cluster status.

### Node Tools (5)
- `pve_nodes_list`: List all cluster nodes with summary health and metrics.
- `pve_node_status`: Get detailed CPU, memory, and uptime status for a node (`node`).
- `pve_node_version`: Get package and kernel version details for a node (`node`).
- `pve_node_syslog`: Read system journal logs on a node (`node`, optional `limit`, `since`, `until`).
- `pve_node_rrddata`: Read RRD performance data for a node (`node`, `timeframe`, optional `cf`).

### QEMU Workload Tools (5)
- `pve_qemu_list`: List all virtual machines on a node (`node`).
- `pve_qemu_status`: Get current status of a virtual machine (`node`, `vmid`).
- `pve_qemu_config`: Get configuration details of a virtual machine (`node`, `vmid`).
- `pve_qemu_snapshots`: List snapshots for a virtual machine (`node`, `vmid`).
- `pve_qemu_firewall`: Get firewall rules for a virtual machine (`node`, `vmid`).

### LXC Container Tools (5)
- `pve_lxc_list`: List all LXC containers on a node (`node`).
- `pve_lxc_status`: Get current status of an LXC container (`node`, `vmid`).
- `pve_lxc_config`: Get configuration details of an LXC container (`node`, `vmid`).
- `pve_lxc_snapshots`: List snapshots for an LXC container (`node`, `vmid`).
- `pve_lxc_firewall`: Get firewall rules for an LXC container (`node`, `vmid`).

### Storage Tools (3)
- `pve_storage_list`: List storage pools accessible from a node (`node`).
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

## Client Configuration

### Claude Desktop / Cursor / Antigravity

Configure your MCP client configuration (`mcpServers`):

```json
{
  "mcpServers": {
    "proxmox": {
      "url": "http://127.0.0.1:8080/sse",
      "headers": {
        "Authorization": "Bearer YOUR_MCP_AUTH_TOKEN"
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
