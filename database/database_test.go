package database

import (
	"path/filepath"
	"testing"

	"github.com/alecthomas/repr"
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
	Connect()

	dom := GetDomainForIP("142.251.33.174")

	repr.Println(dom)
}

func TestGlobRetrieve(t *testing.T) {
	Connect()

	g := models.GlobGroup{
		Glob: "*.youtube.com",
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
