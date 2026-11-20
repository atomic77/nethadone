package handlers

import (
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"os/exec"
	"slices"
	"sync"
	"text/template"
	"time"

	"github.com/atomic77/nethadone/config"
	"github.com/atomic77/nethadone/database"
	"github.com/atomic77/nethadone/models"
	"github.com/cilium/ebpf"
	"github.com/cilium/ebpf/perf"
	"github.com/miekg/dns"
	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"
)

// TODO These generated bpf2go files should probably go somewhere else
//go:generate go run github.com/cilium/ebpf/cmd/bpf2go dnspkt ../ebpf/dnspkt.bpf.c - -O2  -Wall -Werror -Wno-address-of-packed-member
//go:generate go run github.com/cilium/ebpf/cmd/bpf2go trafficmon ../ebpf/trafficmon.bpf.c - -O2  -Wall -Werror -Wno-address-of-packed-member

/*
TODO Revisit whether there is another way that bpf2go can used while avoiding the
embedded bytes that prevent reloading the program in a live running server
go generate go run github.com/cilium/ebpf/cmd/bpf2go throttle ../ebpf/throttle.bpf.c - -O2  -Wall -Werror -Wno-address-of-packed-member
*/
type throttleObjects struct {
	// TODO We are still dependent on direct clang compilation for the tcfilt bpf
	// programs; investigate if this can be moved to bpf2go as with the dns sniffer
	Map  *ebpf.Map     `ebpf:"throttle_stats"`
	Prog *ebpf.Program `ebpf:"throttle"`
}

type BpfContext struct {
	// bpf2go generated
	DnspktObjs     *dnspktObjects
	TrafficmonObjs *trafficmonObjects
	// Custom - avoids go:embed that prevents live-reload
	ThrottleObjs *throttleObjects
}

var BpfCtx BpfContext

// counter sums throttle_stats entry id (0 = scanned, 1 = throttled) over all CPUs.
func (objs *throttleObjects) counter(id uint32) (uint64, error) {
	var perCPU []uint32
	if err := objs.Map.Lookup(id, &perCPU); err != nil {
		return 0, err
	}
	var sum uint64
	for _, v := range perCPU {
		sum += uint64(v)
	}
	return sum, nil
}

func (objs *throttleObjects) Close() error {
	return errors.Join(objs.Map.Close(), objs.Prog.Close())
}

func Initialize() {

	createQdiscs()
	// Load static BPF programs
	BpfCtx.DnspktObjs = &dnspktObjects{}
	err := loadDnspktObjects(BpfCtx.DnspktObjs, nil)
	if err != nil {
		log.Fatalln("failed to load DNS sniffer bpf program ", err)
	}

	BpfCtx.TrafficmonObjs = &trafficmonObjects{}
	err = loadTrafficmonObjects(BpfCtx.TrafficmonObjs, nil)
	if err != nil {
		log.Fatalln("failed to load traffic monitor bpf program ", err)
	}

	// For reasons I've yet to understand (I feel like this isn't the first
	// time I'm adding such a comment :) DNS answers don't seem to be caught
	// on the LAN interface...
	attachFilter(config.Cfg.WanInterface, netlink.HANDLE_MIN_EGRESS, BpfCtx.DnspktObjs.UdpDnsSniff, "dnspkt", 0)
	attachFilter(config.Cfg.WanInterface, netlink.HANDLE_MIN_INGRESS, BpfCtx.DnspktObjs.UdpDnsSniff, "dnspkt", 0)
	attachFilter(config.Cfg.LanInterface, netlink.HANDLE_MIN_EGRESS, BpfCtx.TrafficmonObjs.TrafficMon, "trafficmon", 0)
	// One reader: a second one on the same perf map takes it over and leaves the first idle
	go pollDnsResponses()
	// The throttler is attached by policy.InitPolicy, from the stored policies
}

func ApplyPolicies(ipPolicies *[]models.IpPolicy) {
	objs, err := buildThrottler(ipPolicies)
	if err != nil {
		// Exiting would drop all throttling; the previous throttler stays attached instead
		log.Println("keeping the previous throttler: ", err)
		return
	}
	throttleMu.Lock()
	defer throttleMu.Unlock()
	reattachThrottler(objs, config.Cfg.LanInterface)
	throttleRules, throttleRulesAt = len(*ipPolicies), time.Now()
}

// throttleMu guards BpfCtx.ThrottleObjs, swapped by ApplyPolicies, and the rule count
// and time of the loaded throttler, read by the shaping page.
var (
	throttleMu      sync.Mutex
	throttleRules   int
	throttleRulesAt time.Time
)

// buildThrottler renders the throttler for ipPolicies, compiles it with clang
// and loads it into the kernel (running the verifier) without attaching it.
func buildThrottler(ipPolicies *[]models.IpPolicy) (*throttleObjects, error) {
	targFile, err := os.CreateTemp("", "throttle-*.bpf.c")
	if err != nil {
		return nil, fmt.Errorf("could not create throttler file: %w", err)
	}
	defer os.Remove(targFile.Name())
	objFile, err := os.CreateTemp("", "throttle-*.o")
	if err != nil {
		return nil, fmt.Errorf("could not create throttler object file: %w", err)
	}
	defer os.Remove(objFile.Name())

	if err := rebuildBpf("throttle.bpf.c.tpl", targFile, objFile, ipPolicies); err != nil {
		return nil, err
	}
	spec, err := ebpf.LoadCollectionSpec(objFile.Name())
	if err != nil {
		return nil, fmt.Errorf("failed to load throttler spec: %w", err)
	}
	objs := &throttleObjects{}
	if err := spec.LoadAndAssign(objs, nil); err != nil {
		return nil, fmt.Errorf("verifier rejected throttler: %w", err)
	}
	return objs, nil
}

func renderThrottler(w io.Writer, tplfile string, ipPolicies *[]models.IpPolicy) error {
	tpl := template.Must(template.ParseFS(EmbedThrottlerCode, tplfile))
	return tpl.Execute(w, struct{ IpPolicies *[]models.IpPolicy }{ipPolicies})
}

func rebuildBpf(tplfile string, target *os.File, objFile *os.File, ipPolicies *[]models.IpPolicy) error {
	log.Println("Rebuilding with ", len(*ipPolicies), " throttle targets from policy database")
	log.Println("Temporary file ", target.Name())
	if err := renderThrottler(target, tplfile, ipPolicies); err != nil {
		return fmt.Errorf("failed to render throttler: %w", err)
	}
	// FIXME There surely must be a better way of doing this dynamically
	cmd := exec.Command(
		"clang", "-g", "-O2",
		// asm/ UAPI headers live in the multiarch folder: aarch64, armv7 (native armhf or the
		// armel cross package), plus x86_64 for dev machines
		"-I/usr/include/aarch64-linux-gnu", "-I/usr/include/arm-linux-gnueabihf", "-I/usr/arm-linux-gnueabi/include", "-I/usr/include/x86_64-linux-gnu",
		"-Wall", "-target", "bpf", "-c", target.Name(), "-o", objFile.Name(),
	)
	cmd.Dir = os.TempDir()
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to compile throttler eBPF: %w\n%s", err, out)
	}
	log.Println("Compilation output: ", string(out))
	return nil
}

// reattachThrottler replaces the throttle filter on the LAN HTB root with objs.
func reattachThrottler(objs *throttleObjects, ifname string) {
	log.Println("(Re)attaching throttler BPF to if ", ifname)
	link, err := netlink.LinkByName(ifname)
	if err != nil {
		log.Fatal(err)
	}
	htbRoot := netlink.MakeHandle(1, 0)
	filters, err := netlink.FilterList(link, htbRoot)
	if err != nil {
		log.Fatal("could not list tc filters ", err)
	}
	for _, f := range filters {
		if bf, ok := f.(*netlink.BpfFilter); ok && bf.Name == "throttle" {
			netlink.FilterDel(bf)
		}
	}
	if BpfCtx.ThrottleObjs != nil {
		BpfCtx.ThrottleObjs.Close()
	}
	BpfCtx.ThrottleObjs = objs
	attachFilter(ifname, htbRoot, objs.Prog, "throttle", htbRoot)
}

// attachFilter attaches prog as a direct-action tc filter on ifname under parent
// (a clsact hook or the HTB root). classID 0 leaves the filter's class unset.
func attachFilter(ifname string, parent uint32, prog *ebpf.Program, name string, classID uint32) {
	log.Println("Attaching ", name, " BPF to if ", ifname, " parent ", netlink.HandleStr(parent))
	link, err := netlink.LinkByName(ifname)
	if err != nil {
		log.Fatal(err)
	}
	filter := &netlink.BpfFilter{
		FilterAttrs: netlink.FilterAttrs{
			LinkIndex: link.Attrs().Index,
			Parent:    parent,
			Protocol:  unix.ETH_P_ALL,
		},
		Fd:           prog.FD(),
		Name:         name,
		DirectAction: true,
		ClassId:      classID,
	}
	if err := netlink.FilterAdd(filter); err != nil {
		log.Fatalln("could not attach filter for ", name, " on ", ifname, ": ", err)
	}
}

func pollDnsResponses() {
	// Per CPU. Samples are ~512 bytes, so a single page held only ~7 answers and dropped
	// bursts (a page load fires dozens of lookups). 32 pages holds ~250, at 128 KiB per CPU.
	reader, err := perf.NewReader(BpfCtx.DnspktObjs.DnsArr, 32*os.Getpagesize())
	if err != nil {
		log.Fatalln("failed to initialize perf buffer reader ", err)
	}
	// Poll the event buffer and parse records
	for {
		event, err := reader.Read()
		if err != nil {
			log.Fatalln("failure on event read", err)
		}

		handleDnsPacket(event.RawSample)
	}
}

// handleDnsPacket stores the A records of a DNS payload captured by dnspkt.bpf.c under
// every name of the answer: the queried name and each CNAME in the chain, so a glob on
// e.g. www.youtube.com still matches after youtube-ui.l.google.com resolves to the IP.
func handleDnsPacket(raw []byte) {
	msg := new(dns.Msg)
	if err := msg.Unpack(raw); err != nil {
		log.Println("Error decoding DNS record:", err)
	}
	// A single query's answer section holds only its chain, so every owner name belongs to it
	names := make([]string, 0, len(msg.Answer)+1)
	for _, q := range msg.Question {
		names = append(names, q.Name)
	}
	for _, rr := range msg.Answer {
		if n := rr.Header().Name; !slices.Contains(names, n) {
			names = append(names, n)
		}
	}
	var ips []net.IP
	for _, rr := range msg.Answer {
		if a, ok := rr.(*dns.A); ok {
			ips = append(ips, a.A)
		}
	}
	if len(ips) == 0 {
		return
	}
	if err := database.AddDns(names, ips); err != nil {
		log.Println("failed to write to dns db", err)
	}
}
