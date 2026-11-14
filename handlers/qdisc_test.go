package handlers

import (
	"net"
	"os"
	"testing"

	"github.com/alecthomas/repr"
	"github.com/florianl/go-tc"
	"golang.org/x/sys/unix"
)

func TestQdiscList(t *testing.T) {
	skipUnlessRoot(t)

	tcnl := getTcnl()

	qobj, _ := tcnl.Qdisc().Get()
	for _, c := range qobj {

		println(repr.String(c))
	}
}

func TestClassList(t *testing.T) {
	skipUnlessRoot(t)

	tcnl := getTcnl()

	iface, err := net.InterfaceByName("eth1")
	if err != nil {
		t.Skip("no eth1: ", err)
	}
	cobj, _ := tcnl.Class().Get(&tc.Msg{
		Family:  unix.AF_UNSPEC,
		Ifindex: uint32(iface.Index),
	})
	for _, c := range cobj {

		println(repr.String(c))
	}
}

func skipUnlessRoot(t *testing.T) {
	t.Helper()
	if os.Geteuid() != 0 {
		t.Skip("needs root (go test -exec sudo)")
	}
}
