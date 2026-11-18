package policy

import (
	"net"
	"path/filepath"
	"testing"
	"time"

	"github.com/atomic77/nethadone/config"
	"github.com/atomic77/nethadone/database"
	"github.com/atomic77/nethadone/models"
)

const (
	clientA = "192.168.0.2"
	clientB = "192.168.0.3"
	ytIP    = "142.250.1.1"
)

// setup gives each test fresh sqlite DBs with a "yt" glob group, a usage
// stub reporting `active` above the threshold (everyone else is below), and a capture of
// what would have been compiled into the throttler.
func setup(t *testing.T, active []string) *[][]models.IpPolicy {
	t.Helper()
	dir := t.TempDir()
	config.Cfg.DnsDb = filepath.Join(dir, "dns.db")
	config.Cfg.CfgDb = filepath.Join(dir, "cfg.db")
	config.Cfg.PolicyBackoffInterval = 0
	config.Cfg.NumQdiscClasses = 5
	database.Connect()

	ip := net.ParseIP(ytIP)
	if err := database.AddDns([]string{"www.youtube.com."}, []net.IP{ip}); err != nil {
		t.Fatal(err)
	}
	if err := database.AddGlob(&models.GlobGroup{Name: "yt", Glob: "*.youtube.com."}); err != nil {
		t.Fatal(err)
	}

	vec := func(srcs []string) []database.SrcGlob {
		v := []database.SrcGlob{}
		for _, s := range srcs {
			v = append(v, database.SrcGlob{Src: s, Glob: "yt"})
		}
		return v
	}
	origQuery, origApply := database.GetSrcGlobUsage, applyToKernel
	database.GetSrcGlobUsage = func(window, ago time.Duration, k int, above bool) []database.SrcGlob {
		if above {
			return vec(active)
		}
		panic("policy only asks for clients above the threshold")
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
	applied := setup(t, []string{clientA})

	checkPolicies()

	wantClass(t, clientA, 10)
	want := models.IpPolicy{SrcIpAddr: "192,168,0,2", DestIpAddr: "142,250,1,1", ClassId: 10}
	if len(*applied) != 1 || len((*applied)[0]) != 1 || (*applied)[0][0] != want {
		t.Errorf("applied %+v, want one apply of %+v", *applied, want)
	}
}

func TestBackoffBlocksRecentChange(t *testing.T) {
	applied := setup(t, []string{clientA})
	setPolicy(t, clientA, 20)
	config.Cfg.PolicyBackoffInterval = time.Hour

	checkPolicies()

	wantClass(t, clientA, 20)
	if len(*applied) != 0 {
		t.Errorf("nothing changed but policies were applied: %+v", *applied)
	}
}

func TestIncreaseAfterBackoff(t *testing.T) {
	setup(t, []string{clientA})
	setPolicy(t, clientA, 20)

	checkPolicies()

	wantClass(t, clientA, 30)
}

func TestIncreaseStopsAtSlowestClass(t *testing.T) {
	setup(t, []string{clientA})
	// 5 qdisc classes -> leaves 0x10 (open) .. 0x60
	setPolicy(t, clientA, 60)

	checkPolicies()

	wantClass(t, clientA, 60)
}

func TestIncreaseAndDecreaseInSameTick(t *testing.T) {
	applied := setup(t, []string{clientA})
	setPolicy(t, clientB, 30)

	checkPolicies()

	wantClass(t, clientA, 10)
	wantClass(t, clientB, 20)
	if len(*applied) != 1 || len((*applied)[0]) != 2 {
		t.Errorf("want one apply with both clients, got %+v", *applied)
	}
}

func TestDecreaseFromLowestClassDeletes(t *testing.T) {
	applied := setup(t, nil)
	setPolicy(t, clientB, 10)

	checkPolicies()

	wantClass(t, clientB, 0)
	if len(*applied) != 1 || len((*applied)[0]) != 0 {
		t.Errorf("want one apply with no policies, got %+v", *applied)
	}
}

func TestPolicyForDeletedGlobIsSkipped(t *testing.T) {
	applied := setup(t, []string{clientA})
	// Policy referencing a glob group that no longer exists must not panic
	if err := database.UpdatePolicy(clientB, "gone", 20); err != nil {
		t.Fatal(err)
	}

	checkPolicies()

	if len(*applied) != 1 || len((*applied)[0]) != 1 {
		t.Errorf("want only clientA's policy applied, got %+v", *applied)
	}
}

// A policy that survived a restart (or a DHCP change) has no usage entry at
// all, and must still decay instead of staying throttled forever.
func TestPolicyWithoutUsageDecays(t *testing.T) {
	setup(t, nil)
	setPolicy(t, clientB, 30)
	if err := database.UpdatePolicy(clientB, "gone", 20); err != nil {
		t.Fatal(err)
	}

	checkPolicies()

	wantClass(t, clientB, 20)
	if p := database.GetPolicy(clientB, "gone"); p == nil || p.Class != 10 {
		t.Errorf("policy for deleted glob: want class 10, got %+v", p)
	}
}

func TestNoDecayWithoutHistory(t *testing.T) {
	applied := setup(t, nil)
	setPolicy(t, clientB, 30)
	database.GetSrcGlobUsage = func(time.Duration, time.Duration, int, bool) []database.SrcGlob { return nil }

	checkPolicies()

	wantClass(t, clientB, 30)
	if len(*applied) != 0 {
		t.Errorf("nothing changed but policies were applied: %+v", *applied)
	}
}

func TestIntersect(t *testing.T) {
	a := database.SrcGlob{Src: clientA, Glob: "yt"}
	b := database.SrcGlob{Src: clientB, Glob: "yt"}
	c := database.SrcGlob{Src: clientA, Glob: "rd"}
	got := intersect([]database.SrcGlob{a, b, c}, []database.SrcGlob{c, a}, []database.SrcGlob{b, a})
	if len(got) != 1 || got[0] != a {
		t.Fatalf("intersect = %v, want [%v]", got, a)
	}
	if got := intersect([]database.SrcGlob{a}, nil); len(got) != 0 {
		t.Fatalf("intersect with empty = %v, want []", got)
	}
}
