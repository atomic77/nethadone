package handlers

import (
	"bytes"
	"fmt"
	"net"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

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

func TestHandleDnsPacket(t *testing.T) {
	connectTempDb(t)
	// dnspkt.bpf.c always submits its full 508 byte buffer, zero padded
	raw := make([]byte, 508)
	copy(raw, dnsAnswer(t, "www.youtube.com.", "142.250.1.1", "142.250.1.2"))

	handleDnsPacket(raw)

	for _, ip := range []string{"142.250.1.1", "142.250.1.2"} {
		// Current behaviour: A records are stored under the CNAME target, not the queried name (doc/todo.md)
		if got := database.GetDomainForIP(ip); got != "edge.www.youtube.com." {
			t.Errorf("%s -> %q", ip, got)
		}
	}
	handleDnsPacket([]byte{0xde, 0xad}) // garbage must not panic
}
