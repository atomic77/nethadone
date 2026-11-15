#!/usr/bin/env bash
# Throwaway router topology in network namespaces, so nethadone can run for real
# on a dev machine without touching its interfaces:
#
#   [nth-client] eth0 --- lan0 [nth-router] wan0 --- eth0 [nth-inet]
#    10.10.0.2           10.10.0.1    10.20.0.1        10.20.0.2 (dnsmasq, iperf3)
#                                 mgmt0 10.30.0.2 --- nth-host 10.30.0.1 (host, for the UI)
#
# Usage (needs root, clang, libbpf-dev, dnsmasq, iperf3, dig):
#   sudo scripts/netns-lab.sh up          # namespaces, dnsmasq, iperf3
#   sudo scripts/netns-lab.sh run         # build + run nethadone inside nth-router (foreground)
#   sudo scripts/netns-lab.sh traffic     # in another shell: DNS lookups + iperf3 downloads
#   sudo scripts/netns-lab.sh status      # policies, qdisc class counters
#   sudo scripts/netns-lab.sh down
set -euo pipefail

LAB=${NTH_LAB_DIR:-/tmp/nethadone-lab}
REPO=$(cd "$(dirname "$0")/.." && pwd)
C=nth-client R=nth-router I=nth-inet
nsx() { local ns=$1; shift; ip netns exec "$ns" "$@"; }

up() {
	mkdir -p "$LAB"
	for ns in $C $R $I; do ip netns add $ns; nsx $ns ip link set lo up; done

	ip link add eth0 netns $C type veth peer name lan0 netns $R
	ip link add eth0 netns $I type veth peer name wan0 netns $R
	ip link add nth-host type veth peer name mgmt0 netns $R

	nsx $C ip addr add 10.10.0.2/24 dev eth0
	nsx $R ip addr add 10.10.0.1/24 dev lan0
	nsx $R ip addr add 10.20.0.1/24 dev wan0
	nsx $I ip addr add 10.20.0.2/24 dev eth0
	nsx $R ip addr add 10.30.0.2/24 dev mgmt0
	ip addr add 10.30.0.1/24 dev nth-host
	for l in "$C eth0" "$R lan0" "$R wan0" "$R mgmt0" "$I eth0"; do set -- $l; nsx "$1" ip link set "$2" up; done
	ip link set nth-host up

	nsx $C ip route add default via 10.10.0.1
	nsx $I ip route add 10.10.0.0/24 via 10.20.0.1
	nsx $R sysctl -qw net.ipv4.ip_forward=1

	# Fake "internet": every name under youtube.test / example.test resolves to the inet ns
	nsx $I dnsmasq --no-resolv --no-hosts --listen-address=10.20.0.2 --bind-interfaces \
		--address=/youtube.test/10.20.0.2 --address=/example.test/10.20.0.2 \
		--pid-file="$LAB/dnsmasq.pid"
	nsx $I iperf3 -s -D -B 10.20.0.2 --pidfile "$LAB/iperf3.pid"

	# Policy windows in 10s units instead of 1m: first policy after ~70s, then one step per ~15s
	cat >"$LAB/nethadone.yml" <<-EOF
		lan_interface: lan0
		wan_interface: wan0
		cfg_db: $LAB/cfg.db
		dns_db: $LAB/dns.db
		map_collection_interval: 2s
		policy_check_interval: 10s
		policy_backoff_interval: 15s
		policy_time_unit: 10s
		activity_threshold_bytes: 1000
	EOF
	echo "lab up. nethadone UI (after run): http://10.30.0.2:3000"
}

run() {
	# Build as the invoking user: root usually has no go on PATH and shouldn't own the build cache
	sudo -u "${SUDO_USER:-root}" -i bash -c "cd '$REPO' && go build -o build/nethadone-lab ./cmd/nethadone.go"
	# Seed the glob group once the web server is up
	(for _ in $(seq 30); do
		curl -sf -o /dev/null -d name=youtube -d 'glob=*.youtube.test.' -d description=lab \
			http://10.30.0.2:3000/globs && break
		sleep 1
	done) &
	cd "$LAB" && nsx $R "$REPO/build/nethadone-lab" -config-file nethadone.yml
}

traffic() {
	local end=$((SECONDS + ${1:-240}))
	while ((SECONDS < end)); do
		nsx $C dig +short @10.20.0.2 www.youtube.test >/dev/null
		# -R: server sends, so the bytes cross the router's LAN egress like real downloads
		nsx $C iperf3 -c 10.20.0.2 -R -t 10 -f k | grep -E 'receiver' || true
	done
}

status() {
	echo "== policies"; sqlite3 "$LAB/cfg.db" 'SELECT * FROM policy' || true
	# Unthrottled pairs get no classid and go HTB "direct", unshaped and outside every class counter;
	# veth runs at ~25 Gbit/s, so compare the map/metrics bytes with this total, not the classes.
	echo "== lan0 qdisc total (includes unthrottled direct traffic)"
	nsx $R tc -s qdisc show dev lan0 | grep -A1 '^qdisc htb' || true
	echo "== lan0 classes (throttled traffic only)"; nsx $R tc -s class show dev lan0 | grep -E '^class|Sent' || true
	echo "== metrics (IP bytes; tc also counts 14B Ethernet headers)"; curl -s http://10.30.0.2:3000/metrics | grep '^ip_pair' | head || true
}

down() {
	for f in dnsmasq iperf3; do [ -f "$LAB/$f.pid" ] && kill "$(cat "$LAB/$f.pid")" 2>/dev/null || true; done
	pkill -f "$REPO/build/nethadone-lab" 2>/dev/null || true
	ip link del nth-host 2>/dev/null || true
	for ns in $C $R $I; do ip netns del $ns 2>/dev/null || true; done
	rm -f "$LAB"/*.pid
}

case ${1:-} in
up | run | traffic | status | down) "$@" ;;
*) sed -n '2,14p' "$0"; exit 1 ;;
esac
