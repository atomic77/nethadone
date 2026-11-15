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

// Iface is a network interface as shown on the status page.
type Iface struct {
	Name  string
	Up    bool
	Addrs []string
}
