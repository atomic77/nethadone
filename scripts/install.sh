#!/usr/bin/env bash
# Install or update nethadone on a Debian/Ubuntu/Armbian device. Safe to re-run.
#
#   sudo scripts/install.sh [options]
#   curl -fsSL https://raw.githubusercontent.com/atomic77/nethadone/main/scripts/install.sh | sudo bash -s -- [options]
#
# Options:
#   --lan IF                LAN interface (default: detected, see below)
#   --hotspot SSID PASS     create a NetworkManager wifi hotspot as the LAN (it also does DHCP, DNS and NAT)
#   --nat                   add an nftables masquerade rule on the WAN (wired setups without a hotspot)
#   --binary PATH           install this binary instead of downloading the latest release
#   --no-start              only install files: no sysctl, network changes, checks or service start
#                           (for chroot image builds, see build.sh)
#
# Without --lan, the LAN is the wifi interface with --hotspot, otherwise the only
# physical interface that is neither wireless nor the WAN (the default route).
set -euo pipefail

REPO=atomic77/nethadone
BIN=/usr/local/bin/nethadone
ETC=/etc/nethadone
CONF=$ETC/nethadone.yml
UNIT=/etc/systemd/system/nethadone.service

lan="" ssid="" pass="" nat=0 binary="" start=1
while (($#)); do
	case $1 in
	--lan) lan=$2; shift ;;
	--hotspot) ssid=$2 pass=$3; shift 2 ;;
	--nat) nat=1 ;;
	--binary) binary=$2; shift ;;
	--no-start) start=0 ;;
	*) sed -n '2,17p' "$0" 2>/dev/null || true; echo "unknown option: $1" >&2; exit 1 ;;
	esac
	shift
done

die() { echo "error: $*" >&2; exit 1; }
[ "$(id -u)" = 0 ] || die "run as root"
if [ -n "$ssid" ] && [ ${#pass} -lt 8 ]; then die "hotspot password must be at least 8 characters"; fi

case $(uname -m) in
aarch64) arch=arm64 ;;
armv7l) arch=arm ;;
x86_64) arch=amd64 ;;
*) die "unsupported architecture $(uname -m)" ;;
esac

wan_if() { ip -4 route show default 2>/dev/null | awk '{for (i = 1; i < NF; i++) if ($i == "dev") { print $(i + 1); exit }}'; }
is_wireless() { [ -d "/sys/class/net/$1/wireless" ]; }

echo "== packages"
# clang + libbpf + kernel UAPI headers (via libc6-dev, so a board kernel's own
# linux-libc-dev package is kept) compile the throttler on the device
pkgs=(curl ca-certificates clang libbpf-dev libc6-dev avahi-daemon)
# Known-working asm/ headers on the Orange Pi R1/Zero (see rebuildBpf include paths)
[ $arch = arm ] && pkgs+=(libc6-dev-armel-cross)
((nat)) && pkgs+=(nftables)
[ -n "$ssid" ] && pkgs+=(network-manager)
apt-get update -qq
DEBIAN_FRONTEND=noninteractive apt-get install -y -qq "${pkgs[@]}"

echo "== binary"
if [ -n "$binary" ]; then
	install -m 755 "$binary" $BIN
else
	tmp=$(mktemp)
	curl -fL --retry 3 -o "$tmp" "https://github.com/$REPO/releases/latest/download/nethadone-$arch-linux"
	install -m 755 "$tmp" $BIN
	rm -f "$tmp"
fi
echo "installed $BIN"

echo "== forwarding"
echo "net.ipv4.ip_forward = 1" >/etc/sysctl.d/20-nethadone.conf
((start)) && sysctl -q -p /etc/sysctl.d/20-nethadone.conf

if [ -n "$ssid" ]; then
	echo "== hotspot"
	if [ -z "$lan" ]; then
		for d in /sys/class/net/*/wireless; do [ -e "$d" ] && lan=$(basename "$(dirname "$d")") && break; done
		[ -n "$lan" ] || die "no wifi interface found for the hotspot"
	fi
	if ((start)); then
		# activation can fail on the first try (see below); the profile is still created
		nmcli device wifi hotspot ifname "$lan" con-name Hotspot ssid "$ssid" password "$pass" || true
		# PMF off + WPA2/CCMP on 2.4 GHz: the Zero2's UWE5622 driver can't finish the handshake otherwise
		nmcli connection modify Hotspot connection.autoconnect yes \
			802-11-wireless.band bg 802-11-wireless.channel 6 \
			802-11-wireless-security.key-mgmt wpa-psk 802-11-wireless-security.proto rsn \
			802-11-wireless-security.pairwise ccmp 802-11-wireless-security.group ccmp \
			802-11-wireless-security.pmf disable
		nmcli connection up Hotspot
		echo "hotspot $ssid up on $lan"
	else
		echo "skipped (--no-start); on the device run: sudo scripts/install.sh --hotspot $ssid <password>"
	fi
fi

# In a chroot (--no-start) the routes are the build host's, not the device's
wan=""
if ((start)); then wan=$(wan_if); fi
if [ -z "$lan" ] && ((start)); then
	cands=()
	for d in /sys/class/net/*/device; do
		i=$(basename "$(dirname "$d")")
		[ "$i" != "$wan" ] && ! is_wireless "$i" && cands+=("$i")
	done
	if [ ${#cands[@]} = 1 ]; then
		lan=${cands[0]}
		echo "detected LAN interface $lan"
	else
		echo "warning: could not pick a LAN interface (candidates: ${cands[*]:-none}); pass --lan" >&2
	fi
fi

echo "== config"
mkdir -p $ETC
if [ ! -f $CONF ]; then
	cat >$CONF <<-EOF
		# Only the settings that differ from the defaults; all options are documented in
		# https://github.com/$REPO/blob/main/config/nethadone.yml
		# Run \`sudo nethadone check\` after editing.

		# WAN: empty means the interface holding the default route (currently: ${wan:-none})
		wan_interface: ""
		lan_interface: "${lan:-lan0}"
	EOF
	echo "wrote $CONF"
elif [ -n "$lan" ]; then
	sed -i "s/^lan_interface:.*/lan_interface: \"$lan\"/" $CONF
	echo "set lan_interface: $lan in $CONF"
else
	echo "kept existing $CONF"
fi

if ((nat)); then
	echo "== nat"
	[ -n "$wan" ] || die "--nat needs a default route to find the WAN interface"
	# Re-applied by the service on every start, so it survives reboots without touching /etc/nftables.conf
	cat >$ETC/nat.nft <<-EOF
		table ip nethadone
		delete table ip nethadone
		table ip nethadone {
			chain postrouting {
				type nat hook postrouting priority srcnat; policy accept;
				oifname "$wan" masquerade
			}
		}
	EOF
	mkdir -p $UNIT.d
	printf '[Service]\nExecStartPre=/usr/sbin/nft -f %s\n' $ETC/nat.nft >$UNIT.d/nat.conf
	((start)) && nft -f $ETC/nat.nft
	echo "masquerading out of $wan"
fi

echo "== service"
here=$(cd "$(dirname "${BASH_SOURCE[0]:-$0}")" 2>/dev/null && pwd || true)
if [ -f "$here/nethadone.service" ]; then
	install -m 644 "$here/nethadone.service" $UNIT
else
	curl -fsSL -o $UNIT "https://raw.githubusercontent.com/$REPO/main/scripts/nethadone.service"
fi
systemctl daemon-reload 2>/dev/null || true
systemctl enable nethadone

if ((start)); then
	echo "== check"
	if $BIN check -config-file $CONF; then
		systemctl restart nethadone
		echo "nethadone running: http://$(hostname).local:3000  (logs: journalctl -u nethadone -f)"
	else
		echo "fix the failures above, then: sudo systemctl restart nethadone" >&2
		exit 1
	fi
fi
