package database

import (
	"log"
	"net"
	"os"
	"path/filepath"
	"strings"

	"github.com/atomic77/nethadone/config"
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
		`CREATE TABLE IF NOT EXISTS policy (
			src_ip text,
			glob_group text,
			class int,
			tstamp timestamp,
			PRIMARY KEY (src_ip, glob_group)	
		);`,
		`CREATE TABLE IF NOT EXISTS setting (
			name  text primary key,
			value text
		);`,
	}

	for _, db := range []string{config.Cfg.DnsDb, config.Cfg.CfgDb} {
		if err := os.MkdirAll(filepath.Dir(db), 0o750); err != nil {
			log.Fatal("could not create database directory: ", err)
		}
	}

	dnsDb, err = sqlx.Open("sqlite", config.Cfg.DnsDb+"?mode=rw&busy_timeout=1000")
	dnsDb.DB.SetMaxOpenConns(1)
	if err != nil {
		log.Fatalln("Could not open sqlite DNS db")
	}
	log.Println("Connected to DNS probe database")
	dnsDb.MustExec(`CREATE TABLE IF NOT EXISTS dns (
		saddr text, 
		daddr text,
		domain text, 
		tstamp timestamp,
		PRIMARY KEY (daddr, saddr, domain)
	);`)
	// Older versions stored names as "www.example.com."; OR REPLACE drops a row whose new form already exists
	dnsDb.MustExec(`UPDATE OR REPLACE dns SET domain = lower(rtrim(domain, '.')) WHERE domain LIKE '%.' OR domain <> lower(domain)`)
	PruneDns()

	cfgDb, err = sqlx.Open("sqlite", config.Cfg.CfgDb+"?mode=rw&busy_timeout=1000")
	cfgDb.DB.SetMaxOpenConns(1)
	if err != nil {
		log.Fatalln("Could not open sqlite cfg db")
	}
	log.Println("Connected to cfg database")
	for _, t := range cfgSchema {
		cfgDb.MustExec(t)
	}
	cfgDb.MustExec(`UPDATE glob_group SET glob = lower(rtrim(glob, '.')) WHERE glob LIKE '%.' OR glob <> lower(glob)`)
}

func GetGlob(name string) *models.GlobGroup {

	glob := models.GlobGroup{}
	err := cfgDb.Get(&glob, "SELECT * FROM glob_group WHERE name = $1", name)
	if err != nil {
		return nil
	}
	return &glob
}

func GetGlobs() []models.GlobGroup {

	globs := []models.GlobGroup{}
	cfgDb.Select(&globs, "SELECT * FROM glob_group ")
	return globs
}

// NormaliseName lowercases a domain name or glob and drops the trailing root dot,
// so "*.reddit.com" matches what DNS returns as "WWW.reddit.com.".
func NormaliseName(s string) string {
	return strings.ToLower(strings.TrimSuffix(s, "."))
}

// AddGlob stores g with its glob normalised (g.Glob is updated in place).
func AddGlob(g *models.GlobGroup) error {
	g.Glob = NormaliseName(g.Glob)
	sql := `INSERT INTO glob_group (name, description, glob, device) 
			VALUES (:name, :description, :glob, :device)`
	_, err := cfgDb.NamedExec(sql, g)
	return err
}

func DeleteGlob(name string) error {
	_, err := cfgDb.Exec("DELETE FROM glob_group WHERE name = $1", name)
	return err
}

// AddDns records every (name, ip) combination of one DNS answer in a single transaction,
// so a CNAME chain costs one disk sync instead of one per row; the perf buffer feeding
// handleDnsPacket overflows if each answer takes too long to store.
func AddDns(names []string, ips []net.IP) error {
	tx, err := dnsDb.Beginx()
	if err != nil {
		return err
	}
	defer tx.Rollback() // no-op after Commit
	// TODO Source of the DNS request should be tracked as well
	// to ensure shared IPs aren't filtered for clients accessing
	// unrelated sites
	stmt, err := tx.Preparex(`INSERT INTO dns VALUES ('0.0.0.0', $1, $2, datetime())
	ON CONFLICT (saddr, daddr, domain)
	DO UPDATE SET tstamp = datetime()`)
	if err != nil {
		return err
	}
	defer stmt.Close()
	for _, ip := range ips {
		for _, n := range names {
			if _, err := stmt.Exec(ip.String(), NormaliseName(n)); err != nil {
				return err
			}
		}
	}
	return tx.Commit()
}

func GetPolicy(src_ip string, glob_group string) *models.Policy {

	sql := `SELECT * FROM policy 
			WHERE src_ip = $1 AND glob_group = $2`
	policy := models.Policy{}
	err := cfgDb.Get(&policy, sql, src_ip, glob_group)
	if err != nil {
		// log.Println("failure getting policy:", err)
		return nil
	}
	return &policy
}

func GetAllPolicies() *[]models.Policy {

	sql := `SELECT * FROM policy `
	policies := make([]models.Policy, 0)
	err := cfgDb.Select(&policies, sql)
	if err != nil {
		log.Println("failure getting policies:", err)
		return nil
	}
	return &policies
}

func UpdatePolicy(src_ip string, glob_group string, class int) error {
	// Update/create a policy entry for an ip->glob mapping

	sql := `INSERT INTO policy 
	VALUES (:src_ip, :glob_group, :class, datetime())
	ON CONFLICT (src_ip, glob_group) 
	DO UPDATE SET class = :class, tstamp = datetime()
	`
	_, err := cfgDb.NamedExec(sql, map[string]interface{}{
		"src_ip":     src_ip,
		"glob_group": glob_group,
		"class":      class,
	})
	return err
}

func DeletePolicy(src_ip string, glob_group string) error {
	sql := `DELETE FROM policy 
	WHERE src_ip = :src_ip 
	AND glob_group = :glob_group
	`
	_, err := cfgDb.NamedExec(sql, map[string]interface{}{
		"src_ip":     src_ip,
		"glob_group": glob_group,
	})
	return err
}

func GetIpPolicies(p *models.Policy) *[]models.IpPolicy {
	// Get all IP-level policies for the related glob group based on the IPs that have
	// been observed in the system

	gg := GetGlob(p.GlobGroup)
	if gg == nil {
		return nil
	}
	ipstr := GetIPsMatchingGlob(gg)
	if ipstr == nil {
		return nil
	}
	ipPolicies := make([]models.IpPolicy, len(ipstr))

	src := strings.ReplaceAll(p.SrcIp, ".", ",")
	if p.SrcIp == AllDevices {
		src = "" // the throttler template then matches the remote IP for every LAN device
	}
	for idx := range ipPolicies {
		ipPolicies[idx].ClassId = p.Class
		ipPolicies[idx].DestIpAddr = strings.ReplaceAll(ipstr[idx], ".", ",")
		ipPolicies[idx].SrcIpAddr = src
	}

	return &ipPolicies
}
