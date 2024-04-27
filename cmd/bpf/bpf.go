package main

import (
	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"
)

func main() {
	// Cli tool to use pure go for manipulating TC tables
	h, err := netlink.NewHandle(unix.NETLINK_ROUTE)

	/*
		q := netlink.Qdisc{};
		qa := netlink.QdiscAttrs{};
	*/

}
