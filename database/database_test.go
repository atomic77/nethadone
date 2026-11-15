package database

import (
	"net"
	"path/filepath"
	"testing"

	"github.com/atomic77/nethadone/config"
	"github.com/atomic77/nethadone/models"
)

// connectTemp points both sqlite databases at a fresh temp dir.
func connectTemp(t testing.TB) {
	t.Helper()
	dir := t.TempDir()
	config.Cfg.DnsDb = filepath.Join(dir, "dns.db")
	config.Cfg.CfgDb = filepath.Join(dir, "cfg.db")
	Connect()
	t.Cleanup(func() { dnsDb.Close(); cfgDb.Close() })
}

func addDns(t testing.TB, domain, ip string) {
	t.Helper()
	addr := net.ParseIP(ip)
	if err := AddDns(domain, &addr); err != nil {
		t.Fatal(err)
	}
}

func TestDomainForIP(t *testing.T) {
	connectTemp(t)
	addDns(t, "www.youtube.com.", "142.250.1.1")

	if got := GetDomainForIP("142.250.1.1"); got != "www.youtube.com." {
		t.Errorf("GetDomainForIP = %q", got)
	}
	if got := GetDomainForIP("1.1.1.1"); got != "" {
		t.Errorf("unknown IP: got %q, want empty", got)
	}
}

func TestIPsMatchingGlob(t *testing.T) {
	connectTemp(t)
	addDns(t, "www.youtube.com.", "142.250.1.1")
	addDns(t, "i.ytimg.youtube.com.", "142.250.1.2")
	addDns(t, "www.youtube.com.", "142.250.1.1") // re-seen IP must not duplicate
	addDns(t, "www.wikipedia.org.", "208.80.154.224")
	// Older than the 5 day window
	dnsDb.MustExec(`INSERT INTO dns VALUES ('0.0.0.0', '142.250.9.9', 'old.youtube.com.', datetime('now', '-10 days'))`)

	ips := GetIPsMatchingGlob(&models.GlobGroup{Glob: "*.youtube.com."})
	want := map[string]bool{"142.250.1.1": true, "142.250.1.2": true}
	if len(ips) != len(want) {
		t.Fatalf("got %v, want keys of %v", ips, want)
	}
	for _, ip := range ips {
		if !want[ip] {
			t.Errorf("unexpected IP %s", ip)
		}
	}
}

func TestPolicyCRUD(t *testing.T) {
	connectTemp(t)
	if p := GetPolicy("192.168.0.2", "yt"); p != nil {
		t.Fatalf("expected no policy, got %+v", p)
	}
	if err := UpdatePolicy("192.168.0.2", "yt", 10); err != nil {
		t.Fatal(err)
	}
	if err := UpdatePolicy("192.168.0.2", "yt", 20); err != nil {
		t.Fatal(err)
	}
	p := GetPolicy("192.168.0.2", "yt")
	if p == nil || p.Class != 20 || p.Tstamp.IsZero() {
		t.Fatalf("after upsert got %+v", p)
	}
	if n := len(*GetAllPolicies()); n != 1 {
		t.Errorf("GetAllPolicies has %d rows, want 1", n)
	}
	if err := DeletePolicy("192.168.0.2", "yt"); err != nil {
		t.Fatal(err)
	}
	if p := GetPolicy("192.168.0.2", "yt"); p != nil {
		t.Errorf("policy survived delete: %+v", p)
	}
}

func TestGetIpPolicies(t *testing.T) {
	connectTemp(t)
	addDns(t, "www.youtube.com.", "142.250.1.1")
	if err := AddGlob(&models.GlobGroup{Name: "yt", Glob: "*.youtube.com."}); err != nil {
		t.Fatal(err)
	}

	got := GetIpPolicies(&models.Policy{SrcIp: "192.168.0.2", GlobGroup: "yt", Class: 30})
	want := []models.IpPolicy{{SrcIpAddr: "192,168,0,2", DestIpAddr: "142,250,1,1", ClassId: 30}}
	if got == nil || len(*got) != 1 || (*got)[0] != want[0] {
		t.Errorf("got %+v, want %+v", got, want)
	}

	if got := GetIpPolicies(&models.Policy{GlobGroup: "missing"}); got != nil {
		t.Errorf("missing glob group: got %+v, want nil", got)
	}
}

func BenchmarkDomainForIP(b *testing.B) {
	connectTemp(b)
	addDns(b, "www.youtube.com.", "142.250.1.1")
	for i := 0; i < b.N; i++ {
		if GetDomainForIP("142.250.1.1") == "" {
			b.Fatal("lookup failed")
		}
	}
}
