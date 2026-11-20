package policy

import (
	"cmp"
	"log"
	"slices"
	"time"

	"github.com/atomic77/nethadone/config"
	"github.com/atomic77/nethadone/database"
	"github.com/atomic77/nethadone/handlers"
	"github.com/atomic77/nethadone/models"
)

func InitPolicy() {

	log.Println("Setting up SimpleLoadAverage policy")
	applyPolicies() // stored policies must survive a restart
	go pollPolicyCheck()
}

// intersect returns the entries of a that are also in every one of rest.
// GetSrcGlobUsage results come from map keys, so a has no duplicates.
func intersect(a []database.SrcGlob, rest ...[]database.SrcGlob) []database.SrcGlob {
	for _, b := range rest {
		in := make(map[database.SrcGlob]bool, len(b))
		for _, sg := range b {
			in[sg] = true
		}
		var out []database.SrcGlob
		for _, sg := range a {
			if in[sg] {
				out = append(out, sg)
			}
		}
		a = out
	}
	return a
}

func getActiveGlobGroups() []database.SrcGlob {
	// Anything returned is a client that has exceeded the
	// policy for a given glob group

	k, u := config.GetTunables().ActivityThresholdBytes, config.Cfg.PolicyTimeUnit
	return intersect(
		database.GetSrcGlobUsage(2*u, 0, k, true),
		database.GetSrcGlobUsage(2*u, 5*u, k, true),
		database.GetSrcGlobUsage(5*u, 0, k, true),
	)
}

// See README "Increasing throttling" section for policy details
func increaseThrottling() {

	backOff := time.Now().Add(-config.GetTunables().PolicyBackoffInterval)
	s := getActiveGlobGroups()

	log.Println("Found ", len(s), " globs for throttling")
	for _, match := range s {
		src_ip, glob := match.Src, match.Glob
		p := database.GetPolicy(src_ip, glob)

		if p == nil {
			err := database.UpdatePolicy(src_ip, glob, 10)
			if err != nil {
				log.Println("could not update policy: ", err)
			} else {
				log.Println("Setting new policy for ", src_ip, glob)
			}
			continue
		}

		if p.Tstamp.After(backOff) {
			log.Println("too soon to update policy for ", src_ip, glob)
		} else if p.Class > config.Cfg.NumQdiscClasses*10 {
			log.Println("already hit maximum policy for ", src_ip, glob)
		} else {
			log.Println("increasing throttling policy for ", src_ip, glob)
			if err := database.UpdatePolicy(src_ip, glob, p.Class+10); err != nil {
				log.Println("could not update policy: ", err)
			}
		}
	}
}

func decreasingThrottling() {
	t, u := config.GetTunables(), config.Cfg.PolicyTimeUnit
	k, backOff := t.ActivityThresholdBytes, time.Now().Add(-t.PolicyBackoffInterval)

	active := database.GetSrcGlobUsage(5*u, 0, k, true)
	if active == nil {
		return // not enough history yet (e.g. just restarted)
	}
	policies := database.GetAllPolicies()
	if policies == nil {
		return
	}
	isActive := make(map[database.SrcGlob]bool, len(active))
	for _, sg := range active {
		isActive[sg] = true
	}

	// Walk the stored policies rather than the usage store: policies outlive a
	// restart, a DHCP address change or a deleted glob group, and a client that
	// sends no traffic to the glob would otherwise never appear in usage at all.
	for _, p := range *policies {
		src_ip, glob := p.SrcIp, p.GlobGroup
		if isActive[database.SrcGlob{Src: src_ip, Glob: glob}] {
			continue
		}

		if p.Tstamp.After(backOff) {
			log.Println("too soon to update policy for ", src_ip, glob)
		} else if p.Class <= 10 {
			log.Println("deleting policy for ", src_ip, glob)
			database.DeletePolicy(src_ip, glob)
		} else {
			log.Println("decreasing throttling policy for ", src_ip, glob)
			database.UpdatePolicy(src_ip, glob, p.Class-10)
		}
	}
}

func pollPolicyCheck() {
	log.Println("Setting up policy checker on ", config.Cfg.PolicyCheckInterval, " interval ")
	for range time.Tick(config.Cfg.PolicyCheckInterval) {
		log.Println("Checking policy ")
		checkPolicies()
	}
}

func checkPolicies() {
	database.PruneDns()
	increaseThrottling()
	decreasingThrottling()
	// Always: a glob group gains IPs (a new video CDN host) without any class changing
	applyPolicies()
}

// applyToKernel is a variable so tests can capture policies instead of compiling BPF.
var applyToKernel = handlers.ApplyPolicies

// lastRules are the rules last sent to applyToKernel, sorted; nil until the first apply.
// Compared with what was sent, not what loaded, so a build that fails (clang out of
// memory) is retried only when the rules change, not on every check.
var lastRules []models.IpPolicy

// applyPolicies rebuilds the throttler when the rules differ from the last ones sent:
// a class changed, a policy came or went, or a glob group matched new IPs.
func applyPolicies() {
	policies := database.GetAllPolicies()
	if policies == nil {
		return
	}
	rules := make([]models.IpPolicy, 0)
	for _, p := range *policies {
		ipPolicies := database.GetIpPolicies(&p)
		if ipPolicies != nil {
			rules = append(rules, *ipPolicies...)
		}
	}
	slices.SortFunc(rules, func(a, b models.IpPolicy) int {
		return cmp.Or(cmp.Compare(a.SrcIpAddr, b.SrcIpAddr), cmp.Compare(a.GlobGroup, b.GlobGroup), cmp.Compare(a.DestIpAddr, b.DestIpAddr))
	})
	if lastRules != nil && slices.Equal(rules, lastRules) {
		return
	}
	log.Println("Reapplying policy: ", len(rules), " rules")
	lastRules = rules
	applyToKernel(&rules)
}
