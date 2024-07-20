#!/bin/bash

apt-get update

# For Debian bookworm
apt-get install -y apt-transport-https ca-certificates curl clang llvm jq \
        libelf-dev libpcap-dev libbfd-dev binutils-dev build-essential make \
        bpfcc-tools python3-pip vim libbpf-dev \
        avahi-daemon bcc python-is-python3 \
        python3-dnslib python3-cachetools # for tcpconnect dns tracing in python

# For ubuntu jammy
apt-get install linux-tools-common

wget https://go.dev/dl/go1.22.3.linux-arm64.tar.gz
rm -rf /usr/local/go && tar -C /usr/local -xzf go1.22.3.linux-arm64.tar.gz

echo "export PATH=\$PATH:/usr/local/go/bin" >> /etc/profile

# Local prometheus for metrics collection
wget https://github.com/prometheus/prometheus/releases/download/v2.52.0/prometheus-2.52.0.linux-arm64.tar.gz
tar -C /usr/local -xzf prometheus-2.52.0.linux-arm64.tar.gz

cat >> /etc/prometheus.yml << EOF
  - job_name: "nethadone"
    static_configs:
      - targets: ["localhost:3000"]
EOF

# TODO Add prometheus systemd unit, data directory for autostart
# TODO and ip masq setup / ipv4 forward on boot, or enable
# in nethadone directly, i.e.
# iptables -t nat -A POSTROUTING -o eth0 -j MASQUERADE
# sysctl net.ipv4.ip_forward=1

#####
# Grab the custom BTF-enabled kernel from daeuniverse' repo
wget https://github.com/daeuniverse/armbian-btf-kernel/releases/download/main-2023-06-17/kernel-rockchip64-current_23.08.0-trunk--6.1.34-Sca87-Dbeb1-Pa401-C3053Hfe66-HK01ba-Vc222-B76dc.tar
tar xf kernel-rockchip64-current_23.08.0-trunk--6.1.34-Sca87-Dbeb1-Pa401-C3053Hfe66-HK01ba-Vc222-B76dc.tar
yum install ./linux-*

# Build BPFTool from source - repo packages don't seem to work properly
mkdir ~/src
cd ~/src
git clone --recurse-submodules https://github.com/libbpf/bpftool.git
cd bpftool/src
make -j 4
make install



