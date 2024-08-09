package database

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/prometheus/client_golang/api"
	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"
)

var promDb api.Client

func GetSrcGlobUsage(rate int, mins int, k int, above bool) model.Vector {

	queryAPI := v1.NewAPI(promDb)

	op := ">"
	if !above {
		op = "<"
	}
	pql := fmt.Sprintf(
		`sum by (src_ip, glob) (
			rate(ip_pair_vic_bytes_total{glob!=""}[%dm])
		) %s %d`,
		rate, op, k)
	dr := time.Duration(int64(mins) * int64(time.Minute))
	tm := time.Now().Add(-1 * dr)
	result, warnings, err := queryAPI.Query(context.Background(), pql, tm)
	if err != nil {
		log.Println("unable to query prometheus, policy changes will not be possible: ", err, warnings)
		return model.Vector{}
	} else {

		return result.(model.Vector)
	}
}
