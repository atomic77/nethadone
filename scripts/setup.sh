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
wget https://github.com/prometheus/prometheus/releases/download/v2.52.0/prometheus-2.52.0.linux-amd64.tar.gz
tar -C /usr/local -xzf prometheus-2.52.0.linux-amd64.tar.gz

# BPFTool if it doesn't install w/ custom kernel

mkdir ~/src
cd ~/src
git clone --recurse-submodules https://github.com/libbpf/bpftool.git
cd bpftool/src
make -j 4
sudo make install



