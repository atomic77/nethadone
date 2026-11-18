package handlers

import (
	"net"
	"reflect"
	"strings"
	"testing"

	"github.com/atomic77/nethadone/models"
	"github.com/vishvananda/netlink"
)

func TestParseLeases(t *testing.T) {
	in := `1763400000 aa:bb:cc:dd:ee:01 10.42.0.5 pixel-7 01:aa:bb:cc:dd:ee:01
1763400000 AA:BB:CC:DD:EE:02 10.42.0.6 * *
duid 00:01:00:01:2c:4f
1763400000 1234 fd00::5 laptop 00:01:00:01
`
	got := parseLeases(strings.NewReader(in))
	want := []lease{
		{ip: net.IPv4(10, 42, 0, 5).To4(), mac: "aa:bb:cc:dd:ee:01", hostname: "pixel-7"},
		{ip: net.IPv4(10, 42, 0, 6).To4(), mac: "aa:bb:cc:dd:ee:02"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %+v\nwant %+v", got, want)
	}
}

func TestMergeClients(t *testing.T) {
	ip := func(s string) net.IP { return net.ParseIP(s).To4() }
	mac := func(s string) net.HardwareAddr { m, _ := net.ParseMAC(s); return m }
	bl := []models.Bandwidth{
		{SrcIpAddr: ip("10.42.0.5"), Bytes: 100, GlobName: "yt"},
		{SrcIpAddr: ip("10.42.0.5"), Bytes: 50, GlobName: "yt"},
		{SrcIpAddr: ip("10.42.0.5"), Bytes: 10},
		{SrcIpAddr: ip("10.42.0.9"), Bytes: 500},
		{SrcIpAddr: ip("224.0.0.251"), Bytes: 999},
	}
	neighs := []netlink.Neigh{
		{IP: ip("10.42.0.5"), HardwareAddr: mac("aa:bb:cc:dd:ee:01"), State: netlink.NUD_REACHABLE},
		{IP: ip("10.42.0.7"), HardwareAddr: mac("00:11:22:33:44:55"), State: netlink.NUD_STALE},
		{IP: ip("10.42.0.8"), State: netlink.NUD_FAILED},
	}
	leases := []lease{
		{ip: ip("10.42.0.5"), mac: "aa:bb:cc:dd:ee:01", hostname: "pixel-7"},
		{ip: ip("10.42.0.9"), mac: "00:11:22:33:44:66", hostname: "laptop"},
		{ip: ip("10.42.0.99"), mac: "00:11:22:33:44:77", hostname: "gone"},
	}
	want := []models.Client{
		{IpAddr: ip("10.42.0.9"), Mac: "00:11:22:33:44:66", Hostname: "laptop", Bytes: 500, Remotes: 1},
		{IpAddr: ip("10.42.0.5"), Mac: "aa:bb:cc:dd:ee:01", Randomised: true, Hostname: "pixel-7", State: "reachable", Bytes: 160, Remotes: 3, Globs: []string{"yt"}},
		{IpAddr: ip("10.42.0.7"), Mac: "00:11:22:33:44:55", State: "stale"},
	}
	if got := mergeClients(bl, neighs, leases); !reflect.DeepEqual(got, want) {
		t.Errorf("got  %+v\nwant %+v", got, want)
	}
}
