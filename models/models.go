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
	Name        string
	Description string `db:"description"`
	Glob        string `db:"glob"`
	Device      string `db:"device"`
}

type Device struct {
	Name  string
	Mac   string
	Group string
}
