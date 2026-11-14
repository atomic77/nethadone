package policy

import (
	"net"
	"path/filepath"
	"testing"
	"time"

	"github.com/atomic77/nethadone/config"
	"github.com/atomic77/nethadone/database"
	"github.com/atomic77/nethadone/models"
	"github.com/prometheus/common/model"
)

const (
	clientA = "192.168.0.2"
	clientB = "192.168.0.3"
	ytIP    = "142.250.1.1"
)

// setup gives each test fresh sqlite DBs with a "yt" glob group, a Prometheus
// stub reporting `active` above and `idle` below the threshold, and a capture of
// what would have been compiled into the throttler.
func setup(t *testing.T, active, idle []string) *[][]models.IpPolicy {
	t.Helper()
	dir := t.TempDir()
	config.Cfg.DnsDb = filepath.Join(dir, "dns.db")
	config.Cfg.CfgDb = filepath.Join(dir, "cfg.db")
	config.Cfg.PolicyBackoffInterval = 0
	config.Cfg.NumQdiscClasses = 5
	database.Connect()

	ip := net.ParseIP(ytIP)
	if err := database.AddDns("www.youtube.com.", &ip); err != nil {
		t.Fatal(err)
	}
	if err := database.AddGlob(&models.GlobGroup{Name: "yt", Glob: "*.youtube.com."}); err != nil {
		t.Fatal(err)
	}

	vec := func(srcs []string) model.Vector {
		v := model.Vector{}
		for _, s := range srcs {
			v = append(v, &model.Sample{Metric: model.Metric{"src_ip": model.LabelValue(s), "glob": "yt"}})
		}
		return v
	}
	origQuery, origApply := database.GetSrcGlobUsage, applyToKernel
	database.GetSrcGlobUsage = func(rate, mins, k int, above bool) model.Vector {
		if above {
			return vec(active)
		}
		return vec(idle)
	}
	applied := [][]models.IpPolicy{}
	applyToKernel = func(p *[]models.IpPolicy) { applied = append(applied, *p) }
	t.Cleanup(func() { database.GetSrcGlobUsage, applyToKernel = origQuery, origApply })
	return &applied
}

func setPolicy(t *testing.T, src string, class int) {
	t.Helper()
	if err := database.UpdatePolicy(src, "yt", class); err != nil {
		t.Fatal(err)
	}
}

func wantClass(t *testing.T, src string, class int) {
	t.Helper()
	p := database.GetPolicy(src, "yt")
	switch {
	case class == 0 && p != nil:
		t.Errorf("%s: want no policy, got class %d", src, p.Class)
	case class != 0 && p == nil:
		t.Errorf("%s: want class %d, got no policy", src, class)
	case class != 0 && p.Class != class:
		t.Errorf("%s: want class %d, got %d", src, class, p.Class)
	}
}

func TestNewActiveClientGetsPolicy(t *testing.T) {
	applied := setup(t, []string{clientA}, nil)

	checkPolicies()

	wantClass(t, clientA, 10)
	want := models.IpPolicy{SrcIpAddr: "192,168,0,2", DestIpAddr: "142,250,1,1", ClassId: 10}
	if len(*applied) != 1 || len((*applied)[0]) != 1 || (*applied)[0][0] != want {
		t.Errorf("applied %+v, want one apply of %+v", *applied, want)
	}
}

func TestBackoffBlocksRecentChange(t *testing.T) {
	applied := setup(t, []string{clientA}, nil)
	setPolicy(t, clientA, 20)
	config.Cfg.PolicyBackoffInterval = time.Hour

	checkPolicies()

	wantClass(t, clientA, 20)
	if len(*applied) != 0 {
		t.Errorf("nothing changed but policies were applied: %+v", *applied)
	}
}

func TestIncreaseAfterBackoff(t *testing.T) {
	setup(t, []string{clientA}, nil)
	setPolicy(t, clientA, 20)

	checkPolicies()

	wantClass(t, clientA, 30)
}

func TestIncreaseStopsAtSlowestClass(t *testing.T) {
	setup(t, []string{clientA}, nil)
	// 5 qdisc classes -> leaves 0x10 (open) .. 0x60
	setPolicy(t, clientA, 60)

	checkPolicies()

	wantClass(t, clientA, 60)
}

func TestIncreaseAndDecreaseInSameTick(t *testing.T) {
	applied := setup(t, []string{clientA}, []string{clientB})
	setPolicy(t, clientB, 30)

	checkPolicies()

	wantClass(t, clientA, 10)
	wantClass(t, clientB, 20)
	if len(*applied) != 1 || len((*applied)[0]) != 2 {
		t.Errorf("want one apply with both clients, got %+v", *applied)
	}
}

func TestDecreaseFromLowestClassDeletes(t *testing.T) {
	applied := setup(t, nil, []string{clientB})
	setPolicy(t, clientB, 10)

	checkPolicies()

	wantClass(t, clientB, 0)
	if len(*applied) != 1 || len((*applied)[0]) != 0 {
		t.Errorf("want one apply with no policies, got %+v", *applied)
	}
}

func TestPolicyForDeletedGlobIsSkipped(t *testing.T) {
	applied := setup(t, []string{clientA}, nil)
	// Policy referencing a glob group that no longer exists must not panic
	if err := database.UpdatePolicy(clientB, "gone", 20); err != nil {
		t.Fatal(err)
	}

	checkPolicies()

	if len(*applied) != 1 || len((*applied)[0]) != 1 {
		t.Errorf("want only clientA's policy applied, got %+v", *applied)
	}
}
