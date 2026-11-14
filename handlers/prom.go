package handlers

import (
	"fmt"
	"log"
	"time"

	"github.com/VictoriaMetrics/metrics"
	"github.com/atomic77/nethadone/config"
	"github.com/atomic77/nethadone/database"
)

func InitMetrics() {
	log.Println("Registering prometheus metrics")
	metrics.ExposeMetadata(true)
	go pollMetrics()
}

func pollMetrics() {
	log.Println("Setting up metrics collector on ", config.Cfg.MapCollectionInterval, " interval")
	for range time.Tick(config.Cfg.MapCollectionInterval) {
		// TODO Once we get into the 1000s of IP pairs being tracked, the
		// performance of this method becomes quite slow
		bl := getBandwidthList(false)
		log.Println("Collected bandwidth stats from ", len(bl), " ip pairs")
		pairs := make([]database.PairBytes, 0, len(bl))
		for _, b := range bl {
			pairs = append(pairs, database.PairBytes{
				Src: b.SrcIpAddr.String(), Dst: b.DestIpAddr.String(), Glob: b.GlobName, Bytes: b.Bytes,
			})
			s := fmt.Sprintf(
				`ip_pair_vic_bytes_total{%s="%s", %s="%s", %s="%s", %s="%s"}`,
				"src_ip", b.SrcIpAddr.String(),
				"dest_ip", b.DestIpAddr.String(),
				"prob_domain", b.ProbDomain,
				"glob", b.GlobName,
			)
			metrics.GetOrCreateFloatCounter(s).Set(float64(b.Bytes))
		}
		database.RecordUsage(time.Now(), pairs)

	}
}
