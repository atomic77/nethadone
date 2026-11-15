package handlers

import (
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"os/exec"
	"text/template"

	"github.com/alecthomas/repr"
	"github.com/atomic77/nethadone/config"
	"github.com/atomic77/nethadone/database"
	"github.com/atomic77/nethadone/models"
	"github.com/cilium/ebpf"
	"github.com/cilium/ebpf/perf"
	tc "github.com/florianl/go-tc"
	"github.com/florianl/go-tc/core"
	"github.com/miekg/dns"
	"golang.org/x/sys/unix"

	"github.com/mdlayher/netlink"
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
	Tcnl *tc.Tc
	// bpf2go generated
	DnspktObjs     *dnspktObjects
	TrafficmonObjs *trafficmonObjects
	// Custom - avoids go:embed that prevents live-reload
	ThrottleObjs *throttleObjects
}

var BpfCtx BpfContext

func (objs *throttleObjects) Close() error {
	if err := objs.Prog.Close(); err != nil {
		return err
	}
	return nil
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
	attachDnsSniffer(config.Cfg.WanInterface, tc.HandleMinEgress)
	attachDnsSniffer(config.Cfg.WanInterface, tc.HandleMinIngress)
	attachTrafficMonitor(config.Cfg.LanInterface, tc.HandleMinEgress)

	// Rebuild with our default target filtering so there's something
	// attached on startup
	ipPolicies := []models.IpPolicy{
		{SrcIpAddr: "", DestIpAddr: "192,168,0,174", ClassId: 10},
	}
	ApplyPolicies(&ipPolicies)
}

func ApplyPolicies(ipPolicies *[]models.IpPolicy) {
	objs, err := buildThrottler(ipPolicies)
	if err != nil {
		log.Fatal(err)
	}
	reattachThrottler(objs, config.Cfg.LanInterface, tc.HandleMinEgress)
}

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

func cleanupThrottler(iface *net.Interface) {
	// Clean up any existing filters prior to rebuilding and attaching
	tcnl := getTcnl()
	msg := tc.Msg{
		Ifindex: uint32(iface.Index),
		Parent:  core.BuildHandle(0x1, 0x0),
		// Where did this info flag come from ?
		Info: 0x300,
	}
	tfilt, err := tcnl.Filter().Get(&msg)
	if err != nil {
		log.Fatal("could not get tc filter info ", err)
	}
	for _, tobj := range tfilt {
		if tobj.BPF != nil && tobj.BPF.Name != nil && *tobj.BPF.Name == "throttle" {
			tcnl.Filter().Delete(&tobj)
		}
	}
	if BpfCtx.ThrottleObjs != nil {
		BpfCtx.ThrottleObjs.Map.Close()
		BpfCtx.ThrottleObjs.Prog.Close()
		BpfCtx.ThrottleObjs.Close()
	}

}

func reattachThrottler(objs *throttleObjects, ifname string, direction uint32) {

	log.Println("(Re)attaching throttler BPF to if ", ifname, " direction ", direction)

	tcnl := getTcnl()
	iface, err := net.InterfaceByName(ifname)
	if err != nil {
		log.Fatal(err)
	}

	cleanupThrottler(iface)
	BpfCtx.ThrottleObjs = objs

	fd := uint32(BpfCtx.ThrottleObjs.Prog.FD())
	flags := uint32(0x1)
	clsId := core.BuildHandle(1, 0)

	// Create a tc/filter object that will attach the eBPF program to the qdisc/clsact.
	bpfName := "throttle"
	filter := tc.Object{
		Msg: tc.Msg{
			Family:  unix.AF_UNSPEC,
			Ifindex: uint32(iface.Index),
			Handle:  core.BuildHandle(0x0, 0x1),
			Parent:  core.BuildHandle(0x1, 0x0),
			Info:    0x300,
		},
		Attribute: tc.Attribute{
			Kind: "bpf",
			BPF: &tc.Bpf{
				FD:      &fd,
				Flags:   &flags,
				Name:    &bpfName,
				ClassID: &clsId,
			},
		},
	}
	log.Println("Created filter object: ", repr.String(filter))
	// Attach the tc/filter object with the eBPF program to the qdisc/clsact.
	if err := tcnl.Filter().Add(&filter); err != nil {
		log.Fatalln("could not attach filter for DNS sniffer", err)
		return
	}
}

func attachTrafficMonitor(ifname string, direction uint32) {

	log.Println("Attaching bandwidth monitor BPF to if ", ifname, " direction ", direction)

	tcnl := getTcnl()
	iface, err := net.InterfaceByName(ifname)
	if err != nil {
		log.Fatal(err)
	}
	fd := uint32(BpfCtx.TrafficmonObjs.TrafficMon.FD())
	flags := uint32(0x1)

	// Create a tc/filter object that will attach the eBPF program to the qdisc/clsact.
	filter := tc.Object{
		Msg: tc.Msg{
			Family:  unix.AF_UNSPEC,
			Ifindex: uint32(iface.Index),
			Handle:  0,
			Parent:  core.BuildHandle(tc.HandleRoot, direction),
			Info:    0x300,
		},
		Attribute: tc.Attribute{
			Kind: "bpf",
			BPF: &tc.Bpf{
				FD:    &fd,
				Flags: &flags,
			},
		},
	}
	// Attach the tc/filter object with the eBPF program to the qdisc/clsact.
	if err := tcnl.Filter().Add(&filter); err != nil {
		log.Fatalln("could not attach filter for DNS sniffer", err)
		return
	}
}

func attachDnsSniffer(ifname string, direction uint32) {

	log.Println("Attaching DNS sniffer BPF to if ", ifname, " direction ", direction)

	tcnl := getTcnl()
	iface, err := net.InterfaceByName(ifname)
	if err != nil {
		log.Fatal(err)
	}
	fd := uint32(BpfCtx.DnspktObjs.UdpDnsSniff.FD())
	flags := uint32(0x1)

	// Create a tc/filter object that will attach the eBPF program to the qdisc/clsact.
	filter := tc.Object{
		Msg: tc.Msg{
			Family:  unix.AF_UNSPEC,
			Ifindex: uint32(iface.Index),
			Handle:  0,
			Parent:  core.BuildHandle(tc.HandleRoot, direction),
			Info:    0x300,
		},
		Attribute: tc.Attribute{
			Kind: "bpf",
			BPF: &tc.Bpf{
				FD:    &fd,
				Flags: &flags,
			},
		},
	}
	// Attach the tc/filter object with the eBPF program to the qdisc/clsact.
	if err := tcnl.Filter().Add(&filter); err != nil {
		log.Fatalln("could not attach filter for DNS sniffer", err)
		return
	}
	go pollDnsResponses()
}

func pollDnsResponses() {
	reader, err := perf.NewReader(BpfCtx.DnspktObjs.DnsArr, 4096) // what is a reasonable buffer size ??
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

// handleDnsPacket stores the A records of a DNS payload captured by dnspkt.bpf.c
func handleDnsPacket(raw []byte) {
	msg := new(dns.Msg)
	if err := msg.Unpack(raw); err != nil {
		log.Println("Error decoding DNS record:", err)
	}
	for _, resp := range msg.Answer {
		a, ok := resp.(*dns.A)
		if ok {
			if err := database.AddDns(a.Header().Name, &a.A); err != nil {
				log.Println("failed to write to dns db", err)
			}
		}
	}
}

func getTcnl() *tc.Tc {
	if BpfCtx.Tcnl != nil {
		return BpfCtx.Tcnl
	}

	tcnl, err := tc.Open(&tc.Config{})
	if err != nil {
		log.Fatal("Failed to get tc handle", err)
	}

	err = tcnl.SetOption(netlink.ExtendedAcknowledge, true)
	if err != nil {
		log.Fatal("could not set option ExtendedAcknowledge ", err)
	}
	BpfCtx.Tcnl = tcnl
	return BpfCtx.Tcnl
}
