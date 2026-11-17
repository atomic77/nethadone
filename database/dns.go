package database

import (
	"log"

	"github.com/atomic77/nethadone/models"
)

// GetDomainsByIP maps each IP in the DNS cache to every domain seen for it (a CNAME chain
// stores one row per name), newest first, in one query so callers walking thousands of
// IP pairs don't query per pair. Same 5 day window as GetIPsMatchingGlob.
// This should be enhanced to include the source IP as well
func GetDomainsByIP() map[string][]string {
	rows, err := dnsDb.Query(`
		SELECT daddr, domain FROM dns
		WHERE tstamp between date('now', '-5 days') and date('now', '1 days')
		ORDER BY tstamp DESC, rowid DESC`)
	if err != nil {
		log.Println("failed to query dns cache ", err)
		return nil
	}
	defer rows.Close()
	m := make(map[string][]string)
	for rows.Next() {
		var ip, domain string
		if err := rows.Scan(&ip, &domain); err != nil {
			log.Println("failed to scan dns cache ", err)
			return m
		}
		m[ip] = append(m[ip], domain)
	}
	return m
}

// PruneDns drops DNS rows older than the window GetIPsMatchingGlob considers.
func PruneDns() {
	res, err := dnsDb.Exec(`DELETE FROM dns WHERE tstamp < date('now', '-5 days')`)
	if err != nil {
		log.Println("failed to prune dns cache ", err)
		return
	}
	n, _ := res.RowsAffected()
	log.Println("Pruned ", n, " stale rows from DNS cache")
}

func GetIPsMatchingGlob(g *models.GlobGroup) []string {
	ips := make([]string, 0)
	sql := `
		SELECT distinct(daddr) FROM dns 
		WHERE CAST("domain" as VARCHAR) glob $1
		-- Only consider relatively recent IPs. This will need to
		-- be fine-tuned during real operation
		-- Also figure out UTC/local timezone issue
	    AND tstamp between date('now', '-5 days') and date('now', '1 days')
	`
	err := dnsDb.Select(&ips, sql, g.Glob)
	if err != nil {
		// log.Println("failed to query dns cache ", err)
		return nil
	}
	return ips
}
