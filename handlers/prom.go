package handlers

import (
	"fmt"
	"log"
	"sync/atomic"
	"time"

	"github.com/VictoriaMetrics/metrics"
	"github.com/atomic77/nethadone/config"
	"github.com/atomic77/nethadone/database"
	"github.com/atomic77/nethadone/models"
)

// pairMetrics holds the latest collection's per IP pair counters, served on /metrics.
var pairMetrics atomic.Pointer[metrics.Set]

// pairMetricName quotes the label values: an unescaped `"` (e.g. in a glob group name)
// makes the metrics library panic.
func pairMetricName(b models.Bandwidth) string {
	return fmt.Sprintf(`ip_pair_vic_bytes_total{src_ip=%q,dest_ip=%q,prob_domain=%q,glob=%q}`,
		b.SrcIpAddr.String(), b.DestIpAddr.String(), b.ProbDomain, b.GlobName)
}

func InitMetrics() {
	log.Println("Registering prometheus metrics")
	metrics.ExposeMetadata(true)
	go pollMetrics()
}

func pollMetrics() {
	log.Println("Setting up metrics collector on ", config.Cfg.MapCollectionInterval, " interval")
	for range time.Tick(config.Cfg.MapCollectionInterval) {
		bl := getBandwidthList(false)
		log.Println("Collected bandwidth stats from ", len(bl), " ip pairs")
		pairs := make([]database.PairBytes, 0, len(bl))
		// A fresh set each time, so pairs that vanished stop taking memory
		set := metrics.NewSet()
		for _, b := range bl {
			pairs = append(pairs, database.PairBytes{
				Src: b.SrcIpAddr.String(), Dst: b.DestIpAddr.String(), Glob: b.GlobName, Bytes: b.Bytes,
			})
			set.GetOrCreateFloatCounter(pairMetricName(b)).Set(float64(b.Bytes))
		}
		pairMetrics.Store(set)
		database.RecordUsage(time.Now(), pairs)
		setLastBandwidth(bl)

	}
}
