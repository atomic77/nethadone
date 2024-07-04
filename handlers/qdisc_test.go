package handlers

import (
	"net"
	"testing"

	"github.com/alecthomas/repr"
	"github.com/florianl/go-tc"
	"golang.org/x/sys/unix"
)

func TestQdiscList(t *testing.T) {

	tcnl := getTcnl()

	qobj, _ := tcnl.Qdisc().Get()
	for _, c := range qobj {

		println(repr.String(c))
	}
}

func TestClassList(t *testing.T) {

	tcnl := getTcnl()

	iface, err := net.InterfaceByName("eth1")
	if err != nil {
		t.Fail()
	}
	cobj, _ := tcnl.Class().Get(&tc.Msg{
		Family:  unix.AF_UNSPEC,
		Ifindex: uint32(iface.Index),
	})
	for _, c := range cobj {

		println(repr.String(c))
	}
}
