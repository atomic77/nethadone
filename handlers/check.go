package handlers

import (
	"bufio"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"strings"

	"github.com/atomic77/nethadone/config"
	"github.com/atomic77/nethadone/models"
	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"
)

// DefaultRouteInterface returns the interface holding the IPv4 default route,
// which is the WAN on practically every router setup.
func DefaultRouteInterface() (string, error) {
	routes, err := netlink.RouteList(nil, netlink.FAMILY_V4)
	if err != nil {
		return "", err
	}
	for _, r := range routes {
		if r.Dst == nil || r.Dst.IP.IsUnspecified() {
			if l, err := netlink.LinkByIndex(r.LinkIndex); err == nil {
				return l.Attrs().Name, nil
			}
		}
	}
	return "", errors.New("no IPv4 default route")
}

type checker struct {
	out   io.Writer
	fails int
}

func (c *checker) ok(msg string, a ...any) { fmt.Fprintf(c.out, "[ OK ] "+msg+"\n", a...) }

func (c *checker) warn(fix, msg string, a ...any) {
	fmt.Fprintf(c.out, "[WARN] "+msg+"\n", a...)
	if fix != "" {
		fmt.Fprintf(c.out, "       fix: %s\n", fix)
	}
}

func (c *checker) fail(fix, msg string, a ...any) {
	c.fails++
	fmt.Fprintf(c.out, "[FAIL] "+msg+"\n", a...)
	if fix != "" {
		fmt.Fprintf(c.out, "       fix: %s\n", fix)
	}
}

const pkgFix = "apt-get install -y clang libbpf-dev libc6-dev (or run scripts/install.sh)"

// Check verifies everything nethadone needs on this device, prints one line
// per check to out, and returns false if any check failed (warnings pass).
// It changes nothing on the system.
func Check(out io.Writer) bool {
	c := &checker{out: out}
	root := os.Geteuid() == 0
	if root {
		c.ok("running as root")
	} else {
		c.fail("run with sudo", "not running as root")
	}

	var uts unix.Utsname
	unix.Uname(&uts)
	release := unix.ByteSliceToString(uts.Release[:])
	if _, err := os.Stat("/sys/kernel/btf/vmlinux"); err == nil {
		c.ok("kernel %s has BTF", release)
	} else {
		c.fail("install a BTF-enabled kernel, see doc/install.md", "kernel %s has no BTF (/sys/kernel/btf/vmlinux missing)", release)
	}
	checkKernelConfig(c, release)

	if _, err := exec.LookPath("clang"); err != nil {
		c.fail(pkgFix, "clang not found in PATH")
	} else if !root {
		c.warn("", "skipping throttler test compile (needs root to load into the kernel)")
	} else if objs, err := buildThrottler(&[]models.IpPolicy{}); err != nil {
		c.fail(pkgFix, "throttler test compile/load failed: %v", err)
	} else {
		objs.Close()
		c.ok("throttler compiles with clang and passes the verifier")
	}

	checkInterfaces(c)

	if b, _ := os.ReadFile("/proc/sys/net/ipv4/ip_forward"); strings.TrimSpace(string(b)) == "1" {
		c.ok("IPv4 forwarding enabled")
	} else {
		c.fail("echo net.ipv4.ip_forward=1 > /etc/sysctl.d/20-nethadone.conf && sysctl --system", "IPv4 forwarding disabled")
	}
	checkNat(c)

	if c.fails > 0 {
		fmt.Fprintf(out, "%d check(s) failed\n", c.fails)
	}
	return c.fails == 0
}

func checkKernelConfig(c *checker, release string) {
	cfg, src := readKernelConfig(release)
	if cfg == nil {
		c.warn("", "kernel config not found (/proc/config.gz, /boot/config-%s); cannot check qdisc support", release)
		return
	}
	var missing []string
	for _, opt := range []string{"CONFIG_NET_SCH_HTB", "CONFIG_NET_SCH_NETEM", "CONFIG_NET_SCH_PRIO",
		"CONFIG_NET_SCH_INGRESS", "CONFIG_NET_CLS_BPF", "CONFIG_NET_CLS_ACT"} {
		if v := cfg[opt]; v != "y" && v != "m" {
			missing = append(missing, opt)
		}
	}
	if len(missing) > 0 {
		c.fail("use a kernel with these options enabled", "kernel lacks %s (from %s)", strings.Join(missing, ", "), src)
	} else {
		c.ok("kernel has HTB, netem, prio, clsact and cls_bpf (from %s)", src)
	}
}

func readKernelConfig(release string) (map[string]string, string) {
	var r io.Reader
	src := "/proc/config.gz"
	if f, err := os.Open(src); err == nil {
		defer f.Close()
		if gz, err := gzip.NewReader(f); err == nil {
			r = gz
		}
	}
	if r == nil {
		src = "/boot/config-" + release
		f, err := os.Open(src)
		if err != nil {
			return nil, ""
		}
		defer f.Close()
		r = f
	}
	cfg := map[string]string{}
	sc := bufio.NewScanner(r)
	for sc.Scan() {
		if k, v, ok := strings.Cut(sc.Text(), "="); ok {
			cfg[k] = v
		}
	}
	return cfg, src
}

func checkInterfaces(c *checker) {
	wan, lan := config.Cfg.WanInterface, config.Cfg.LanInterface
	defIf, defErr := DefaultRouteInterface()

	if wan == "" {
		c.fail("set wan_interface in the config file", "wan_interface empty and no default route to detect it from")
	} else if checkLink(c, "WAN", wan, "wan_interface") {
		if defErr != nil {
			c.warn("", "no IPv4 default route; clients will have no internet access")
		} else if defIf != wan {
			c.warn("", "default route is via %s, not the WAN %s", defIf, wan)
		} else {
			c.ok("default route via WAN %s", wan)
		}
	}

	if lan == wan {
		c.fail("set lan_interface to the interface your devices connect to", "LAN and WAN are both %s", lan)
	} else if checkLink(c, "LAN", lan, "lan_interface") {
		if ifc, err := net.InterfaceByName(lan); err == nil {
			if addrs, _ := ifc.Addrs(); !hasIPv4(addrs) {
				c.fail("give "+lan+" an address and DHCP server (e.g. the nmcli hotspot, see doc/install.md)", "LAN %s has no IPv4 address", lan)
			}
		}
	}
}

// checkLink reports whether ifname exists and is up; on failure it lists the candidates.
func checkLink(c *checker, role, ifname, key string) bool {
	l, err := netlink.LinkByName(ifname)
	if err != nil {
		c.fail("set "+key+" to one of: "+strings.Join(candidateInterfaces(), ", "), "%s interface %s does not exist", role, ifname)
		return false
	}
	if l.Attrs().Flags&net.FlagUp == 0 {
		c.fail("ip link set "+ifname+" up (and make it persistent)", "%s interface %s is down", role, ifname)
		return false
	}
	c.ok("%s interface %s is up", role, ifname)
	return true
}

func candidateInterfaces() []string {
	var names []string
	ifs, _ := net.Interfaces()
	for _, i := range ifs {
		if i.Flags&net.FlagLoopback == 0 && i.Name != config.Cfg.WanInterface && !strings.HasPrefix(i.Name, "docker") && !strings.HasPrefix(i.Name, "veth") {
			names = append(names, i.Name)
		}
	}
	return names
}

func hasIPv4(addrs []net.Addr) bool {
	for _, a := range addrs {
		if ipn, ok := a.(*net.IPNet); ok && ipn.IP.To4() != nil {
			return true
		}
	}
	return false
}

// checkNat only warns: a NetworkManager hotspot or a routed setup may not need our rule.
func checkNat(c *checker) {
	for _, cmd := range [][]string{{"nft", "list", "ruleset"}, {"iptables-save", "-t", "nat"}} {
		if out, err := exec.Command(cmd[0], cmd[1:]...).Output(); err == nil &&
			strings.Contains(strings.ToLower(string(out)), "masquerade") {
			c.ok("NAT masquerade rule present")
			return
		}
	}
	c.warn("scripts/install.sh --nat, or see doc/install.md", "no NAT masquerade rule found; LAN clients may not reach the internet")
}
