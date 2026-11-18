package models

import (
	"net"
	"time"
)

type GlobGroup struct {
	Name        string `db:"name"`
	Description string `db:"description"`
	Glob        string `db:"glob"`
	Device      string `db:"device"`
}

type Device struct {
	Name  string
	Mac   string
	Group string
}

type Policy struct {
	SrcIp     string    `db:"src_ip"`
	GlobGroup string    `db:"glob_group"`
	Class     int       `db:"class"`
	Tstamp    time.Time `db:"tstamp"`
}

// IPaddr is comma separated due to use of IP_ADDRESS macro in bpf code, eg. 192,168,0,14
type IpPolicy struct {
	SrcIpAddr  string
	DestIpAddr string
	ClassId    int // The 'integer' part, not including 0x
}

// Bandwidth is one (LAN client, remote IP) pair from the trafficmon map, for the UI.
type Bandwidth struct {
	SrcIpAddr  net.IP
	DestIpAddr net.IP
	Bytes      uint64
	ProbDomain string
	GlobName   string
}

// Client is one LAN device, merged from trafficmon, the neighbour table and DHCP leases.
type Client struct {
	IpAddr     net.IP
	Mac        string
	Randomised bool   // locally administered MAC, typical of phones' private addresses
	Hostname   string // from the DHCP lease, if any
	State      string // neighbour table state, e.g. "reachable"; empty if not in the table
	Bytes      uint64
	Remotes    int
	Globs      []string
}

// TcNode is one qdisc or class in the LAN shaping tree, with the kernel's counters.
type TcNode struct {
	Handle     string
	Depth      int
	Kind       string // e.g. "htb", "netem", "class"
	RateBits   uint64 // HTB class rate and ceil, bits/s
	CeilBits   uint64
	DelayMs    float64 // netem added delay
	DefaultCls string  // HTB qdisc default class
	Bytes      uint64
	Packets    uint64
	Drops      uint32
	Overlimits uint32
	Backlog    uint32
	Bps        float64  // bytes/s since the previous page load
	Policies   []string // "src_ip glob" pairs whose policy class maps to this leaf
}

// IfaceStats is a LAN or WAN interface's kernel counters for the shaping page.
type IfaceStats struct {
	Name, Role       string
	RxBytes, TxBytes uint64
	RxBps, TxBps     float64
	RxDrops, TxDrops uint64
}

// SysStats is everything on the shaping page except the tree.
type SysStats struct {
	Uptime             time.Duration
	Load               [3]float64
	MemTotal, MemAvail uint64 // bytes
	Ifaces             []IfaceStats
	Scanned, Throttled uint64 // throttler packet counters, reset at each rebuild
	Rules              int
	RulesAt            time.Time
}

// Iface is a network interface as shown on the status page.
type Iface struct {
	Name  string
	Up    bool
	Addrs []string
}
