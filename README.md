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

Replace qdisc for interface w/ FQ root:
```bash
sudo tc qdisc add dev enx3a406b1307a9 clsact
sudo tc qdisc replace dev enx3a406b1307a9 root fq
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
