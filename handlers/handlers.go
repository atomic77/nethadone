package handlers

import (
	"log"
	"strconv"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/utils"
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

func Rules(c *fiber.Ctx) error {

	// Only one default rule for now
	fparams := make([]FiltParams, 1)
	fparams[0] = FiltParams{
		SrcIpAddr:  "192,168,0,108",
		DestIpAddr: "192,168,0,14",
		DelayMs:    10,
	}
	return c.Render("rules", fiber.Map{
		"FiltParams": fparams,
	}, "layouts/base")
}

func RuleChange(c *fiber.Ctx) error {

	delay, err := strconv.Atoi(utils.CopyString(c.FormValue("delay")))
	if err != nil {
		return c.JSON(fiber.Map{
			"status": "Failed to parse delay value",
			"err":    err,
			"val":    c.FormValue("delay"),
		})
	}
	fparams := FiltParams{
		SrcIpAddr:  utils.CopyString(c.FormValue("src")),
		DestIpAddr: utils.CopyString(c.FormValue("dest")),
		DelayMs:    delay,
	}

	log.Println(fparams)
	redeployBpf(&fparams)
	return c.Redirect("/rulesets")
	// return c.JSON(fiber.Map{
	// 	"status": "OK",
	// })
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
