/* Simple program to test polling and parsing of DNS buffers dropped into perf array
by pkt.bpf.c. To be integrated into main website  */

package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"os"

	"github.com/alecthomas/repr"
	"github.com/atomic77/nethadone/handlers"
	"github.com/cilium/ebpf/perf"
	tc "github.com/florianl/go-tc"
	"github.com/florianl/go-tc/core"
	"github.com/miekg/dns"
	"golang.org/x/sys/unix"

	"github.com/mdlayher/netlink"
)

/*
type bpfObj struct {
	Map  *ebpf.Map     `ebpf:"dns_arr"`
	Prog *ebpf.Program `ebpf:"handle_udp"`
}

func (objs *bpfObj) Close() error {
	if err := objs.Prog.Close(); err != nil {
		return err
	}
	return nil
}
*/

func main() {
	// Cli tool to use pure go for manipulating TC tables

	ifname := flag.String("interface", "eth0", "Interface to attach to")
	dir := flag.String("direction", "ingress", "Ingress or Egress")
	// delay := flag.Int("delayMs", 10, "Delay in ms")
	flag.Parse()

	// fparams := filtParams{
	// 	SrcIpAddr:  *src,
	// 	DestIpAddr: *dest,
	// 	DelayMs:    *delay,
	// }

	tcnl, err := tc.Open(&tc.Config{})
	if err != nil {
		log.Fatal("Failed to get tc handle", err)
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

	iface, err := net.InterfaceByName(*ifname)
	if err != nil {
		log.Fatal("could not open interface ", err)
	}

	// Try to use an existing qdisc; o/w create
	// qdisc := tc.Object{}
	// var qdisc tc.Object
	for _, v := range qdiscs {
		if v.Ifindex == uint32(iface.Index) {
			log.Println("Matching qdisc: ", v.Ifindex, v.Info, v.Attribute)
		}
	}

	handlers.Initialize(*ifname)

	fd := uint32(handlers.BpfCtx.DnspktObjs.HandleUdp.FD())
	flags := uint32(0x1)

	dirFlag := tc.HandleMinIngress
	if *dir == "Egress" {
		dirFlag = tc.HandleMinEgress
	}
	// Create a tc/filter object that will attach the eBPF program to the qdisc/clsact.
	filter := tc.Object{
		Msg: tc.Msg{
			Family:  unix.AF_UNSPEC,
			Ifindex: uint32(iface.Index),
			Handle:  0,
			Parent:  core.BuildHandle(tc.HandleRoot, dirFlag),
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

	reader, err := perf.NewReader(handlers.BpfCtx.DnspktObjs.DnsArr, 4096) // what is a reasonable buffer size ??
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
			log.Fatalln("Error decoding DNS record:", err)
		}
		repr.Println(msg)

	}
}
