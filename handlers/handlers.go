package handlers

import (
	"bytes"
	"cmp"
	"encoding/binary"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"

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
	renderStatus(w, r, "")
}

// renderStatus renders the status page; msg reports the outcome of an update.
func renderStatus(w http.ResponseWriter, r *http.Request, msg string) {
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
	render(w, r, views.Status(config.Cfg.LanInterface, config.Cfg.WanInterface, len(database.GetGlobs()), nPolicies, rows, config.BuildInfo(), msg))
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
	hostnames := map[string]string{}
	for _, l := range readLeases() {
		if l.hostname != "" {
			hostnames[l.ip.String()] = l.hostname
		}
	}
	rules, at := loadedRules()
	rows := policyRows(policies, rules, hostnames, database.GetDomainsByIP(), database.GetGlobs())
	render(w, r, views.Policies(rows, len(rules), at, classLabel))
}

// policyRows joins the stored policies with the rules of the running throttler. A policy whose
// rules are missing or carry another class gets a Note: the rebuild failed or has not happened yet.
func policyRows(policies []models.Policy, rules []models.IpPolicy, hostnames map[string]string,
	domains map[string][]string, globs []models.GlobGroup) []models.PolicyRow {
	type key struct{ src, glob string }
	byPolicy := map[key][]models.IpPolicy{}
	for _, r := range rules {
		src := strings.ReplaceAll(r.SrcIpAddr, ",", ".")
		if src == "" {
			src = database.AllDevices
		}
		byPolicy[key{src, r.GlobGroup}] = append(byPolicy[key{src, r.GlobGroup}], r)
	}
	rows := make([]models.PolicyRow, 0, len(policies))
	for _, p := range policies {
		row := models.PolicyRow{Policy: p, Hostname: hostnames[p.SrcIp]}
		if p.SrcIp == database.AllDevices {
			row.Hostname = "every device"
		}
		matched := byPolicy[key{p.SrcIp, p.GlobGroup}]
		for _, r := range matched {
			ip := strings.ReplaceAll(r.DestIpAddr, ",", ".")
			dom, _ := matchDomains(globs, domains[ip])
			row.Remotes = append(row.Remotes, models.RemoteIP{IP: ip, Domain: dom})
		}
		slices.SortFunc(row.Remotes, func(a, b models.RemoteIP) int {
			return bytes.Compare(net.ParseIP(a.IP).To16(), net.ParseIP(b.IP).To16())
		})
		switch {
		case !slices.ContainsFunc(globs, func(g models.GlobGroup) bool { return g.Name == p.GlobGroup }):
			row.Note = "Glob group is switched off or deleted; this policy is not enforced and will decay."
		case len(matched) == 0:
			row.Note = "Not in the running throttler: no remote IPs seen for this glob group yet, or the rebuild is pending or failed."
		case matched[0].ClassId != p.Class:
			row.Note = fmt.Sprintf("The running throttler still uses class %d; the rebuild is pending or failed.", matched[0].ClassId)
		}
		rows = append(rows, row)
	}
	return rows
}

// presetGlobs are offered on the globs page for new installations. Notes:
//   - sqlite GLOB syntax, one pattern per group (no {a,b}), so a platform with unrelated
//     domain names needs several groups, each throttled on its own.
//   - An IP is stored under the queried name and every CNAME in its chain, so a glob on the
//     name users type also matches CDN-backed sites.
//   - "*reddit.com" also matches the bare domain, unlike "*.reddit.com", at the cost of
//     matching e.g. "notreddit.com".
//   - Video CDN domains (googlevideo, nflxvideo, ttvnw, ...) carry the bulk of the bytes.
//   - Every matching IP becomes a throttler rule; very broad groups on a busy network push
//     toward the ~1000 rule limit (see AGENTS.md known issues).
var presetGlobs = []models.GlobGroup{
	// Short video
	{Name: "tiktok", Description: "TikTok (site, API, CDNs)", Glob: "*tiktok*"}, // tiktok.com, tiktokv.com, tiktokcdn(-us).com
	{Name: "youtube", Description: "YouTube site", Glob: "*youtube.com"},
	{Name: "youtube-cdn", Description: "YouTube video CDN", Glob: "*googlevideo.com"},
	{Name: "youtube-img", Description: "YouTube thumbnails", Glob: "*ytimg.com"},
	// Social feeds
	{Name: "instagram", Description: "Instagram (site + cdninstagram.com)", Glob: "*instagram.com"},
	{Name: "facebook", Description: "Facebook", Glob: "*facebook.com"},
	{Name: "fbcdn", Description: "Meta CDN (Facebook + Instagram media)", Glob: "*fbcdn.net"},
	{Name: "snapchat", Description: "Snapchat", Glob: "*snapchat.com"},
	{Name: "snap-cdn", Description: "Snapchat CDN", Glob: "*sc-cdn.net"},
	{Name: "twitter", Description: "X/Twitter media", Glob: "*twimg.com"},
	{Name: "twitter-web", Description: "Twitter site", Glob: "*twitter.com"},
	{Name: "x", Description: "X site (subdomains only)", Glob: "*.x.com"}, // "*x.com" would also match netflix.com
	{Name: "reddit", Description: "Reddit (site, static, media)", Glob: "*reddit*"},
	{Name: "reddit-img", Description: "Reddit images/video", Glob: "*redd.it"},
	{Name: "pinterest", Description: "Pinterest", Glob: "*pinterest.com"},
	{Name: "pinimg", Description: "Pinterest images", Glob: "*pinimg.com"},
	// Streaming
	{Name: "twitch", Description: "Twitch site", Glob: "*twitch.tv"},
	{Name: "twitch-cdn", Description: "Twitch video", Glob: "*ttvnw.net"},
	{Name: "netflix", Description: "Netflix site", Glob: "*netflix.com"},
	{Name: "netflix-cdn", Description: "Netflix video/images", Glob: "*nflx*"}, // nflxvideo.net, nflxso.net, nflximg.net, nflxext.com
	// Games
	{Name: "roblox", Description: "Roblox", Glob: "*roblox.com"},
	{Name: "roblox-cdn", Description: "Roblox CDN", Glob: "*rbxcdn.com"},
}

func renderGlobs(w http.ResponseWriter, r *http.Request, form models.GlobGroup, errMsg string) {
	globs := database.GetAllGlobs()
	var presets []models.GlobGroup
	for _, p := range presetGlobs {
		if !slices.ContainsFunc(globs, func(g models.GlobGroup) bool { return g.Name == p.Name }) {
			presets = append(presets, p)
		}
	}
	render(w, r, views.Globs(globs, presets, form, errMsg))
}

func Globs(w http.ResponseWriter, r *http.Request) {
	renderGlobs(w, r, models.GlobGroup{}, "")
}

// GlobPresets adds the checked presetGlobs.
func GlobPresets(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	for _, name := range r.Form["preset"] {
		i := slices.IndexFunc(presetGlobs, func(g models.GlobGroup) bool { return g.Name == name })
		if i < 0 {
			continue
		}
		g := presetGlobs[i]
		if err := database.AddGlob(&g); err != nil {
			log.Println("Failed to insert preset glob ", name, ": ", err)
			w.WriteHeader(http.StatusUnprocessableEntity)
			renderGlobs(w, r, models.GlobGroup{}, fmt.Sprintf("could not add %q (does it already exist?)", name))
			return
		}
	}
	http.Redirect(w, r, "/globs", http.StatusSeeOther)
}

// GlobEnable turns a glob group on or off. A disabled group matches nothing, so the next
// policy check drops its throttler rules and its policies decay as for a deleted group.
func GlobEnable(w http.ResponseWriter, r *http.Request) {
	if err := database.SetGlobEnabled(r.FormValue("name"), r.FormValue("enabled") == "1"); err != nil {
		log.Println("Failed to update glob record ", err)
		http.Error(w, "could not update glob group", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/globs", http.StatusSeeOther)
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
		renderGlobs(w, r, g, err.Error())
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

// matchDomains picks the glob group for an IP's domains (newest first). Any name, e.g. the
// queried name ahead of its CNAME chain, can match, as in GetIPsMatchingGlob. It returns the
// matching name, or the newest name when no glob matches.
func matchDomains(globs []models.GlobGroup, doms []string) (dom, glob string) {
	for _, d := range doms {
		if gg := getMatchingGlobGroup(globs, d); gg != nil {
			return d, gg.Name
		}
	}
	if len(doms) > 0 {
		dom = doms[0]
	}
	return dom, ""
}

// decodeIpPairKey splits a trafficmon.bpf.c src_dest_bytes key into (LAN client, remote) IPs.
// The program runs on LAN egress, so the key's low word (packet daddr) is the client.
func decodeIpPairKey(key uint64) (net.IP, net.IP) {
	b := make([]byte, 8)
	binary.LittleEndian.PutUint64(b, key)
	return net.IP(b[0:4]), net.IP(b[4:8])
}

func getBandwidthList(globsOnly bool) []models.Bandwidth {
	var key, val uint64
	vals := make([]models.Bandwidth, 0)
	if BpfCtx.TrafficmonObjs != nil {
		globs := database.GetGlobs()
		domains := database.GetDomainsByIP()
		entries := BpfCtx.TrafficmonObjs.trafficmonMaps.SrcDestBytes.Iterate()
		for entries.Next(&key, &val) {
			srcip, destip := decodeIpPairKey(key)
			bl := models.Bandwidth{SrcIpAddr: srcip, DestIpAddr: destip, Bytes: val}
			bl.ProbDomain, bl.GlobName = matchDomains(globs, domains[destip.String()])
			if globsOnly && bl.GlobName == "" {
				continue
			}
			vals = append(vals, bl)
		}
	}
	return vals
}

const bandwidthPageSize = 100

// lastBandwidth is pollMetrics' latest walk of the trafficmon map, sorted by bytes descending.
// It is replaced, never mutated, so readers can keep slicing an old copy.
var (
	lastBandwidthMu sync.RWMutex
	lastBandwidth   []models.Bandwidth
)

func setLastBandwidth(bl []models.Bandwidth) {
	slices.SortFunc(bl, func(a, b models.Bandwidth) int { return cmp.Compare(b.Bytes, a.Bytes) })
	lastBandwidthMu.Lock()
	lastBandwidth = bl
	lastBandwidthMu.Unlock()
}

// pageBounds clamps the 1-based page to [1, pages] and returns the slice bounds for it.
func pageBounds(total, page, size int) (start, end, clamped, pages int) {
	pages = max(1, (total+size-1)/size)
	clamped = min(max(page, 1), pages)
	start = (clamped - 1) * size
	end = min(start+size, total)
	return
}

// Bandwidth serves the cached list from pollMetrics, so it can be stale by up to map_collection_interval.
func Bandwidth(w http.ResponseWriter, r *http.Request) {
	lastBandwidthMu.RLock()
	bl := lastBandwidth
	lastBandwidthMu.RUnlock()
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	start, end, page, pages := pageBounds(len(bl), page, bandwidthPageSize)
	render(w, r, views.Bandwidth(bl[start:end], page, pages, len(bl)))
}

func MetricsHandleFunc(w http.ResponseWriter, req *http.Request) {
	if s := pairMetrics.Load(); s != nil {
		s.WritePrometheus(w)
	}
}
