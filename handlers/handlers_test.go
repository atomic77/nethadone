package handlers

import (
	"bytes"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/VictoriaMetrics/metrics"
	"github.com/atomic77/nethadone/config"
	"github.com/atomic77/nethadone/database"
	"github.com/atomic77/nethadone/models"
	"github.com/miekg/dns"
)

func connectTempDb(t testing.TB) {
	t.Helper()
	dir := t.TempDir()
	config.Cfg.DnsDb = filepath.Join(dir, "dns.db")
	config.Cfg.CfgDb = filepath.Join(dir, "cfg.db")
	database.Connect()
}

func TestRenderThrottler(t *testing.T) {
	var buf bytes.Buffer
	policies := []models.IpPolicy{
		{SrcIpAddr: "192,168,0,2", DestIpAddr: "142,250,1,1", ClassId: 30},
		{DestIpAddr: "10,0,0,1", ClassId: 10},
	}
	if err := renderThrottler(&buf, "throttle.bpf.c.tpl", &policies); err != nil {
		t.Fatal(err)
	}
	src := strings.Join(strings.Fields(buf.String()), " ")
	for _, want := range []string{
		"l3->saddr == IP_ADDRESS(142,250,1,1) && l3->daddr == IP_ADDRESS(192,168,0,2) ) { skb->tc_classid = 0x30;",
		"l3->saddr == IP_ADDRESS(10,0,0,1) ) { skb->tc_classid = 0x10;",
	} {
		if !strings.Contains(src, want) {
			t.Errorf("rendered throttler missing %q", want)
		}
	}
}

// The class a policy emits (0x{{ .ClassId }}) must be a leaf class createLeafQdiscs made.
func TestPolicyClassMatchesLeafHandle(t *testing.T) {
	classRe := regexp.MustCompile(`tc_classid = 0x([0-9a-f]+);`)
	for idx := 0; idx <= config.Cfg.NumQdiscClasses; idx++ {
		class := (idx + 1) * 10
		var buf bytes.Buffer
		renderThrottler(&buf, "throttle.bpf.c.tpl", &[]models.IpPolicy{{DestIpAddr: "1,2,3,4", ClassId: class}})
		m := classRe.FindStringSubmatch(buf.String())
		if m == nil {
			t.Fatal("no tc_classid in rendered throttler")
		}
		var minor uint16
		fmt.Sscanf(m[1], "%x", &minor)
		if minor != leafHandle(idx) {
			t.Errorf("policy class %d emits minor %#x, but leaf %d is %#x", class, minor, idx, leafHandle(idx))
		}
	}
}

func dnsAnswer(t *testing.T, name string, ips ...string) []byte {
	t.Helper()
	m := new(dns.Msg)
	m.SetQuestion(name, dns.TypeA)
	m.Response = true
	m.Answer = append(m.Answer, &dns.CNAME{Hdr: dns.RR_Header{Name: name, Rrtype: dns.TypeCNAME, Class: dns.ClassINET}, Target: "edge." + name})
	for _, ip := range ips {
		m.Answer = append(m.Answer, &dns.A{Hdr: dns.RR_Header{Name: "edge." + name, Rrtype: dns.TypeA, Class: dns.ClassINET}, A: net.ParseIP(ip)})
	}
	b, err := m.Pack()
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestPageBounds(t *testing.T) {
	for _, c := range []struct{ total, page, start, end, clamped, pages int }{
		{0, 1, 0, 0, 1, 1},       // empty list still has one page
		{250, 0, 0, 100, 1, 3},   // missing/invalid page -> first
		{250, 3, 200, 250, 3, 3}, // short last page
		{250, 9, 200, 250, 3, 3}, // past the end -> last
		{200, 2, 100, 200, 2, 2}, // exact multiple
	} {
		s, e, p, n := pageBounds(c.total, c.page, 100)
		if s != c.start || e != c.end || p != c.clamped || n != c.pages {
			t.Errorf("pageBounds(%d, %d) = %d,%d,%d,%d; want %+v", c.total, c.page, s, e, p, n, c)
		}
	}
}

func TestBandwidthPage(t *testing.T) {
	bl := make([]models.Bandwidth, 150)
	for i := range bl {
		bl[i] = models.Bandwidth{SrcIpAddr: net.IPv4(192, 168, 0, 2), DestIpAddr: net.IPv4(10, 0, byte(i/256), byte(i)), Bytes: uint64(i)}
	}
	setLastBandwidth(bl)
	t.Cleanup(func() { setLastBandwidth(nil) })

	rec := httptest.NewRecorder()
	Bandwidth(rec, httptest.NewRequest("GET", "/bandwidth?page=2", nil))
	body := rec.Body.String()
	if !strings.Contains(body, "page 2 of 2") || !strings.Contains(body, "/bandwidth?page=1") {
		t.Errorf("missing pagination in page 2")
	}
	// sorted descending, so page 2 holds the 50 smallest: 10.0.0.49 down to 10.0.0.0
	if !strings.Contains(body, "10.0.0.49") || strings.Contains(body, "10.0.0.50<") {
		t.Errorf("page 2 has wrong rows")
	}
}

func TestHandleDnsPacket(t *testing.T) {
	connectTempDb(t)
	// dnspkt.bpf.c always submits its full 508 byte buffer, zero padded
	raw := make([]byte, 508)
	copy(raw, dnsAnswer(t, "www.youtube.com.", "142.250.1.1", "142.250.1.2"))

	handleDnsPacket(raw)

	for _, ip := range []string{"142.250.1.1", "142.250.1.2"} {
		// Stored under the queried name and the CNAME target, normalised
		got := database.GetDomainsByIP()[ip]
		slices.Sort(got)
		if !slices.Equal(got, []string{"edge.www.youtube.com", "www.youtube.com"}) {
			t.Errorf("%s -> %q", ip, got)
		}
	}
	handleDnsPacket([]byte{0xde, 0xad}) // garbage must not panic
}

func TestHandleDnsPacketLongChain(t *testing.T) {
	connectTempDb(t)
	// doc/todo.md example: media.licdn.com -> 3 CNAMEs -> A
	m := new(dns.Msg)
	m.SetQuestion("media.licdn.com.", dns.TypeA)
	m.Response = true
	chain := []string{"media.licdn.com.", "2-01-2c3e-005c.cdx.cedexis.net.", "od.linkedin.edgesuite.net.", "a1916.dscg2.akamai.net."}
	for i := 0; i+1 < len(chain); i++ {
		m.Answer = append(m.Answer, &dns.CNAME{Hdr: dns.RR_Header{Name: chain[i], Rrtype: dns.TypeCNAME, Class: dns.ClassINET}, Target: chain[i+1]})
	}
	m.Answer = append(m.Answer, &dns.A{Hdr: dns.RR_Header{Name: chain[3], Rrtype: dns.TypeA, Class: dns.ClassINET}, A: net.ParseIP("206.248.168.162")})
	raw, err := m.Pack()
	if err != nil {
		t.Fatal(err)
	}
	handleDnsPacket(raw)

	if got := database.GetDomainsByIP()["206.248.168.162"]; len(got) != 4 || !slices.Contains(got, "media.licdn.com") {
		t.Errorf("chain stored as %q", got)
	}
	if ips := database.GetIPsMatchingGlob(&models.GlobGroup{Glob: "*.licdn.com"}); !slices.Equal(ips, []string{"206.248.168.162"}) {
		t.Errorf("glob on queried name matched %v", ips)
	}
}

func TestMatchDomains(t *testing.T) {
	globs := []models.GlobGroup{{Name: "yt", Glob: "*youtube.com"}}
	// newest name is the CDN one; the glob still matches the queried name
	if dom, glob := matchDomains(globs, []string{"youtube-ui.l.google.com", "www.youtube.com"}); dom != "www.youtube.com" || glob != "yt" {
		t.Errorf("got %q/%q", dom, glob)
	}
	if dom, glob := matchDomains(globs, []string{"a.example.com", "b.example.com"}); dom != "a.example.com" || glob != "" {
		t.Errorf("no match: got %q/%q, want newest name and no glob", dom, glob)
	}
	if dom, glob := matchDomains(globs, nil); dom != "" || glob != "" {
		t.Errorf("unknown IP: got %q/%q", dom, glob)
	}
}

func TestClassLabel(t *testing.T) {
	config.Cfg.NumQdiscClasses, config.Cfg.StartRateKbs, config.Cfg.MinRateKbs, config.Cfg.MaxDelayMs = 5, 1000, 50, 50
	for c, want := range map[int]string{10: "unthrottled", 20: "810 kbit/s, +10 ms", 60: "50 kbit/s, +50 ms"} {
		if got := classLabel(c); got != want {
			t.Errorf("classLabel(%d) = %q, want %q", c, got, want)
		}
	}
}

func TestGlobForm(t *testing.T) {
	connectTempDb(t)
	post := func(h http.HandlerFunc, form url.Values) *httptest.ResponseRecorder {
		req := httptest.NewRequest("POST", "/globs", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		rec := httptest.NewRecorder()
		h(rec, req)
		return rec
	}
	for _, bad := range []url.Values{{"name": {"x"}}, {"name": {"x"}, "glob": {"[bad"}}} {
		if rec := post(GlobAdd, bad); rec.Code != http.StatusUnprocessableEntity || !strings.Contains(rec.Body.String(), `role="alert"`) {
			t.Errorf("%v: got %d, want 422 with an error", bad, rec.Code)
		}
	}
	ok := url.Values{"name": {"yt"}, "glob": {"*.youtube.com"}}
	if rec := post(GlobAdd, ok); rec.Code != http.StatusSeeOther || database.GetGlob("yt") == nil {
		t.Fatalf("add: got %d, glob %v", rec.Code, database.GetGlob("yt"))
	}
	if rec := post(GlobAdd, ok); rec.Code != http.StatusUnprocessableEntity {
		t.Errorf("duplicate add: got %d, want 422", rec.Code)
	}
	if rec := post(GlobDelete, url.Values{"name": {"yt"}}); rec.Code != http.StatusSeeOther || database.GetGlob("yt") != nil {
		t.Errorf("delete: got %d, glob still present: %v", rec.Code, database.GetGlob("yt") != nil)
	}
}

func TestPairMetricNameEscapes(t *testing.T) {
	b := models.Bandwidth{SrcIpAddr: net.IPv4(192, 168, 0, 2), DestIpAddr: net.IPv4(1, 2, 3, 4),
		ProbDomain: `a\"b.example.com`, GlobName: `my "fun" sites\`}
	if err := metrics.ValidateMetric(pairMetricName(b)); err != nil {
		t.Fatal(err)
	}
}
