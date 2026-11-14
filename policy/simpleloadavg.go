package policy

import (
	"log"
	"time"

	"github.com/atomic77/nethadone/config"
	"github.com/atomic77/nethadone/database"
	"github.com/atomic77/nethadone/handlers"
	"github.com/atomic77/nethadone/models"
	mapset "github.com/deckarep/golang-set/v2"
)

func InitPolicy() {

	log.Println("Setting up SimpleLoadAverage policy")
	go pollPolicyCheck()
}

func createSrcGlobSet(sgs []database.SrcGlob) mapset.Set[database.SrcGlob] {
	return mapset.NewSet(sgs...)
}

func getActiveGlobGroups() mapset.Set[database.SrcGlob] {
	// Anything we get in s is a client that has exceeded the
	// policy for a given glob group

	k, u := config.Cfg.ActivityThresholdBytes, config.Cfg.PolicyTimeUnit
	s1 := createSrcGlobSet(database.GetSrcGlobUsage(2*u, 0, k, true))
	s2 := createSrcGlobSet(database.GetSrcGlobUsage(2*u, 5*u, k, true))
	s3 := createSrcGlobSet(database.GetSrcGlobUsage(5*u, 0, k, true))

	s12 := s1.Intersect(s2)
	s := s3.Intersect(s12)
	return s
}

// See README "Increasing throttling" section for policy details
func increaseThrottling() bool {

	changed := false
	s := getActiveGlobGroups()

	log.Println("Found ", s.Cardinality(), " globs for throttling")
	for _, match := range s.ToSlice() {
		src_ip, glob := match.Src, match.Glob
		p := database.GetPolicy(src_ip, glob)

		if p == nil {
			err := database.UpdatePolicy(src_ip, glob, 10)
			if err != nil {
				log.Println("could not update policy: ", err)
			} else {
				log.Println("Setting new policy for ", src_ip, glob)
				changed = true
			}
			continue
		}

		backOff := time.Now().Add(-config.Cfg.PolicyBackoffInterval)
		if p.Tstamp.After(backOff) {
			log.Println("too soon to update policy for ", src_ip, glob)
		} else if p.Class > config.Cfg.NumQdiscClasses*10 {
			log.Println("already hit maximum policy for ", src_ip, glob)
		} else {
			log.Println("increasing throttling policy for ", src_ip, glob)
			if err := database.UpdatePolicy(src_ip, glob, p.Class+10); err != nil {
				log.Println("could not update policy: ", err)
			} else {
				changed = true
			}
		}
	}
	return changed
}

func decreasingThrottling() bool {

	changed := false
	k, u := config.Cfg.ActivityThresholdBytes, config.Cfg.PolicyTimeUnit

	s := createSrcGlobSet(database.GetSrcGlobUsage(5*u, 0, k, false))

	log.Println(s.Cardinality(), " globs that can be decreased")
	for _, match := range s.ToSlice() {
		src_ip, glob := match.Src, match.Glob
		p := database.GetPolicy(src_ip, glob)

		if p == nil {
			// If we have no policy in place and we're below the threshold
			// there is no need to do anything
			continue
		}

		backOff := time.Now().Add(-config.Cfg.PolicyBackoffInterval)
		if p.Tstamp.After(backOff) {
			log.Println("too soon to update policy for ", src_ip, glob)
		} else if p.Class <= 10 {
			log.Println("deleting policy for ", src_ip, glob)
			database.DeletePolicy(src_ip, glob)
			changed = true
		} else {
			log.Println("decreasing throttling policy for ", src_ip, glob)
			database.UpdatePolicy(src_ip, glob, p.Class-10)
			changed = true
		}
	}
	return changed
}

func pollPolicyCheck() {
	log.Println("Setting up policy checker on ", config.Cfg.PolicyCheckInterval, " interval ")
	for range time.Tick(config.Cfg.PolicyCheckInterval) {
		log.Println("Checking policy ")
		checkPolicies()
	}
}

func checkPolicies() {
	increased := increaseThrottling()
	decreased := decreasingThrottling()
	if increased || decreased {
		applyPolicies()
	}
}

// applyToKernel is a variable so tests can capture policies instead of compiling BPF.
var applyToKernel = handlers.ApplyPolicies

func applyPolicies() {
	log.Println("Reapplying policy")
	policies := database.GetAllPolicies()
	if policies == nil {
		return
	}
	allPolicies := make([]models.IpPolicy, 0)
	for _, p := range *policies {
		ipPolicies := database.GetIpPolicies(&p)
		if ipPolicies != nil {
			allPolicies = append(allPolicies, *ipPolicies...)
		}
	}
	applyToKernel(&allPolicies)

}
