_Miscellaneous notes from earlier in development. To be cleaned up and moved elsewhere if
still relevant._

## Background

Relevant docs:

https://netdevconf.info//0x14/pub/papers/55/0x14-paper55-talk-paper.pdf
http://vger.kernel.org/lpc_bpf2018_talks/lpc-bpf-2018-shaping.pdf
https://qmonnet.github.io/whirl-offload/2020/04/11/tc-bpf-direct-action/

## R1plus Kernel BTF support

Armbian builds for r1plus do not have BTF kernel debugging info built in
at `/sys/kernel/btf/vmlinux`. Attempting to build a new kernel with the required
options like DEBUG_INFO_BTF does not seem to produce an image which can 
be started up, and takes forever. 

https://github.com/daeuniverse/armbian-btf-kernel has some kernels which 
can be copied into an Armbian Jammy 23.08 fresh installation; need to 
use `kernel-rockchip64-current_23.08.0-trunk--6.1.34-Sca87-Dbeb1-Pa401-C3053Hfe66-HK01ba-Vc222-B76dc.tar` for orangepi-r1plus.


## IP Address Aggregation

Great repo with ip address block reservation info:
https://github.com/ipverse

Command to extract all aggregated.json files into one,

```
find . -type f -name 'aggregated.json' -print0 |   sort -z |   xargs -0 cat -- >> all.json
```

Then can make easy work of this with duckdb:

```bash
D select * from 'all.json' where handle like '%WATERLOO%';
┌────────┬──────────────────────┬──────────────────────┬───────────────────────────────────────────────────────────────┐
│  asn   │        handle        │     description      │                            subnets                            │
│ int64  │       varchar        │       varchar        │            struct(ipv4 varchar[], ipv6 varchar[])             │
├────────┼──────────────────────┼──────────────────────┼───────────────────────────────────────────────────────────────┤
│  12093 │ UWATERLOO            │ University of Wate…  │ {'ipv4': [129.97.0.0/16, 198.96.155.0/24], 'ipv6': [2620:10…  │
│  30559 │ NAVTECHWATERLOOOFF…  │ Navtech Systems Su…  │ {'ipv4': [204.138.153.0/24], 'ipv6': []}                      │
│  36061 │ WATERLOO-01          │ Waterloo Fiber       │ {'ipv4': [170.62.164.0/22], 'ipv6': [2602:f9c2::/36]}         │
│ 398848 │ OPENTEXT-NA-WATERL…  │ Open Text Corporat…  │ {'ipv4': [204.107.30.0/23], 'ipv6': []}                       │
│   4508 │ WATERLOO-INTUITION   │ NeuStyle             │ {'ipv4': [23.175.32.0/24, 155.254.2.0/23, 198.89.188.0/23],…  │
└────────┴──────────────────────┴──────────────────────┴───────────────────────────────────────────────────────────────┘
D
```

TODO Make this more query-friendly (similar to Maxmind IP databases?)

## Temporarily enabling ip forwarding on box

```bash
iptables -t nat -A POSTROUTING -o eth0 -j MASQUERADE
sysctl net.ipv4.ip_forward=1
```

On client:

```bash
ip route add default via 192.168.171.128
```

## Prometheus Queries

Top IP addr endpoints:
```
topk(10, rate(ip_pair_vic_bytes_total[1m]))
```

Aggregated bandwidth over 5m for glob groups:
```
sum by (glob) (rate(ip_pair_vic_bytes_total{glob!=""}[5m]))
```

## Armv7 support

Attempting to get this running on orangepi zero (armv7 chipset):

Additional headers

```bash
sudo apt-get install libc6-dev-armel-cross
```

Install BTF kernel provided by dae:

```bash
wget https://github.com/daeuniverse/armbian-btf-kernel/releases/download/main-2023-06-17/kernel-sunxi-current_23.08.0-trunk--6.1.33-S2f39-Dbeb1-Pb9fd-Cca58Hfe66-HK01ba-V014b-B76dc.tar
tar xf kernel-sunxi-current_23.08.0-trunk--6.1.33-S2f39-Dbeb1-Pb9fd-Cca58Hfe66-HK01ba-V014b-B76dc.tar
sudo dpkg --install linux-*
```

BPF compilation needs different include dir:
```bash
clang -g -O2 -I/usr/arm-linux-gnueabi/include -Wall -target bpf -c throttle.bpf.c -o throttle.o
```

Qdiscs:

```bash
sudo tc qdisc add dev eth0 clsact
sudo tc qdisc add dev wlan0 clsact
sudo tc qdisc replace dev eth0 root fq
sudo tc qdisc replace dev wlan0 root fq
```

## QDisc Notes

For reasons I've yet to understand, less data that goes into `lan0` seems to go through 
the qdisc than if throttling is attempted on `eth0`. But this prevents us from applying
source-based filtering, since by the time the packet arrives on `eth0`, it has been natted
to appear as if it's coming from the ip of `eth0`. 

The following qdisc structure still allows much more data through on `iperf3` tests than these 
rate limits suggest, and the quantity of data that runs through the bpf classifier is maybe 10-25%
of the actual transfer, but it does a good enough job of deteriorating traffic that we can 
leave this aside for another day.

Example of qdisc setup that we can use to steadily deteriorate traffic:

```bash

sudo tc qdisc delete dev lan0 root

sudo tc qdisc add dev lan0 root handle 1: htb default 10

sudo tc class add dev lan0 parent 1: classid 1:1 htb rate 100Mbit ceil 100Mbit

sudo tc class add dev lan0 parent 1:1 classid 1:10 htb rate 95Mbit ceil 100Mbit
sudo tc class add dev lan0 parent 1:1 classid 1:20 htb rate 500kbit ceil 500kbit
sudo tc class add dev lan0 parent 1:1 classid 1:30 htb rate 50kbit ceil 50kbit

sudo tc qdisc add dev lan0 parent 1:10 handle 10: sfq perturb 10
sudo tc qdisc add dev lan0 parent 1:20 handle 20: netem delay 25ms 5ms
sudo tc qdisc add dev lan0 parent 1:30 handle 30: netem delay 250ms 50ms

sudo tc filter add dev lan0 protocol ip parent 1:0 bpf obj ebpf/throttle.o classid 1: direct-action

```

