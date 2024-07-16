package models

import "time"

var schema string

type DnsProbe struct {
	Saddr  string    `db:"saddr"`
	Daddr  string    `db:"daddr"`
	Dport  int       `db:"dport"`
	Domain string    `db:"domain"`
	Tstamp time.Time `db:"tstamp"`
}

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
