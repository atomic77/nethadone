package handlers

import (
	"log"

	"github.com/gofiber/fiber/v2"
	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"

	"net"
)

func Index(c *fiber.Ctx) error {
	// Render index
	return c.Render("index", fiber.Map{
		"Title": "Hello, embedded template world!",
	}, "layouts/base")
}

func Interfaces(c *fiber.Ctx) error {
	// Main page for interface list
	ifaces, err := net.Interfaces()
	if err != nil {
		log.Panic("Something weird when trying to list ifaces", err)
	}

	nh, err := netlink.NewHandle(unix.NETLINK_ROUTE)
	if err != nil {
		log.Panic("could not get netlink handle", err)
	}

	ll, err := nh.LinkList()
	if err != nil {
		log.Panic("could not get get link list", err)
	}

	return c.Render("interfaces", fiber.Map{
		"Title":      "Interfaces",
		"Interfaces": ifaces,
		"LinkList":   ll,
	}, "layouts/base")
}

func Devices(c *fiber.Ctx) error {
	// Render index
	interfaces := make([]string, 2)
	interfaces[0] = "asdf"
	interfaces[1] = "fdsa"
	return c.Render("devices", fiber.Map{
		"Title": "Interfaces",
		"Devices": fiber.Map{
			"Alias":  "laptop",
			"IPAddr": "192.168.0.2",
		},
	}, "layouts/base")
}

// NotFound returns custom 404 page
func NotFound(c *fiber.Ctx) error {
	return c.Status(404).SendFile("./static/private/404.html")
}
