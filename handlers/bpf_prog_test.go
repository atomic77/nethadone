package handlers

import (
	"encoding/binary"
	"fmt"
	"net"
	"os"
	"os/exec"
	"slices"
	"testing"
	"time"

	"github.com/atomic77/nethadone/database"
	"github.com/atomic77/nethadone/models"
	"github.com/cilium/ebpf"
	"github.com/cilium/ebpf/perf"
)

// These tests load the BPF programs and run them against synthetic packets with
// BPF_PROG_TEST_RUN. Nothing is attached to any interface. Run with:
//   go test -exec sudo ./handlers -run BPF

const (
	lanClient = "192.168.0.2"
	remote    = "142.250.1.1"
)

// ipv4Packet builds an Ethernet + IPv4 (+ UDP when udpSrcPort != 0) frame.
func ipv4Packet(src, dst string, udpSrcPort uint16, payload []byte) []byte {
	l4 := payload
	proto := byte(6) // TCP; header contents don't matter to these programs
	if udpSrcPort != 0 {
		proto = 17
		l4 = make([]byte, 8+len(payload))
		binary.BigEndian.PutUint16(l4[0:], udpSrcPort)
		binary.BigEndian.PutUint16(l4[2:], 40000)
		binary.BigEndian.PutUint16(l4[4:], uint16(len(l4)))
		copy(l4[8:], payload)
	}
	pkt := make([]byte, 14+20+len(l4))
	binary.BigEndian.PutUint16(pkt[12:], 0x0800)
	ip := pkt[14:]
	ip[0] = 0x45
	binary.BigEndian.PutUint16(ip[2:], uint16(20+len(l4)))
	ip[8] = 64
	ip[9] = proto
	copy(ip[12:16], net.ParseIP(src).To4())
	copy(ip[16:20], net.ParseIP(dst).To4())
	copy(ip[20:], l4)
	return pkt
}

func run(t *testing.T, prog *ebpf.Program, pkt []byte) {
	t.Helper()
	if _, err := prog.Run(&ebpf.RunOptions{Data: pkt}); err != nil {
		t.Fatal("BPF_PROG_TEST_RUN: ", err)
	}
}

func TestBPFTrafficMonitor(t *testing.T) {
	skipUnlessRoot(t)
	connectTempDb(t)
	ip := net.ParseIP(remote)
	database.AddDns([]string{"www.youtube.com."}, []net.IP{ip})
	database.AddGlob(&models.GlobGroup{Name: "yt", Glob: "*.youtube.com."})

	objs := &trafficmonObjects{}
	if err := loadTrafficmonObjects(objs, nil); err != nil {
		t.Fatal(err)
	}
	defer objs.Close()
	BpfCtx.TrafficmonObjs = objs
	defer func() { BpfCtx.TrafficmonObjs = nil }()

	// LAN egress sees replies: remote -> client
	pkt := ipv4Packet(remote, lanClient, 0, make([]byte, 100))
	run(t, objs.TrafficMon, pkt)
	run(t, objs.TrafficMon, pkt)

	bl := getBandwidthList(false)
	if len(bl) != 1 {
		t.Fatalf("want 1 ip pair, got %+v", bl)
	}
	b := bl[0]
	if b.SrcIpAddr.String() != lanClient || b.DestIpAddr.String() != remote {
		t.Errorf("pair decoded as src=%s dest=%s, want src=%s dest=%s", b.SrcIpAddr, b.DestIpAddr, lanClient, remote)
	}
	if want := uint64(2 * (len(pkt) - 14)); b.Bytes != want {
		t.Errorf("bytes = %d, want %d", b.Bytes, want)
	}
	if b.ProbDomain != "www.youtube.com" || b.GlobName != "yt" {
		t.Errorf("domain/glob = %q/%q", b.ProbDomain, b.GlobName)
	}
}

func TestBPFDnsSniffer(t *testing.T) {
	skipUnlessRoot(t)
	connectTempDb(t)

	objs := &dnspktObjects{}
	if err := loadDnspktObjects(objs, nil); err != nil {
		t.Fatal(err)
	}
	defer objs.Close()
	reader, err := perf.NewReader(objs.DnsArr, 4096)
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()

	run(t, objs.UdpDnsSniff, ipv4Packet(remote, lanClient, 0, make([]byte, 64))) // not DNS
	run(t, objs.UdpDnsSniff, ipv4Packet("8.8.8.8", lanClient, 53, dnsAnswer(t, "www.youtube.com.", remote)))

	reader.SetDeadline(time.Now().Add(2 * time.Second))
	rec, err := reader.Read()
	if err != nil {
		t.Fatal("no DNS event from sniffer: ", err)
	}
	handleDnsPacket(rec.RawSample)
	if got := database.GetDomainsByIP()[remote]; !slices.Contains(got, "www.youtube.com") || !slices.Contains(got, "edge.www.youtube.com") {
		t.Errorf("sniffed answer stored as %q", got)
	}
	reader.SetDeadline(time.Now().Add(100 * time.Millisecond))
	if _, err := reader.Read(); err == nil {
		t.Error("non-DNS packet produced an event")
	}
}

// loadThrottler renders, compiles (clang, as on the router) and loads the throttler.
func loadThrottler(t *testing.T, policies []models.IpPolicy) *throttleObjects {
	t.Helper()
	if _, err := exec.LookPath("clang"); err != nil {
		t.Skip("clang not installed")
	}
	if _, err := os.Stat("/usr/include/bpf/bpf_helpers.h"); err != nil {
		t.Skip("libbpf headers not installed (libbpf-dev)")
	}
	objs, err := buildThrottler(&policies)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { objs.Map.Close(); objs.Prog.Close() })
	return objs
}

// throttleStat sums the per-CPU counter id (0 = scanned, 1 = throttled).
func throttleStat(t *testing.T, objs *throttleObjects, id uint32) uint32 {
	t.Helper()
	var perCPU []uint32
	if err := objs.Map.Lookup(id, &perCPU); err != nil {
		t.Fatal(err)
	}
	var sum uint32
	for _, v := range perCPU {
		sum += v
	}
	return sum
}

func TestBPFThrottlerMatches(t *testing.T) {
	skipUnlessRoot(t)
	objs := loadThrottler(t, []models.IpPolicy{
		{SrcIpAddr: "192,168,0,2", DestIpAddr: "142,250,1,1", ClassId: 30},
	})

	run(t, objs.Prog, ipv4Packet(remote, lanClient, 0, nil))     // throttled
	run(t, objs.Prog, ipv4Packet(remote, "192.168.0.3", 0, nil)) // other client
	run(t, objs.Prog, ipv4Packet(lanClient, remote, 0, nil))     // wrong direction

	if got := throttleStat(t, objs, 0); got != 3 {
		t.Errorf("scanned = %d, want 3", got)
	}
	if got := throttleStat(t, objs, 1); got != 1 {
		t.Errorf("throttled = %d, want 1", got)
	}
}

// One `if` per IP pair: make sure a large policy set still compiles and passes the verifier.
// Around 1000 rules, clang ("Branch target out of insn range") or the verifier ("cannot be
// patched due to 16-bit range") fail (kernel 7.0 / clang 21); probe with NTH_THROTTLE_RULES.
func TestBPFThrottlerScale(t *testing.T) {
	skipUnlessRoot(t)
	n := 500
	if s := os.Getenv("NTH_THROTTLE_RULES"); s != "" {
		fmt.Sscan(s, &n)
	}
	policies := make([]models.IpPolicy, n)
	for i := range policies {
		policies[i] = models.IpPolicy{SrcIpAddr: "192,168,0,2", DestIpAddr: fmt.Sprintf("10,%d,%d,%d", i>>16&255, i>>8&255, i&255), ClassId: 20}
	}
	start := time.Now()
	objs := loadThrottler(t, policies)
	t.Logf("%d rules compiled and verified in %s", n, time.Since(start))
	run(t, objs.Prog, ipv4Packet(fmt.Sprintf("10.0.%d.%d", (n-1)>>8&255, (n-1)&255), lanClient, 0, nil))
	if got := throttleStat(t, objs, 1); got != 1 {
		t.Errorf("last rule did not match: throttled = %d", got)
	}
}
