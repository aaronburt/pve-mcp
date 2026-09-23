package pve

type ClusterStatusItem struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Type    string `json:"type"`
	IP      string `json:"ip,omitempty"`
	Level   string `json:"level,omitempty"`
	Local   int    `json:"local,omitempty"`
	NodeID  int    `json:"nodeid,omitempty"`
	Online  int    `json:"online,omitempty"`
	Version int    `json:"version,omitempty"`
}

type ClusterResource struct {
	ID        string  `json:"id"`
	Type      string  `json:"type"`
	Node      string  `json:"node,omitempty"`
	Status    string  `json:"status,omitempty"`
	VMID      int     `json:"vmid,omitempty"`
	Name      string  `json:"name,omitempty"`
	CPU       float64 `json:"cpu,omitempty"`
	MaxCPU    int     `json:"maxcpu,omitempty"`
	Mem       int64   `json:"mem,omitempty"`
	MaxMem    int64   `json:"maxmem,omitempty"`
	Disk      int64   `json:"disk,omitempty"`
	MaxDisk   int64   `json:"maxdisk,omitempty"`
	Uptime    int64   `json:"uptime,omitempty"`
	Pool      string  `json:"pool,omitempty"`
	DiskRead  int64   `json:"diskread,omitempty"`
	DiskWrite int64   `json:"diskwrite,omitempty"`
	NetIn     int64   `json:"netin,omitempty"`
	NetOut    int64   `json:"netout,omitempty"`
}

type NodeItem struct {
	Node    string  `json:"node"`
	Status  string  `json:"status"`
	CPU     float64 `json:"cpu"`
	MaxCPU  int     `json:"maxcpu"`
	Mem     int64   `json:"mem"`
	MaxMem  int64   `json:"maxmem"`
	Disk    int64   `json:"disk"`
	MaxDisk int64   `json:"maxdisk"`
	Uptime  int64   `json:"uptime"`
	Level   string  `json:"level"`
	SSLFPR  string  `json:"ssl_fingerprint,omitempty"`
}

type StorageItem struct {
	Storage string  `json:"storage"`
	Type    string  `json:"type"`
	Content string  `json:"content"`
	Active  int     `json:"active"`
	Enabled int     `json:"enabled"`
	Shared  int     `json:"shared"`
	Total   int64   `json:"total"`
	Used    int64   `json:"used"`
	Avail   int64   `json:"avail"`
	UsedPct float64 `json:"used_fraction,omitempty"`
}
