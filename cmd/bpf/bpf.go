package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"os/exec"
	"text/template"

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
	// Map  *ebpf.Map     `ebpf:"my_map"`
	Prog *ebpf.Program `ebpf:"tc_ingress"`
}

// Comma separated, eg. 192,168,0,14
type filtParams struct {
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
		log.Panic(err)
	}

	return prog

}

func compileBpf(tplfile string, target string, p *filtParams) {
	// Uff. There must be a better way that avoids ebpf2go.
	// FIXME when root compiles the bpf code, it fails to load for some reason, so su back
	// to regular user acct which produces different object code

	// tpl.Execute()
	cfile := "/tmp/mybpfprog.c"
	f, err := os.Create(cfile)
	if err != nil {
		log.Panic("failed to create rendered file ", err)
	}
	// tpl := template.Must(template.New("t").ParseFiles(tplfile))
	tpl := template.Must(template.ParseFiles(tplfile))
	// err = tpl.ExecuteTemplate(f, tplfile, p)
	err = tpl.Execute(f, p)
	if err != nil {
		log.Panic("failed to render file ", err)
	}

	out, err := exec.Command(
		"su", "atomic", "-c",
		// "clang", "-g", "-O2", "-I/usr/include/aarch64-linux-gnu", "-Wall", "-target", "bpf",
		// "-c", file, "-o", target,
		`clang -g -O2 -I/usr/include/aarch64-linux-gnu -Wall -target bpf -c `+cfile+" -o "+target,
	).CombinedOutput()
	if err != nil {
		log.Panic("failed to compile ebpf prog, out: ", string(out), " err: ", err)
	}
	log.Println("Compilation output: ", string(out))
}

func attachQdiscToIface(tcnl *tc.Tc, iface *net.Interface) {

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
			log.Panic("couldn't replace qdisc ", err)
		}
	}
}
func main() {
	// Cli tool to use pure go for manipulating TC tables

	src := flag.String("src", "192,168,0,108", "Source ip")
	dest := flag.String("dest", "192,168,0,14", "Dest ip")
	delay := flag.Int("delayMs", 10, "Delay in ms")
	flag.Parse()

	fparams := filtParams{
		SrcIpAddr:  *src,
		DestIpAddr: *dest,
		DelayMs:    *delay,
	}

	tcnl, err := tc.Open(&tc.Config{})
	if err != nil {
		log.Panic("Failed to get tc handle", err)
	}
	defer func() {
		if err := tcnl.Close(); err != nil {
			log.Printf("could not close rtnetlink socket: %v\n", err)
		}
	}()

	err = tcnl.SetOption(netlink.ExtendedAcknowledge, true)
	if err != nil {
		fmt.Fprintf(os.Stderr, "could not set option ExtendedAcknowledge: %v\n", err)
		return
	}

	// get all the qdiscs from all interfaces
	qdiscs, err := tcnl.Qdisc().Get()
	if err != nil {
		log.Printf("could not get qdiscs: %v\n", err)
		return
	}

	iface, err := net.InterfaceByName("enx3a406b1307a9")
	if err != nil {
		log.Panic(err)
	}

	// Try to use an existing qdisc; o/w create
	// qdisc := tc.Object{}
	// var qdisc tc.Object
	for _, v := range qdiscs {
		if v.Ifindex == uint32(iface.Index) {
			// Got a match
			// qdisc = v
			log.Println("Matching qdisc: ", v.Ifindex, v.Info, v.Attribute)
		}
	}

	compileBpf(
		"/home/atomic/chiron/ebpf/tcfilt.bpf.c.tpl",
		"/home/atomic/chiron/ebpf/tcfilt.o",
		&fparams,
	)

	// prog := getSampleProg()
	spec, err := ebpf.LoadCollectionSpec("ebpf/tcfilt.o")
	if err != nil {
		log.Panic("failed to load spec ", err)
	}

	var objs filtObjs
	if err := spec.LoadAndAssign(&objs, nil); err != nil {
		log.Panic("failed to load and assign prog spec", err)
	}

	fd := uint32(objs.Prog.FD())
	flags := uint32(0x1)

	// Create a tc/filter object that will attach the eBPF program to the qdisc/clsact.
	filter := tc.Object{
		Msg: tc.Msg{
			Family:  unix.AF_UNSPEC,
			Ifindex: uint32(iface.Index),
			Handle:  0,
			Parent:  core.BuildHandle(tc.HandleRoot, tc.HandleMinEgress),
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
		fmt.Fprintf(os.Stderr, "could not attach filter for eBPF program: %v\n", err)
		return
	}
}
