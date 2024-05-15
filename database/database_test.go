package database

import (
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
