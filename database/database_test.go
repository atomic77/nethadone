package database

import (
	"path/filepath"
	"testing"

	"github.com/alecthomas/repr"
	"github.com/atomic77/nethadone/config"
	"github.com/atomic77/nethadone/models"
)

func TestSqlXMapQuery(t *testing.T) {
	Connect()

	r := dnsDb.QueryRowx("SELECT * FROM dns LIMIT 1")
	m := make(map[string]interface{}, 1)
	r.MapScan(m)

	repr.Println(m)
}

func TestSqlXStruct(t *testing.T) {
	Connect()
	m := models.DnsProbe{}

	r := dnsDb.QueryRowx("SELECT * FROM dns LIMIT 1")
	r.StructScan(&m)

	repr.Println(m)
}

func TestDomainRetrieve(t *testing.T) {
	config.Cfg.DnsDb = "/tmp/dns.db"
	config.Cfg.CfgDb = "/tmp/cfg.db"
	Connect()

	dom := GetDomainForIP("104.17.24.14")

	repr.Println(dom)
}

func TestGlobRetrieve(t *testing.T) {
	Connect()

	g := models.GlobGroup{
		Glob: "*.youtube.com.",
	}
	dom := GetIPsMatchingGlob(&g)

	repr.Println(dom)
}

func TestDomainMatch(t *testing.T) {
	Connect()

	dom := GetDomainForIP("142.251.33.174")

	matched, err := filepath.Match("*tube.com", dom)
	repr.Println(err)
	repr.Println(matched)
	repr.Println(dom)
}

func BenchmarkDomainMatch(b *testing.B) {
	// TODO properly generate a test db rather than depending on a copy
	config.Cfg.DnsDb = "/tmp/dns.db"
	config.Cfg.CfgDb = "/tmp/cfg.db"
	Connect()
	for i := 0; i < b.N; i++ {
		dom := GetDomainForIP("104.17.24.14")
		if dom == "" {
			b.Fail()
		}
	}
}

func BenchmarkGlobMatch(b *testing.B) {
	config.Cfg.DnsDb = "/tmp/dns.db"
	config.Cfg.CfgDb = "/tmp/cfg.db"
	Connect()
	for i := 0; i < b.N; i++ {
		g := models.GlobGroup{
			Glob: "*.youtube.com.",
		}
		ips := GetIPsMatchingGlob(&g)
		if ips == nil {
			b.Fail()
		}
	}
}
