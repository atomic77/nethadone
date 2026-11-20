package handlers

import (
	"bufio"
	"cmp"
	"log"
	"net/http"
	"os"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/atomic77/nethadone/config"
	"github.com/atomic77/nethadone/database"
	"github.com/atomic77/nethadone/models"
	"github.com/atomic77/nethadone/views"
	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"
)

type counterSample struct {
	v uint64
	t time.Time
}

// shortcut: rates are deltas since the previous page load by any viewer, so two open tabs
// halve the window; add kernel rate estimators (tc ... est) if that ever matters.
var (
	samplesMu sync.Mutex
	samples   = map[string]counterSample{}
)

// rate returns the per-second increase of counter key since the last call, 0 on the first call or a reset.
func rate(key string, v uint64, now time.Time) float64 {
	samplesMu.Lock()
	defer samplesMu.Unlock()
	prev, ok := samples[key]
	samples[key] = counterSample{v, now}
	dt := now.Sub(prev.t).Seconds()
	if !ok || v < prev.v || dt <= 0 {
		return 0
	}
	return float64(v-prev.v) / dt
}

// tcTree walks the qdiscs and classes of a link from the root, depth first in handle order.
// The clsact qdisc hangs off its own parent, so it is left out.
func tcTree(qdiscs []netlink.Qdisc, classes []netlink.Class, now time.Time) []models.TcNode {
	type item struct {
		handle uint32
		node   models.TcNode
	}
	children := map[uint32][]item{}
	add := func(handle, parent uint32, n models.TcNode, st *netlink.ClassStatistics) {
		n.Handle = netlink.HandleStr(handle)
		if st != nil && st.Basic != nil {
			n.Bytes, n.Packets = st.Basic.Bytes, uint64(st.Basic.Packets)
			n.Bps = rate("tc "+n.Handle, n.Bytes, now)
		}
		if st != nil && st.Queue != nil {
			n.Drops, n.Overlimits, n.Backlog = st.Queue.Drops, st.Queue.Overlimits, st.Queue.Backlog
		}
		children[parent] = append(children[parent], item{handle, n})
	}
	for _, q := range qdiscs {
		a := q.Attrs()
		n := models.TcNode{Kind: q.Type()}
		switch q := q.(type) {
		case *netlink.Htb:
			n.DefaultCls = netlink.HandleStr(netlink.MakeHandle(uint16(a.Handle>>16), uint16(q.Defcls)))
		case *netlink.Netem:
			n.DelayMs = float64(q.Latency) / netlink.TickInUsec() / 1000
		}
		add(a.Handle, a.Parent, n, (*netlink.ClassStatistics)(a.Statistics))
	}
	for _, c := range classes {
		a := c.Attrs()
		n := models.TcNode{Kind: c.Type() + " class"}
		if h, ok := c.(*netlink.HtbClass); ok {
			n.RateBits, n.CeilBits = h.Rate*8, h.Ceil*8 // the kernel reports bytes/s
		}
		parent := a.Parent
		if parent == netlink.HANDLE_ROOT { // the kernel reports top-level classes (1:1) as root, not 1:
			parent = a.Handle &^ 0xffff
		}
		add(a.Handle, parent, n, a.Statistics)
	}
	var out []models.TcNode
	var walk func(parent uint32, depth int)
	walk = func(parent uint32, depth int) {
		kids := children[parent]
		slices.SortFunc(kids, func(a, b item) int { return cmp.Compare(a.handle, b.handle) })
		for _, k := range kids {
			k.node.Depth = depth
			out = append(out, k.node)
			if k.handle != parent { // guard against a self-parented entry looping forever
				walk(k.handle, depth+1)
			}
		}
	}
	walk(netlink.HANDLE_ROOT, 0)
	return out
}

// policiesByLeaf maps leaf class handles ("1:30") to the policies the throttler sends there.
func policiesByLeaf(policies []models.Policy) map[string][]string {
	m := map[string][]string{}
	for _, p := range policies {
		h := netlink.HandleStr(netlink.MakeHandle(1, leafHandle(p.Class/10-1)))
		m[h] = append(m[h], p.SrcIp+" "+p.GlobGroup)
	}
	return m
}

func memAvailable() uint64 {
	f, err := os.Open("/proc/meminfo")
	if err != nil {
		return 0
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		if f := strings.Fields(sc.Text()); len(f) >= 2 && f[0] == "MemAvailable:" {
			kb, _ := strconv.ParseUint(f[1], 10, 64)
			return kb * 1024
		}
	}
	return 0
}

func sysStats(now time.Time) models.SysStats {
	var s models.SysStats
	var si unix.Sysinfo_t
	if err := unix.Sysinfo(&si); err == nil {
		s.Uptime = time.Duration(si.Uptime) * time.Second
		for i, l := range si.Loads {
			s.Load[i] = float64(l) / (1 << unix.SI_LOAD_SHIFT)
		}
		s.MemTotal = uint64(si.Totalram) * uint64(si.Unit)
	}
	s.MemAvail = memAvailable()
	for _, iface := range []struct{ name, role string }{{config.Cfg.LanInterface, "LAN"}, {config.Cfg.WanInterface, "WAN"}} {
		l, err := netlink.LinkByName(iface.name)
		if err != nil || l.Attrs().Statistics == nil {
			continue
		}
		st := l.Attrs().Statistics
		s.Ifaces = append(s.Ifaces, models.IfaceStats{
			Name: iface.name, Role: iface.role,
			RxBytes: st.RxBytes, TxBytes: st.TxBytes,
			RxBps: rate("rx "+iface.name, st.RxBytes, now), TxBps: rate("tx "+iface.name, st.TxBytes, now),
			RxDrops: st.RxDropped, TxDrops: st.TxDropped,
		})
	}
	throttleMu.Lock()
	defer throttleMu.Unlock()
	s.Rules, s.RulesAt = len(throttleRules), throttleRulesAt
	if t := BpfCtx.ThrottleObjs; t != nil && t.Map != nil {
		var err error
		if s.Scanned, err = t.counter(0); err == nil {
			s.Throttled, err = t.counter(1)
		}
		if err != nil {
			log.Println("shaping: reading throttle_stats: ", err)
		}
	}
	return s
}

// Shaping shows the LAN HTB tree with live kernel counters, plus interface, throttler and system numbers.
func Shaping(w http.ResponseWriter, r *http.Request) {
	now := time.Now()
	var tree []models.TcNode
	if link, err := netlink.LinkByName(config.Cfg.LanInterface); err != nil {
		log.Println("shaping: ", err)
	} else {
		qdiscs, err := netlink.QdiscList(link)
		if err != nil {
			log.Println("shaping: listing qdiscs: ", err)
		}
		classes, err := netlink.ClassList(link, 0)
		if err != nil {
			log.Println("shaping: listing classes: ", err)
		}
		tree = tcTree(qdiscs, classes, now)
	}
	var policies []models.Policy
	if p := database.GetAllPolicies(); p != nil {
		policies = *p
	}
	byLeaf := policiesByLeaf(policies)
	for i := range tree {
		tree[i].Policies = byLeaf[tree[i].Handle]
	}
	render(w, r, views.Shaping(sysStats(now), tree))
}
