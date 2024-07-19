package handlers

import (
	"log"
	"net"

	"github.com/alecthomas/repr"
	"github.com/atomic77/nethadone/config"
	tc "github.com/florianl/go-tc"
	"github.com/florianl/go-tc/core"
	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"
)

func uint64ptr(v uint64) *uint64 {
	return &v
}

const MBit = 1000 * 1000
const KBit = 1000
const Ms = 1000

type leafConfig struct {
	Rate     uint64
	Ceil     uint64
	Delay    uint32
	Variance uint32
	Kind     string
	Iface    int
}

// Netlink seems to be causing fewer difficulties in converting the
// equivalent `tc` commands than go-tc.
func createQdiscs() {

	/*
		Create LAN-side qdiscs where most of our qdisc setup will be

		Root nodes equivalent to:
		sudo tc qdisc add dev lan0 root handle 1: htb default 10
		sudo tc class add dev lan0 parent 1: classid 1:1 htb rate 100Mbit ceil 100Mbit
	*/

	log.Println("Checking LAN interface ", config.Cfg.LanInterface)
	iface, err := net.InterfaceByName(config.Cfg.LanInterface)
	if err != nil {
		log.Fatal("could not open interface ", err)
	}

	err = netlink.QdiscDel(
		&netlink.GenericQdisc{QdiscAttrs: netlink.QdiscAttrs{LinkIndex: iface.Index, Parent: netlink.HANDLE_ROOT}},
	)

	if err != nil {
		log.Println("could not find root qdisc to delete: ", err)
	}

	log.Println("Creating LAN Qdiscs and classes")
	rootQdisc := netlink.NewHtb(
		netlink.QdiscAttrs{LinkIndex: iface.Index, Handle: netlink.MakeHandle(1, 0), Parent: netlink.HANDLE_ROOT},
	)
	rootQdisc.Defcls = 0x10

	err = netlink.QdiscAdd(rootQdisc)
	if err != nil {
		log.Fatal("couldn't add root qdisc: ", err)
	}

	rootClsAttr := netlink.NewHtbClass(
		netlink.ClassAttrs{LinkIndex: iface.Index, Handle: netlink.MakeHandle(1, 1), Parent: netlink.MakeHandle(1, 0)},
		netlink.HtbClassAttrs{Rate: 100 * MBit, Ceil: 100 * MBit},
	)
	err = netlink.ClassAdd(rootClsAttr)
	if err != nil {
		log.Fatal("couldn't add root class: ", err)
	}

	leaves := generateQdiscLeafNodes(iface)

	createLeafQdiscs(&leaves)

	recreateBpfQdisc(iface)

	// Prepare BPF clsact for WAN interface
	log.Println("Checking WAN interface ", config.Cfg.WanInterface)
	iface, err = net.InterfaceByName(config.Cfg.WanInterface)
	if err != nil {
		log.Fatal("could not open interface ", err)
	}
	recreateBpfQdisc(iface)

}

func recreateBpfQdisc(iface *net.Interface) {
	// Deleting the clsact qdisc on startup ensures that any child filters, ebpf progs are cleaned up
	bpfQdisc := &netlink.Clsact{
		netlink.QdiscAttrs{LinkIndex: iface.Index, Handle: netlink.MakeHandle(0xffff, 0), Parent: netlink.HANDLE_CLSACT},
	}

	netlink.QdiscDel(
		&netlink.GenericQdisc{QdiscAttrs: netlink.QdiscAttrs{LinkIndex: iface.Index, Parent: netlink.HANDLE_CLSACT}},
	)

	err := netlink.QdiscAdd(bpfQdisc)
	if err != nil {
		log.Fatal("could not add clsact qdisc on ", iface, ": ", err)
	}
}

func generateQdiscLeafNodes(iface *net.Interface) []leafConfig {
	/* Create child nodes roughly equivalent to the following

	sudo tc class add dev lan0 parent 1:1 classid 1:10 htb rate 95Mbit ceil 100Mbit
	sudo tc class add dev lan0 parent 1:1 classid 1:20 htb rate 500kbit ceil 500kbit
	sudo tc class add dev lan0 parent 1:1 classid 1:30 htb rate 50kbit ceil 50kbit

	sudo tc qdisc add dev lan0 parent 1:10 handle 10: pfifo
	sudo tc qdisc add dev lan0 parent 1:20 handle 20: pfifo
	sudo tc qdisc add dev lan0 parent 1:30 handle 30: netem delay 250ms 50ms

	Create a successively slower set of bandwidth classes based on our configuration;
	The first will be the 'open' pipe based on the configured bandwidth for the WAN
	*/
	leaves := []leafConfig{
		{Rate: 95 * MBit, Ceil: 100 * MBit, Kind: "prio", Iface: iface.Index},
	}

	for i := 1; i <= config.Cfg.NumQdiscClasses; i++ {

		factor := float64(i) / float64(config.Cfg.NumQdiscClasses)
		scaledRate := (1-factor)*float64((config.Cfg.StartRateKbs-config.Cfg.MinRateKbs)) + float64(config.Cfg.MinRateKbs)
		scaledRate *= KBit
		scaledDelay := factor * float64(config.Cfg.MaxDelayMs) * Ms
		lc := leafConfig{
			Rate:     uint64(scaledRate),
			Ceil:     uint64(scaledRate),
			Kind:     "netem",
			Delay:    uint32(scaledDelay),
			Variance: uint32(scaledDelay / 10),
			Iface:    iface.Index,
		}
		log.Println(repr.String(lc))
		leaves = append(leaves, lc)
	}

	return leaves
}

func createLeafQdiscs(leaves *[]leafConfig) {

	for idx, leaf := range *leaves {
		log.Println(repr.String(leaf))
		handleId := (uint16)(idx+1) * 16
		cls := netlink.NewHtbClass(
			netlink.ClassAttrs{LinkIndex: leaf.Iface, Handle: netlink.MakeHandle(1, handleId), Parent: netlink.MakeHandle(1, 1)},
			netlink.HtbClassAttrs{Rate: leaf.Rate, Ceil: leaf.Rate},
		)

		err := netlink.ClassAdd(cls)
		if err != nil {
			log.Fatal("couldn't add leaf class for ", repr.String(leaf), ":", err)
		}

		if leaf.Kind == "netem" {
			qd := netlink.NewNetem(
				netlink.QdiscAttrs{LinkIndex: leaf.Iface, Handle: netlink.MakeHandle(handleId, 0), Parent: netlink.MakeHandle(1, handleId)},
				netlink.NetemQdiscAttrs{Latency: leaf.Delay, Jitter: leaf.Variance},
			)
			err = netlink.QdiscAdd(qd)
		} else {
			qd := netlink.NewPrio(
				netlink.QdiscAttrs{LinkIndex: leaf.Iface, Handle: netlink.MakeHandle(handleId, 0), Parent: netlink.MakeHandle(1, handleId)},
			)
			err = netlink.QdiscAdd(qd)
		}

		if err != nil {
			log.Fatal("couldn't add leaf qdisc for ", repr.String(leaf), ":", err)
		}
	}
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
		},
	}

	if err := tcnl.Qdisc().Add(&qdisc); err != nil {
		log.Println("couldn't add qdisc ", err)
		if err := tcnl.Qdisc().Replace(&qdisc); err != nil {
			log.Fatal("couldn't replace qdisc ", err)
		}
	}
}
