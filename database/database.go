package database

import (
	"log"
	"net"
	"os"

	"github.com/atomic77/nethadone/models"
	"github.com/jmoiron/sqlx"
	_ "modernc.org/sqlite"
)

var (
	dnsDb *sqlx.DB
	cfgDb *sqlx.DB
)

func Connect() {
	var err error
	// Look into ORM solutions for golang later
	cfgSchema := []string{`
		CREATE TABLE IF NOT EXISTS glob_group (
			name  text primary key,
			description text,
			glob text,
			device text
		);`,
		`CREATE TABLE IF NOT EXISTS device (
			name  text primary key,
			mac   text,
			"group" text
		);`,
	}

	home, _ := os.UserHomeDir()
	dnsDb, err = sqlx.Open("sqlite", home+"/dns.db?mode=rw")
	if err != nil {
		log.Fatalln("Could not open sqlite DNS db")
	}
	log.Println("Connected to DNS probe database")
	dnsDb.MustExec(`CREATE TABLE IF NOT EXISTS dns (
		saddr text, daddr text, dport int, domain text, tstamp timestamp         
	);`)

	cfgDb, err = sqlx.Open("sqlite", home+"/cfg.db?mode=rw")
	if err != nil {
		log.Fatalln("Could not open sqlite cfg db")
	}
	log.Println("Connected to cfg database")
	for _, t := range cfgSchema {
		cfgDb.MustExec(t)
	}
}

func GetGlobs() []models.GlobGroup {

	globs := []models.GlobGroup{}
	cfgDb.Select(&globs, "SELECT * FROM glob_group ")
	return globs
}

func AddGlob(g *models.GlobGroup) error {
	sql := `INSERT INTO glob_group (name, description, glob, device) 
			VALUES (:name, :description, :glob, :device)`
	_, err := cfgDb.NamedExec(sql, g)
	return err
}

func AddDns(domain string, ipaddr *net.IP) error {

	sql := `INSERT INTO dns VALUES (:saddr, :daddr, :dport, :domain, datetime())`
	_, err := dnsDb.NamedExec(sql, map[string]interface{}{
		"saddr":  "0.0.0.0",
		"daddr":  ipaddr.String(),
		"dport":  0,
		"domain": domain,
	})
	return err

}
