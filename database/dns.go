package database

import (
	"github.com/atomic77/nethadone/models"
)

// Retrieve the most likely domain for a given IP based on our
// DNS cache. This should be enhanced to include the source IP as well
func GetDomainForIP(ip string) string {
	var domain string
	sql := `
		SELECT domain FROM dns 
		WHERE daddr = $1
		ORDER BY tstamp DESC LIMIT 1
	`
	err := dnsDb.Get(&domain, sql, ip)
	if err != nil {
		// log.Println("failed to query dns cache ", err)
		return ""
	}
	return domain
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
