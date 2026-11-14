package database

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/prometheus/client_golang/api"
)

func TestSrcGlobUsageQuery(t *testing.T) {
	var query string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.ParseForm()
		query = r.Form.Get("query")
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status":"success","data":{"resultType":"vector","result":[
			{"metric":{"src_ip":"192.168.0.2","glob":"yt"},"value":[1700000000,"512"]}]}}`))
	}))
	defer srv.Close()
	var err error
	promDb, err = api.NewClient(api.Config{Address: srv.URL})
	if err != nil {
		t.Fatal(err)
	}

	vec := GetSrcGlobUsage(2, 5, 100, false)

	compact := strings.Join(strings.Fields(query), " ")
	for _, want := range []string{`rate(ip_pair_vic_bytes_total{glob!=""}[2m])`, `) < 100`} {
		if !strings.Contains(compact, want) {
			t.Errorf("query %q missing %q", compact, want)
		}
	}
	if len(vec) != 1 || vec[0].Metric["src_ip"] != "192.168.0.2" || vec[0].Value != 512 {
		t.Errorf("unexpected result %v", vec)
	}
}

func TestSrcGlobUsageUnreachable(t *testing.T) {
	promDb, _ = api.NewClient(api.Config{Address: "http://127.0.0.1:1"})
	if vec := GetSrcGlobUsage(2, 0, 100, true); len(vec) != 0 {
		t.Errorf("want empty vector when Prometheus is down, got %v", vec)
	}
}
