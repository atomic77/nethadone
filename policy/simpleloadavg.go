package policy

import (
	"log"
	"time"

	"github.com/atomic77/nethadone/database"
	"github.com/atomic77/nethadone/handlers"
	"github.com/atomic77/nethadone/models"
	mapset "github.com/deckarep/golang-set/v2"
	"github.com/prometheus/common/model"
)

type srcGlob struct {
	src  model.LabelValue
	glob model.LabelValue
}

func InitPolicy() {

	log.Println("Setting up SimpleLoadAverage policy")
	go pollPolicyCheck()
}

func createSrcGlobSet(vec model.Vector) mapset.Set[srcGlob] {
	s := mapset.NewSet[srcGlob]()

	for _, v := range vec {
		sg := srcGlob{
			src:  v.Metric["src_ip"],
			glob: v.Metric["glob"],
		}
		s.Add(sg)
	}

	return s
}

// See README "Increasing throttling" section for policy details
func increaseThrottling() {

	k := 1
	// s1 := createSrcGlobSet(database.GetSrcGlobUsage(1, 0, k, true))
	// s2 := createSrcGlobSet(database.GetSrcGlobUsage(1, 5, k, true))
	s1 := createSrcGlobSet(database.GetSrcGlobUsage(5, 0, k, true))
	s2 := createSrcGlobSet(database.GetSrcGlobUsage(5, 0, k, true))
	s3 := createSrcGlobSet(database.GetSrcGlobUsage(5, 0, k, true))

	s12 := s1.Intersect(s2)
	s := s3.Intersect(s12)

	// Anything we get in s is a client that has exceeded the
	// policy for a given glob group

	for _, match := range s.ToSlice() {
		src_ip := string(match.src)
		glob := string(match.glob)
		p := database.GetPolicy(src_ip, glob)

		if p == nil {
			err := database.UpdatePolicy(src_ip, glob, 10)
			if err != nil {
				log.Println("could not update policy: ", err)
			}
			continue
		}

		// After testing set this back to 5 or some more sensible default
		// from configuration
		backOff := time.Now().Add(time.Minute * -1)
		if p.Tstamp.After(backOff) {
			log.Println("too soon to update policy for ", src_ip, glob)
		} else if p.Class > handlers.BaseConfig.NumQdiscClasses {
			log.Println("already hit maximum policy for ", src_ip, glob)
		} else {
			log.Println("increasing throttling policy for ", src_ip, glob)
			database.UpdatePolicy(src_ip, glob, p.Class+10)
		}
	}
}

func decreasingThrottling() {

	k := 1

	s := createSrcGlobSet(database.GetSrcGlobUsage(5, 0, k, false))

	for _, match := range s.ToSlice() {
		src_ip := string(match.src)
		glob := string(match.glob)
		p := database.GetPolicy(src_ip, glob)

		if p == nil {
			// If we have no policy in place and we're below the threshold
			// there is no need to do anything
			continue
		}

		// After testing set this back to 5 or some more sensible default
		// from configuration
		backOff := time.Now().Add(time.Minute * -1)
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
	log.Println("Setting up metrics collector")
	for range time.Tick(time.Minute * 1) {
		log.Println("Checking policy ")
		increaseThrottling()
		decreasingThrottling()
		applyPolicies()
	}
}

func applyPolicies() {
	log.Println("Reapplying policy")
	policies := database.GetAllPolicies()
	allPolicies := make([]models.IpPolicy, 0)
	for _, p := range *policies {
		ipPolicies := database.GetIpPolicies(&p)
		for _, ip := range *ipPolicies {
			allPolicies = append(allPolicies, ip)
		}
		// This isn't working for some reason though the static checker is complaining about the loop above ?
		// allPolicies = append(allPolicies, *ipPolicies)
	}
	handlers.ApplyPolicies(&allPolicies)

}
func pollPolicyApply() {
	for range time.Tick(time.Minute * 1) {
		applyPolicies()
	}
}
