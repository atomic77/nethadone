package handlers

import (
	"fmt"
	"log"
	"time"

	"github.com/VictoriaMetrics/metrics"
)

func InitMetrics() {
	log.Println("Registering prometheus metrics")
	metrics.ExposeMetadata(true)
	go pollMetrics()
}

func pollMetrics() {
	log.Println("Setting up metrics collector")
	for range time.Tick(time.Second * 10) {
		bl := getBandwidthList(false)
		log.Println("Tick happened, collected ", len(bl), " pairs")
		for _, b := range bl {
			s := fmt.Sprintf(
				`ip_pair_vic_bytes_total{%s="%s", %s="%s", %s="%s", %s="%s"}`,
				"src_ip", b.SrcIpAddr.String(),
				"dest_ip", b.DestIpAddr.String(),
				"prob_domain", b.ProbDomain,
				"glob", b.GlobName,
			)
			metrics.GetOrCreateFloatCounter(s).Set(float64(b.Bytes))
		}

	}
}
