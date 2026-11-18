package handlers

import (
	"bufio"
	"bytes"
	"cmp"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/atomic77/nethadone/config"
	"github.com/atomic77/nethadone/models"
	"github.com/atomic77/nethadone/views"
	"github.com/vishvananda/netlink"
)

// lease is one IPv4 line of a dnsmasq leases file.
type lease struct {
	ip       net.IP
	mac      string
	hostname string
}

// parseLeases reads dnsmasq's "expiry mac ip hostname client-id" lines, skipping the duid and IPv6 lines.
func parseLeases(r io.Reader) []lease {
	var ls []lease
	sc := bufio.NewScanner(r)
	for sc.Scan() {
		f := strings.Fields(sc.Text())
		if len(f) < 4 {
			continue
		}
		ip := net.ParseIP(f[2]).To4()
		if ip == nil {
			continue
		}
		l := lease{ip: ip, mac: strings.ToLower(f[1])}
		if f[3] != "*" { // dnsmasq writes * when the client sent no hostname
			l.hostname = f[3]
		}
		ls = append(ls, l)
	}
	return ls
}

func readLeases() []lease {
	if config.Cfg.DhcpLeasesFile == "" {
		return nil
	}
	files, _ := filepath.Glob(config.Cfg.DhcpLeasesFile)
	var ls []lease
	for _, name := range files {
		f, err := os.Open(name)
		if err != nil {
			log.Println("reading DHCP leases: ", err)
			continue
		}
		ls = append(ls, parseLeases(f)...)
		f.Close()
	}
	return ls
}

var neighStates = map[int]string{
	netlink.NUD_REACHABLE: "reachable",
	netlink.NUD_STALE:     "stale",
	netlink.NUD_DELAY:     "delay",
	netlink.NUD_PROBE:     "probe",
	netlink.NUD_PERMANENT: "permanent",
}

// mergeClients builds one row per LAN IP seen in traffic or in the neighbour table.
// Leases only add a hostname (or a MAC) to those, since lease files keep expired devices.
func mergeClients(bl []models.Bandwidth, neighs []netlink.Neigh, leases []lease) []models.Client {
	byIP := map[string]*models.Client{}
	get := func(ip net.IP) *models.Client {
		c, ok := byIP[ip.String()]
		if !ok {
			c = &models.Client{IpAddr: ip}
			byIP[ip.String()] = c
		}
		return c
	}
	for _, b := range bl {
		if b.SrcIpAddr.IsMulticast() || b.SrcIpAddr.Equal(net.IPv4bcast) {
			continue
		}
		c := get(b.SrcIpAddr)
		c.Bytes += b.Bytes
		c.Remotes++
		if b.GlobName != "" && !slices.Contains(c.Globs, b.GlobName) {
			c.Globs = append(c.Globs, b.GlobName)
		}
	}
	for _, n := range neighs {
		state, ok := neighStates[n.State]
		if !ok || n.IP.To4() == nil || len(n.HardwareAddr) == 0 {
			continue // failed, incomplete or noarp entries are not live devices
		}
		c := get(n.IP.To4())
		c.State = state
		c.Mac = n.HardwareAddr.String()
	}
	for _, l := range leases {
		c, ok := byIP[l.ip.String()]
		if !ok {
			continue
		}
		c.Hostname = l.hostname
		if c.Mac == "" {
			c.Mac = l.mac
		}
	}
	clients := make([]models.Client, 0, len(byIP))
	for _, c := range byIP {
		if mac, err := net.ParseMAC(c.Mac); err == nil {
			c.Randomised = mac[0]&0x02 != 0
		}
		clients = append(clients, *c)
	}
	slices.SortFunc(clients, func(a, b models.Client) int {
		return cmp.Or(cmp.Compare(b.Bytes, a.Bytes), bytes.Compare(a.IpAddr.To4(), b.IpAddr.To4()))
	})
	return clients
}

// Clients lists LAN devices. Traffic comes from pollMetrics' cache, so it lags by up to map_collection_interval.
func Clients(w http.ResponseWriter, r *http.Request) {
	lastBandwidthMu.RLock()
	bl := lastBandwidth
	lastBandwidthMu.RUnlock()
	var neighs []netlink.Neigh
	if link, err := netlink.LinkByName(config.Cfg.LanInterface); err != nil {
		log.Println("clients: ", err)
	} else if neighs, err = netlink.NeighList(link.Attrs().Index, netlink.FAMILY_V4); err != nil {
		log.Println("clients: listing neighbours: ", err)
	}
	render(w, r, views.Clients(mergeClients(bl, neighs, readLeases())))
}
