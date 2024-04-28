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
sudo tc qdisc add dev enx3a406b1307a9 clsact
sudo tc qdisc replace dev enx3a406b1307a9 root fq
```

Note:
replace above seems to be necessary (ends up replacing pfifo_fast with fq ?)

```bash
$ sudo tc qdisc show
qdisc pfifo_fast 0: dev enx3a406b1307a9 root refcnt 2 bands 3 priomap 1 2 2 2 1 2 0 0 1 1 1 1 1 1 1 1
qdisc clsact ffff: dev enx3a406b1307a9 parent ffff:fff1
atomic@r1plus:~/chiron$ sudo tc qdisc replace dev enx3a406b1307a9 root fq
atomic@r1plus:~/chiron$ sudo tc qdisc show
qdisc fq 8001: dev enx3a406b1307a9 root refcnt 2 limit 10000p flow_limit 100p buckets 1024 orphan_mask 1023 quantum 3028b initial_quantum 15140b low_rate_threshold 550Kbit refill_delay 40ms timer_slack 10us horizon 10s horizon_drop
qdisc clsact ffff: dev enx3a406b1307a9 parent ffff:fff1
```

build, attach:
```bash
clang -g -O2 -I/usr/include/aarch64-linux-gnu -Wall -target bpf -c tcfilt.bpf.c -o tcfilt.o
sudo tc filter add dev enp0s3 egress bpf direct-action obj tcfilt.o sec tc
```

View logs

```bash
sudo cat /sys/kernel/debug/tracing/trace_pipe

# To remove
sudo tc filter del dev enp0s3 egress
```

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
