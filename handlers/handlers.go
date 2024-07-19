package handlers

import (
	"encoding/binary"
	"log"
	"net/http"
	"path/filepath"

	"github.com/VictoriaMetrics/metrics"
	"github.com/atomic77/nethadone/database"
	"github.com/atomic77/nethadone/models"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/utils"
	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"

	"net"
)

// var config Config

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
		log.Fatal("Something weird when trying to list ifaces", err)
	}

	nh, err := netlink.NewHandle(unix.NETLINK_ROUTE)
	if err != nil {
		log.Fatal("could not get netlink handle", err)
	}

	ll, err := nh.LinkList()
	if err != nil {
		log.Fatal("could not get get link list", err)
	}

	return c.Render("interfaces", fiber.Map{
		"Title":      "Interfaces",
		"Interfaces": ifaces,
		"LinkList":   ll,
	}, "layouts/base")
}

func Policies(c *fiber.Ctx) error {

	// bl := getBandwidthList(true)
	policies := database.GetAllPolicies()
	return c.Render("policies", fiber.Map{
		// "BandwidthList": bl,
		"Policies": policies,
	}, "layouts/base")
}

func Globs(c *fiber.Ctx) error {

	g := database.GetGlobs()

	return c.Render("globs", fiber.Map{
		"Globs": g,
	}, "layouts/base")
}

func GlobAdd(c *fiber.Ctx) error {

	g := models.GlobGroup{
		Name:        utils.CopyString(c.FormValue("name")),
		Description: utils.CopyString(c.FormValue("description")),
		Glob:        utils.CopyString(c.FormValue("glob")),
		Device:      utils.CopyString(c.FormValue("device")),
	}

	err := database.AddGlob(&g)
	if err != nil {
		log.Println("Failed to insert glob record ", err)
	}
	return c.Redirect("/globs")
}

type BandwidthList struct {
	SrcIpAddr  net.IP
	DestIpAddr net.IP
	Bytes      uint64
	ProbDomain string
	GlobName   string
}

func getMatchingGlobGroup(dom string) *models.GlobGroup {
	// TODO Move this somewhere more appropriate
	globs := database.GetGlobs()
	for _, g := range globs {
		matched, _ := filepath.Match(g.Glob, dom)
		if matched {
			return &g
		}
	}
	return nil
}

func getBandwidthList(globsOnly bool) []BandwidthList {
	// TODO This needs to be optimized

	var key, val uint64
	vals := make([]BandwidthList, 0)
	if BpfCtx.TrafficmonObjs != nil {
		// entries := BpfCtx.TcFilterObjs.Map.Iterate()
		entries := BpfCtx.TrafficmonObjs.trafficmonMaps.SrcDestBytes.Iterate()
		for entries.Next(&key, &val) {
			// net.IP
			b := make([]byte, 8)
			binary.LittleEndian.PutUint64(b, key)
			src := binary.BigEndian.Uint32(b[0:4])
			dest := binary.BigEndian.Uint32(b[4:8])
			srcip := make(net.IP, 4)
			binary.BigEndian.PutUint32(srcip, src)
			destip := make(net.IP, 4)
			binary.BigEndian.PutUint32(destip, dest)
			dom := database.GetDomainForIP(destip.String())
			gg := getMatchingGlobGroup(dom)
			bl := BandwidthList{
				SrcIpAddr:  srcip,
				DestIpAddr: destip,
				Bytes:      val,
				ProbDomain: dom,
			}
			if gg != nil {
				bl.GlobName = gg.Name
				if globsOnly {
					vals = append(vals, bl)
				}
			}
			if !globsOnly {
				vals = append(vals, bl)
			}
		}
	}
	return vals
}

func Bandwidth(c *fiber.Ctx) error {
	bl := getBandwidthList(false)
	return c.Render("bandwidth", fiber.Map{
		"BandwidthList": bl,
	}, "layouts/base")
}

func Devices(c *fiber.Ctx) error {
	// TODO Provide something useful here based on actual devices in
	// system and clients seen on the network
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

func MetricsHandleFunc(w http.ResponseWriter, req *http.Request) {
	metrics.WritePrometheus(w, false)
}

// NotFound returns custom 404 page
func NotFound(c *fiber.Ctx) error {
	return c.Status(404).SendFile("./static/private/404.html")
}
