package handlers

import (
	"cmp"
	"encoding/binary"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"path/filepath"
	"slices"
	"strings"

	"github.com/VictoriaMetrics/metrics"
	"github.com/a-h/templ"
	"github.com/atomic77/nethadone/config"
	"github.com/atomic77/nethadone/database"
	"github.com/atomic77/nethadone/models"
	"github.com/atomic77/nethadone/views"
)

func render(w http.ResponseWriter, r *http.Request, c templ.Component) {
	if err := c.Render(r.Context(), w); err != nil {
		log.Println("render ", r.URL.Path, ": ", err)
	}
}

func Index(w http.ResponseWriter, r *http.Request) {
	ifaces, err := net.Interfaces()
	if err != nil {
		http.Error(w, "could not list interfaces: "+err.Error(), http.StatusInternalServerError)
		return
	}
	rows := make([]models.Iface, 0, len(ifaces))
	for _, i := range ifaces {
		row := models.Iface{Name: i.Name, Up: i.Flags&net.FlagUp != 0}
		addrs, _ := i.Addrs()
		for _, a := range addrs {
			row.Addrs = append(row.Addrs, a.String())
		}
		rows = append(rows, row)
	}
	nPolicies := 0
	if p := database.GetAllPolicies(); p != nil {
		nPolicies = len(*p)
	}
	render(w, r, views.Status(config.Cfg.LanInterface, config.Cfg.WanInterface, len(database.GetGlobs()), nPolicies, rows))
}

// classLabel describes policy class c. Class 10 is leaf 0, the open pipe; class 20 is scaled leaf 1, and so on.
func classLabel(c int) string {
	if c <= 10 {
		return "unthrottled"
	}
	kbs, ms := leafRate(c/10 - 1)
	return fmt.Sprintf("%.0f kbit/s, +%.0f ms", kbs, ms)
}

func Policies(w http.ResponseWriter, r *http.Request) {
	var policies []models.Policy
	if p := database.GetAllPolicies(); p != nil {
		policies = *p
	}
	render(w, r, views.Policies(policies, classLabel))
}

func Globs(w http.ResponseWriter, r *http.Request) {
	render(w, r, views.Globs(database.GetGlobs(), models.GlobGroup{}, ""))
}

func GlobAdd(w http.ResponseWriter, r *http.Request) {
	g := models.GlobGroup{
		Name:        strings.TrimSpace(r.FormValue("name")),
		Description: strings.TrimSpace(r.FormValue("description")),
		Glob:        strings.TrimSpace(r.FormValue("glob")),
	}
	var err error
	if g.Name == "" || g.Glob == "" {
		err = errors.New("name and glob are required")
	} else if _, err = filepath.Match(g.Glob, ""); err != nil {
		err = fmt.Errorf("invalid glob %q: %w", g.Glob, err)
	} else if err = database.AddGlob(&g); err != nil {
		log.Println("Failed to insert glob record ", err)
		err = fmt.Errorf("could not add %q (does it already exist?)", g.Name)
	}
	if err != nil {
		w.WriteHeader(http.StatusUnprocessableEntity)
		render(w, r, views.Globs(database.GetGlobs(), g, err.Error()))
		return
	}
	http.Redirect(w, r, "/globs", http.StatusSeeOther)
}

// GlobDelete removes a glob group. Its policies are left to decay through the normal policy loop.
func GlobDelete(w http.ResponseWriter, r *http.Request) {
	if err := database.DeleteGlob(r.FormValue("name")); err != nil {
		log.Println("Failed to delete glob record ", err)
		http.Error(w, "could not delete glob group", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/globs", http.StatusSeeOther)
}

func getMatchingGlobGroup(globs []models.GlobGroup, dom string) *models.GlobGroup {
	for _, g := range globs {
		if matched, _ := filepath.Match(g.Glob, dom); matched {
			return &g
		}
	}
	return nil
}

// decodeIpPairKey splits a trafficmon.bpf.c src_dest_bytes key into (LAN client, remote) IPs.
// The program runs on LAN egress, so the key's low word (packet daddr) is the client.
func decodeIpPairKey(key uint64) (net.IP, net.IP) {
	b := make([]byte, 8)
	binary.LittleEndian.PutUint64(b, key)
	return net.IP(b[0:4]), net.IP(b[4:8])
}

func getBandwidthList(globsOnly bool) []models.Bandwidth {
	// TODO This needs to be optimized

	var key, val uint64
	vals := make([]models.Bandwidth, 0)
	if BpfCtx.TrafficmonObjs != nil {
		globs := database.GetGlobs()
		entries := BpfCtx.TrafficmonObjs.trafficmonMaps.SrcDestBytes.Iterate()
		for entries.Next(&key, &val) {
			srcip, destip := decodeIpPairKey(key)
			dom := database.GetDomainForIP(destip.String())
			bl := models.Bandwidth{
				SrcIpAddr:  srcip,
				DestIpAddr: destip,
				Bytes:      val,
				ProbDomain: dom,
			}
			if gg := getMatchingGlobGroup(globs, dom); gg != nil {
				bl.GlobName = gg.Name
			} else if globsOnly {
				continue
			}
			vals = append(vals, bl)
		}
	}
	return vals
}

func Bandwidth(w http.ResponseWriter, r *http.Request) {
	bl := getBandwidthList(false)
	slices.SortFunc(bl, func(a, b models.Bandwidth) int { return cmp.Compare(b.Bytes, a.Bytes) })
	render(w, r, views.Bandwidth(bl))
}

func MetricsHandleFunc(w http.ResponseWriter, req *http.Request) {
	metrics.WritePrometheus(w, false)
}
