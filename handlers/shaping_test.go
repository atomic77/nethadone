package handlers

import (
	"net"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/atomic77/nethadone/config"
	"github.com/atomic77/nethadone/database"
	"github.com/atomic77/nethadone/models"
	"github.com/vishvananda/netlink"
)

func TestRate(t *testing.T) {
	now := time.Now()
	if r := rate("t", 1000, now); r != 0 {
		t.Errorf("first sample = %v, want 0", r)
	}
	if r := rate("t", 3000, now.Add(2*time.Second)); r != 1000 {
		t.Errorf("rate = %v, want 1000", r)
	}
	if r := rate("t", 10, now.Add(3*time.Second)); r != 0 {
		t.Errorf("after counter reset = %v, want 0", r)
	}
}

func TestPoliciesByLeaf(t *testing.T) {
	got := policiesByLeaf([]models.Policy{{SrcIp: "10.0.0.2", GlobGroup: "yt", Class: 10}, {SrcIp: "10.0.0.3", GlobGroup: "yt", Class: 30}})
	if !slices.Equal(got["1:10"], []string{"10.0.0.2 yt"}) || !slices.Equal(got["1:30"], []string{"10.0.0.3 yt"}) {
		t.Errorf("got %v", got)
	}
}

// Builds the real qdisc tree and throttler on two throwaway dummy interfaces (never real
// ones), sends a throttled packet and checks the shaping page shows it in the right leaf.
func TestBPFShapingDummy(t *testing.T) {
	skipUnlessRoot(t)
	loadThrottler(t, nil) // skips without clang or libbpf headers
	lan, wan := "nthtest0", "nthtest1"
	for _, name := range []string{lan, wan} {
		d := &netlink.Dummy{LinkAttrs: netlink.LinkAttrs{Name: name}}
		netlink.LinkDel(d) // left over if a log.Fatal in a previous run skipped the cleanup
		if err := netlink.LinkAdd(d); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { netlink.LinkDel(d) })
		if err := netlink.LinkSetUp(d); err != nil {
			t.Fatal(err)
		}
	}
	l, _ := netlink.LinkByName(lan)
	addr, _ := netlink.ParseAddr("10.199.0.1/24")
	if err := netlink.AddrAdd(l, addr); err != nil {
		t.Fatal(err)
	}
	saved := config.Cfg
	t.Cleanup(func() { config.Cfg = saved; BpfCtx.ThrottleObjs = nil })
	config.Cfg.LanInterface, config.Cfg.WanInterface = lan, wan
	connectTempDb(t)

	createQdiscs()
	// LAN egress sees the router (10.199.0.1) as the "remote" sending to client 10.199.0.2.
	ApplyPolicies(&[]models.IpPolicy{{SrcIpAddr: "10,199,0,2", DestIpAddr: "10,199,0,1", ClassId: 30}})
	if err := database.UpdatePolicy("10.199.0.2", "yt", 30); err != nil {
		t.Fatal(err)
	}
	conn, err := net.Dial("udp", "10.199.0.2:9")
	if err != nil {
		t.Fatal(err)
	}
	conn.Write(make([]byte, 1000))
	conn.Close()
	time.Sleep(200 * time.Millisecond) // the leaf's netem holds packets for its delay before HTB counts them

	rec := httptest.NewRecorder()
	Shaping(rec, httptest.NewRequest("GET", "/shaping", nil))
	body := rec.Body.String()
	for _, want := range []string{"default 1:10", "1:30", "10.199.0.2 yt", "nthtest0", "1 rules", "1 of"} {
		if !strings.Contains(body, want) {
			t.Errorf("page lacks %q", want)
		}
	}

	qdiscs, _ := netlink.QdiscList(l)
	classes, _ := netlink.ClassList(l, 0)
	tree := tcTree(qdiscs, classes, time.Now())
	var handles []string
	for _, n := range tree {
		handles = append(handles, n.Handle)
		switch n.Handle {
		case "1:30":
			if n.Packets != 1 || n.CeilBits == 0 || n.Depth != 2 {
				t.Errorf("leaf 1:30 = %+v, want 1 packet, a ceiling and depth 2", n)
			}
		case "30:0":
			if n.DelayMs == 0 || n.Depth != 3 {
				t.Errorf("netem 30:0 = %+v, want a delay and depth 3", n)
			}
		}
	}
	t.Log(handles)
	if len(tree) < 2+2*(config.Cfg.NumQdiscClasses+1) {
		t.Errorf("tree has %d nodes: %v", len(tree), handles)
	}
}
