package policy

import (
	"log"
	"time"

	"github.com/alecthomas/repr"
	"github.com/atomic77/nethadone/database"
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
	s1 := createSrcGlobSet(database.GetSrcGlobUsage(1, 0, k))
	s2 := createSrcGlobSet(database.GetSrcGlobUsage(1, 5, k))
	s3 := createSrcGlobSet(database.GetSrcGlobUsage(5, 0, k))

	s12 := s1.Intersect(s2)
	s := s3.Intersect(s12)

	repr.Println(s)
}

func decreasingThrottling() {

	// Implement me
}

func pollPolicyCheck() {
	log.Println("Setting up metrics collector")
	for range time.Tick(time.Minute * 1) {
		log.Println("Checking policy ")

		increaseThrottling()
	}
}
