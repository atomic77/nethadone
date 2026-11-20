package handlers

import (
	"os"
	"testing"

	"github.com/alecthomas/repr"
	"github.com/vishvananda/netlink"
)

func TestQdiscList(t *testing.T) {
	skipUnlessRoot(t)

	qdiscs, err := netlink.QdiscList(nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, q := range qdiscs {
		t.Log(repr.String(q))
	}
}

func TestClassList(t *testing.T) {
	skipUnlessRoot(t)

	link, err := netlink.LinkByName("eth1")
	if err != nil {
		t.Skip("no eth1: ", err)
	}
	classes, err := netlink.ClassList(link, 0)
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range classes {
		t.Log(repr.String(c))
	}
}

func skipUnlessRoot(t *testing.T) {
	t.Helper()
	if os.Geteuid() != 0 {
		t.Skip("needs root (go test -exec sudo)")
	}
}
