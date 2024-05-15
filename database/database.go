package database

import (
	"log"

	"github.com/atomic77/nethadone/models"
	"github.com/jmoiron/sqlx"
	_ "modernc.org/sqlite"
)

var (
	// db  *sql.DB
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
	dnsDb, err = sqlx.Open("sqlite", "/tmp/dns.db?mode=ro")
	if err != nil {
		log.Fatalln("Could not open sqlite DNS db")
	}
	log.Println("Connected to DNS probe database")

	cfgDb, err = sqlx.Open("sqlite", "/tmp/cfg.db?mode=rw")
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
