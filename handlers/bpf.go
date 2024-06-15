package handlers

import (
	"log"
	"net"
	"os"
	"os/exec"
	"text/template"

	"github.com/alecthomas/repr"
	"github.com/atomic77/nethadone/database"
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
//go:generate go run github.com/cilium/ebpf/cmd/bpf2go throttle ../ebpf/throttle.bpf.c - -O2  -Wall -Werror -Wno-address-of-packed-member

type filtObjs struct {
	// TODO We are still dependent on direct clang compilation for the tcfilt bpf
	// programs; investigate if this can be moved to bpf2go as with the dns sniffer
	Map  *ebpf.Map     `ebpf:"src_dest_bytes"`
	Prog *ebpf.Program `ebpf:"tc_ingress"`
}

type BpfContext struct {
	Tcnl *tc.Tc
	// TODO Remove old TcFilterObjs when this is fully moved to bpf2go
	TcFilterObjs *filtObjs
	// bpf2go generated
	DnspktObjs     *dnspktObjects
	TrafficmonObjs *trafficmonObjects
	ThrottleObjs   *throttleObjects
}

var BpfCtx BpfContext

// IPaddr is comma separated due to use of IP_ADDRESS macro in bpf code, eg. 192,168,0,14
type FiltParams struct {
	SrcIpAddr  string
	DestIpAddr string
	DelayMs    int
}

func (objs *filtObjs) Close() error {
	if err := objs.Prog.Close(); err != nil {
		return err
	}
	return nil
}

func InitializeBpf(ifname string) {
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

	attachDnsSniffer("eth0", tc.HandleMinEgress)
	attachDnsSniffer("eth1", tc.HandleMinEgress)
	attachDnsSniffer("eth0", tc.HandleMinIngress)
	attachDnsSniffer("eth1", tc.HandleMinIngress)
	attachTrafficMonitor("eth0", tc.HandleMinIngress)
	attachTrafficMonitor("eth0", tc.HandleMinEgress)
}

func rebuildBpf(tplfile string, target string, fparams *[]FiltParams) {
	log.Println("Rebuilding with ", len(*fparams), " throttle targets")

	f, err := os.Create(target)
	if err != nil {
		log.Fatal("failed to create rendered file ", err)
	}
	tpl := template.Must(template.ParseFiles(tplfile))
	type fdata struct {
		FiltParams *[]FiltParams
	}
	err = tpl.Execute(f, fdata{FiltParams: fparams})
	if err != nil {
		log.Fatal("failed to render file ", err)
	}
	// FIXME There surely must be a better way of doing this dynamically
	cmd := exec.Command(
		"go", "run", "github.com/cilium/ebpf/cmd/bpf2go",
		"-go-package", "handlers",
		"throttle",
		"../ebpf/throttle.bpf.c", "-O2", "-Wall", "-Werror",
		"-Wno-address-of-packed-member",
	)
	cmd.Dir = "handlers/"
	out, err := cmd.CombinedOutput()
	if err != nil {
		log.Fatal("failed to run bpf2go, out: ", string(out), " err: ", err)
	}
	log.Println("Compilation output: ", string(out))

}

func cleanupThrottler(iface *net.Interface, direction uint32) {
	// Clean up any existing filters prior to rebuilding and attaching
	tcnl := getTcnl()
	msg := tc.Msg{
		Ifindex: uint32(iface.Index),
		Parent:  core.BuildHandle(tc.HandleRoot, direction),
		Info:    0x300,
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

	cleanupThrottler(iface, direction)

	BpfCtx.ThrottleObjs = &throttleObjects{}
	err = loadThrottleObjects(BpfCtx.ThrottleObjs, nil)
	if err != nil {
		log.Fatalln("failed to reload throttling bpf program ", err)
	}

	fd := uint32(BpfCtx.ThrottleObjs.Throttle.FD())
	flags := uint32(0x1)

	// Create a tc/filter object that will attach the eBPF program to the qdisc/clsact.
	bpfName := "throttle"
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
				Name:  &bpfName,
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

func compileBpf(tplfile string, target string, fparams *[]FiltParams) {
	// Used for dynamic recompilation of tcfilt bpf program that will be
	// continuously retemplated and recompiled.
	// Investigate how or if this can be accomplished with bpf2go
	// DNS packet sniffing BPF program that is static has been moved to bpf2go already

	cfile := "/tmp/mybpfprog.c"
	f, err := os.Create(cfile)
	if err != nil {
		log.Fatal("failed to create rendered file ", err)
	}
	tpl := template.Must(template.ParseFiles(tplfile))
	type fdata struct {
		FiltParams *[]FiltParams
	}
	err = tpl.Execute(f, fdata{FiltParams: fparams})
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

func attachQdiscToIface(tcnl *tc.Tc, iface *net.Interface) {
	// TODO Unused, assume the qdisc is created

	qdisc := tc.Object{
		Msg: tc.Msg{
			Family:  unix.AF_UNSPEC,
			Ifindex: uint32(iface.Index),
			Handle:  core.BuildHandle(tc.HandleRoot, 0x0000),
			Parent:  tc.HandleMinEgress,
			Info:    0,
		},
		Attribute: tc.Attribute{
			Kind: "clsact",
			// Kind: "fq",
		},
	}

	if err := tcnl.Qdisc().Add(&qdisc); err != nil {
		log.Println("couldn't add qdisc ", err)
		if err := tcnl.Qdisc().Replace(&qdisc); err != nil {
			log.Fatal("couldn't replace qdisc ", err)
		}
	}
}
func getQdiscInfo() {

	// get all the qdiscs from all interfaces
	/*
		qdiscs, err := tcnl.Qdisc().Get()
		if err != nil {
			log.Printf("could not get qdiscs: %v\n", err)
			return
		}
		for _, v := range qdiscs {
			if v.Ifindex == uint32(iface.Index) {
				// Got a match
				// qdisc = v
				log.Println("Matching qdisc: ", v.Ifindex, v.Info, v.Attribute)
			}
		}
	*/
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

func redeployTcFilt(fparams *[]FiltParams) {

	tcnl := getTcnl()

	iface, err := net.InterfaceByName("eth0")
	if err != nil {
		log.Fatal(err)
	}

	compileBpf(
		"ebpf/tcfilt.bpf.c.tpl",
		"/tmp/tcfilt.o",
		fparams,
	)

	spec, err := ebpf.LoadCollectionSpec("/tmp/tcfilt.o")
	if err != nil {
		log.Fatal("failed to load spec ", err)
	}

	if BpfCtx.TcFilterObjs != nil {
		BpfCtx.TcFilterObjs.Prog.Close()
	}
	tcFilt := &filtObjs{}
	if err := spec.LoadAndAssign(tcFilt, nil); err != nil {
		log.Fatal("failed to load and assign prog spec", err)
	}
	BpfCtx.TcFilterObjs = tcFilt

	log.Println("bpf - tcfilter objs ref", repr.String(BpfCtx.TcFilterObjs))
	// fd := uint32(BpfCtx.TcFilterObjs.Prog.FD())
	fd := uint32(tcFilt.Prog.FD())
	flags := uint32(0x1)

	// Create a tc/filter object that will attach the eBPF program to the qdisc/clsact.
	filter := tc.Object{
		Msg: tc.Msg{
			Family:  unix.AF_UNSPEC,
			Ifindex: uint32(iface.Index),
			Handle:  0,
			// Parent:  core.BuildHandle(tc.HandleRoot, tc.HandleMinEgress),
			Parent: core.BuildHandle(tc.HandleRoot, tc.HandleMinIngress),
			Info:   0x300,
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
		log.Fatal("could not attach filter for eBPF program: ", err)
		return
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
