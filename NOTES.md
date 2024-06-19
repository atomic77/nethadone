## Background

Relevant docs:

https://netdevconf.info//0x14/pub/papers/55/0x14-paper55-talk-paper.pdf
http://vger.kernel.org/lpc_bpf2018_talks/lpc-bpf-2018-shaping.pdf
https://qmonnet.github.io/whirl-offload/2020/04/11/tc-bpf-direct-action/

## Setup

```bash
apt-get install -y apt-transport-https ca-certificates curl clang llvm jq libelf-dev libpcap-dev libbfd-dev binutils-dev build-essential make linux-tools-common linux-tools-$(uname -r) bpfcc-tools python3-pip
```


## TC ingress example

Replace qdisc for interface w/ FQ root. Unsure why the initial creation of clsact
```bash
sudo tc qdisc add dev eth0 clsact
sudo tc qdisc replace dev eth0 root fq
```

Note:
replace above seems to be necessary (ends up replacing pfifo_fast with fq ?)

```bash
$ sudo tc qdisc show
qdisc pfifo_fast 0: dev enx3a406b1307a9 root refcnt 2 bands 3 priomap 1 2 2 2 1 2 0 0 1 1 1 1 1 1 1 1
qdisc clsact ffff: dev enx3a406b1307a9 parent ffff:fff1
atomic@r1plus:~/nethadone$ sudo tc qdisc replace dev enx3a406b1307a9 root fq
atomic@r1plus:~/nethadone$ sudo tc qdisc show
qdisc fq 8001: dev enx3a406b1307a9 root refcnt 2 limit 10000p flow_limit 100p buckets 1024 orphan_mask 1023 quantum 3028b initial_quantum 15140b low_rate_threshold 550Kbit refill_delay 40ms timer_slack 10us horizon 10s horizon_drop
qdisc clsact ffff: dev enx3a406b1307a9 parent ffff:fff1
```

build, attach:
```bash
clang -g -O2 -I/usr/include/aarch64-linux-gnu -Wall -target bpf -c tcfilt.bpf.c -o tcfilt.o
clang -g -O2 -Wall -target bpf -c tcfilt.bpf.c -o tcfilt.o
sudo tc filter add dev enp0s3 egress bpf direct-action obj tcfilt.o sec tc
```

View logs

```bash
sudo cat /sys/kernel/debug/tracing/trace_pipe

# To remove
sudo tc filter del dev enp0s3 egress
```

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

## Golang example vs working tcfilt

```bash
$ sudo bpftool prog list
24: sched_cls  name test  tag a04f5eef06a7f555  gpl
        loaded_at 2024-04-28T14:40:14-0400  uid 0
        xlated 16B  jited 104B  memlock 4096B
29: sched_cls  name tc_ingress  tag c285f188f853dd97  gpl
        loaded_at 2024-04-28T14:45:23-0400  uid 0
        xlated 472B  jited 408B  memlock 4096B  map_ids 5
        btf_id 9
```


## Other examples


### Ingress / Egress data counting example

Compiling example from https://liuhangbin.netlify.app/post/ebpf-and-xdp/ on arm64 box:

```bash
clang -g -O2 -I/usr/include/aarch64-linux-gnu -Wall -target bpf -c tc-ingress-count.bpf.c -o tc-ingress-count.o
```

Loading via iproute2:

```bash
 sudo tc qdisc add dev enp0s3 clsact
 sudo tc filter add dev enp0s3 ingress bpf direct-action obj tctest2.o sec ingress
 sudo tc filter add dev enp0s3 egress bpf da obj tctest2.o sec egress
```

To retrieve map:
```bash
$ sudo bpftool map dump --pretty name acc_map
[{
        "key": ["0x00","0x00","0x00","0x00"
        ],
        "value": ["0x5e","0xc2","0x01","0x00"
        ]
    },{
        "key": ["0x01","0x00","0x00","0x00"
        ],
        "value": ["0x66","0x19","0x03","0x00"
        ]
    }
]
```

To remove:

```bash
sudo tc filter del dev enp0s3 ingress
sudo tc filter del dev enp0s3 egress
```


## Docker to VM 

Can build docker image:
```bash
docker buildx build -t atomic77/nethadone:latest .
```

Then use [d2vm](https://github.com/linka-cloud/d2vm) to convert to a qcow image which can then be used with full kernel virtualization:

```bash
sudo d2vm convert atomic77/nethadone -o nethadone.qcow2 -p 1234
d2vm run qemu --networking bridge,virbr0 --mem 4096 --cpus 4 ../nethadone.qcow2 
```

On Windows with VirtualBox, storage type needs to be AHCI

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
