package handlers

import (
	"log"
	"net"
	"os"
	"os/exec"
	"text/template"

	"github.com/alecthomas/repr"
	"github.com/atomic77/nethadone/database"
	"github.com/atomic77/nethadone/models"
	"github.com/cilium/ebpf"
	"github.com/cilium/ebpf/perf"
	tc "github.com/florianl/go-tc"
	"github.com/florianl/go-tc/core"
	"github.com/miekg/dns"
	"golang.org/x/sys/unix"

	// "github.com/vishvananda/netlink"
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

func Initialize(cfg *Config) {

	config = *cfg
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

	attachDnsSniffer(config.LanInterface, tc.HandleMinEgress)
	attachDnsSniffer(config.LanInterface, tc.HandleMinIngress)
	attachTrafficMonitor(config.LanInterface, tc.HandleMinEgress)

	// Rebuild with our default target filtering so there's something
	// attached on startup
	ipPolicies := []models.IpPolicy{
		{SrcIpAddr: "", DestIpAddr: "192,168,0,174", ClassId: 10},
	}
	ApplyPolicies(&ipPolicies)
}

func ApplyPolicies(ipPolicies *[]models.IpPolicy) {
	rebuildBpf("ebpf/throttle.bpf.c.tpl", "ebpf/throttle.bpf.c", ipPolicies)
	reattachThrottler(config.LanInterface, tc.HandleMinEgress)
}

func rebuildBpf(tplfile string, target string, ipPolicies *[]models.IpPolicy) {
	log.Println("Rebuilding with ", len(*ipPolicies), " throttle targets from policy database")

	f, err := os.Create(target)
	if err != nil {
		log.Fatal("failed to create rendered file ", err)
	}
	tpl := template.Must(template.ParseFiles(tplfile))
	type fdata struct {
		IpPolicies *[]models.IpPolicy
	}
	err = tpl.Execute(f, fdata{IpPolicies: ipPolicies})
	if err != nil {
		log.Fatal("failed to render file ", err)
	}
	// FIXME There surely must be a better way of doing this dynamically
	cmd := exec.Command("make", "build-throttler")
	cmd.Dir = "ebpf/"
	out, err := cmd.CombinedOutput()
	if err != nil {
		log.Fatal("failed to rebuild throttler eBPF, out: ", string(out), " err: ", err)
	}
	log.Println("Compilation output: ", string(out))

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
			log.Println("found prior throttler filter, removing")
			repr.Println(tobj)
			tcnl.Filter().Delete(&tobj)
		}
	}
	if BpfCtx.ThrottleObjs != nil {
		log.Println("Found existing Throttler BPF; closing")
		// TODO Confirm what here is necessary
		BpfCtx.ThrottleObjs.Map.Close()
		BpfCtx.ThrottleObjs.Prog.Close()
		BpfCtx.ThrottleObjs.Close()
	}

}

func reattachThrottler(ifname string, direction uint32) {

	log.Println("(Re)attaching throttler BPF to if ", ifname, " direction ", direction)

	tcnl := getTcnl()
	iface, err := net.InterfaceByName(ifname)
	if err != nil {
		log.Fatal(err)
	}

	cleanupThrottler(iface)

	BpfCtx.ThrottleObjs = &throttleObjects{}
	spec, err := ebpf.LoadCollectionSpec("ebpf/throttle.o")
	if err != nil {
		log.Fatal("failed to load spec ", err)
	}

	if err = spec.LoadAndAssign(BpfCtx.ThrottleObjs, nil); err != nil {
		log.Fatal("failed to load and assign prog spec", err)
	}

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

/*
	func CompileBpf(tplfile string, target string, ips *[]models.IpPolicy) {
		// Used for dynamic recompilation of tcfilt bpf program that will be
		// continuously rendered based on the template and recompiled.
		// It seems that this can't easily be done with bpf2go since the binary code
		// gets compiled into the binary
		// DNS packet sniffing BPF program that is static has been moved to bpf2go already

		cfile := "/tmp/mybpfprog.c"
		f, err := os.Create(cfile)
		if err != nil {
			log.Fatal("failed to create rendered file ", err)
		}
		tpl := template.Must(template.ParseFiles(tplfile))
		type fdata struct {
			IpPolicies *[]models.IpPolicy
		}
		err = tpl.Execute(f, fdata{IpPolicies: ips})
		if err != nil {
			log.Fatal("failed to render file ", err)
		}

		out, err := exec.Command(
			// "su", "atomic", "-c",
			// "clang", "-g", "-O2", "-I/usr/include/aarch64-linux-gnu", "-Wall", "-target", "bpf",
			// "-c", file, "-o", target,
			// `clang -g -O2 -I/usr/include/aarch64-linux-gnu -Wall -target bpf -c ` + cfile + " -o " + target,
			"clang", "-g", "-O2", "-I/usr/include/aarch64-linux-gnu", "-Wall", "-target", "bpf",
			"-c", cfile, "-o", target,
		).CombinedOutput()
		if err != nil {
			log.Fatal("failed to compile ebpf prog, out: ", string(out), " err: ", err)
		}
		log.Println("Compilation output: ", string(out))
	}
*/
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

		msg := new(dns.Msg)
		if err := msg.Unpack(event.RawSample); err != nil {
			log.Println("Error decoding DNS record:", err)
		}

		for _, resp := range msg.Answer {
			a, ok := resp.(*dns.A)
			if ok {
				err = database.AddDns(a.Header().Name, &a.A)
				log.Println("DNS A response:", a.Header().Name, a.A.String())
				if err != nil {
					log.Println("failed to write to dns db", err)
				}
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
	// Need to defer this to shutdown or another more appropriate time
	// defer func() {
	// 	log.Println("Cleaning up tcnl")
	// 	if err := tcnl.Close(); err != nil {
	// 		log.Printf("could not close rtnetlink socket: %v\n", err)
	// 	}
	// }()

	err = tcnl.SetOption(netlink.ExtendedAcknowledge, true)
	if err != nil {
		log.Fatal("could not set option ExtendedAcknowledge ", err)
	}
	BpfCtx.Tcnl = tcnl
	return BpfCtx.Tcnl
}
