package policy

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/alecthomas/repr"
	"github.com/atomic77/nethadone/database"
	"github.com/prometheus/client_golang/api"
	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"
)

func TestPromQuery(t *testing.T) {
	promURL := "http://localhost:9090"

	client, err := api.NewClient(api.Config{Address: promURL})
	if err != nil {
		t.Error(err)
	}

	// Create an API instance for querying Prometheus
	queryAPI := v1.NewAPI(client)

	// Example query to get the current Prometheus version
	pql := `sum by (src_ip, glob) (rate(ip_pair_vic_bytes_total{glob!=""}[5m]))`
	tm := time.Now().Add(-3 * time.Minute)
	result, warnings, err := queryAPI.Query(context.Background(), pql, tm)
	if err != nil {
		panic(err)
	}
	sample := result.(model.Vector)

	if len(warnings) > 0 {
		fmt.Println("Warnings:", warnings)
	}

	// Print the query result
	// fmt.Println(repr.String(sample))
	for _, samp := range sample {
		fmt.Println(
			"glob", samp.Metric["glob"],
			" src_ip", samp.Metric["src_ip"],
			"val: ", samp.Value)
	}

}

func TestIncrease(t *testing.T) {

	database.Connect()
	increaseThrottling()
}

func TestPolicyBase(t *testing.T) {

	database.Connect()
	k := 10
	vec1 := database.GetSrcGlobUsage(1, 0, k)
	vec2 := database.GetSrcGlobUsage(1, 5, k)
	vec3 := database.GetSrcGlobUsage(5, -6, k)

	repr.Println(vec1, vec2, vec3)

}
