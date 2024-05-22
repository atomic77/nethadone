package handlers

import (
	"log"
	"net"
	"os"
	"os/exec"
	"text/template"

	"github.com/alecthomas/repr"
	"github.com/cilium/ebpf"
	"github.com/cilium/ebpf/asm"
	tc "github.com/florianl/go-tc"
	"github.com/florianl/go-tc/core"
	"golang.org/x/sys/unix"

	// "github.com/vishvananda/netlink"
	"github.com/mdlayher/netlink"
)

type filtObjs struct {
	// TODO Add data structure in here
	// bpf2go can be used to automate and sync this, but for now
	// should be simple enough to maintain these links
	Map  *ebpf.Map     `ebpf:"src_dest_bytes"`
	Prog *ebpf.Program `ebpf:"tc_ingress"`
}

// Need to better understand why this is necessary to get the
// context to be shareable across calls
type BpfContext struct {
	Tcnl         *tc.Tc
	TcFilterObjs *filtObjs
}

var BpfCtx BpfContext

// Comma separated, eg. 192,168,0,14
type FiltParams struct {
	SrcIpAddr  string
	DestIpAddr string
	DelayMs    int
}

func (objs *filtObjs) Close() error {
	// if err := objs.Map.Close(); err != nil {
	// 	return err
	// }
	if err := objs.Prog.Close(); err != nil {
		return err
	}
	return nil
}

func getSampleProg() *ebpf.Program {

	// Load an example prog until we can figure out how to compile
	// in ours
	spec := ebpf.ProgramSpec{
		Name: "test",
		Type: ebpf.SchedCLS,
		Instructions: asm.Instructions{
			// Set exit code to 0
			asm.Mov.Imm(asm.R0, 0),
			asm.Return(),
		},
		License: "GPL",
	}

	prog, err := ebpf.NewProgram(&spec)
	if err != nil {
		log.Fatal(err)
	}

	return prog

}

func compileBpf(tplfile string, target string, fparams *[]FiltParams) {
	// Uff. There must be a better way that avoids ebpf2go.
	// FIXME when root compiles the bpf code, it fails to load for some reason, so su back
	// to regular user acct which produces different object code

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
		"su", "atomic", "-c",
		// "clang", "-g", "-O2", "-I/usr/include/aarch64-linux-gnu", "-Wall", "-target", "bpf",
		// "-c", file, "-o", target,
		`clang -g -O2 -I/usr/include/aarch64-linux-gnu -Wall -target bpf -c `+cfile+" -o "+target,
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
func redeployBpf(fparams *[]FiltParams) {
	// Cli tool to use pure go for manipulating TC tables

	tcnl := getTcnl()

	iface, err := net.InterfaceByName("eth0")
	if err != nil {
		log.Fatal(err)
	}

	compileBpf(
		"/home/atomic/nethadone/ebpf/tcfilt.bpf.c.tpl",
		"/home/atomic/nethadone/ebpf/tcfilt.o",
		fparams,
	)

	spec, err := ebpf.LoadCollectionSpec("ebpf/tcfilt.o")
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
