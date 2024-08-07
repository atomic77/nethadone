#!/bin/bash

case "$(arch)" in
  'aarch64')
    prom_url='https://github.com/prometheus/prometheus/releases/download/v2.53.1/prometheus-2.53.1.linux-arm64.tar.gz'
    go_url='https://go.dev/dl/go1.22.3.linux-arm64.tar.gz'
    kern_url='https://github.com/daeuniverse/armbian-btf-kernel/releases/download/main-2023-06-17/kernel-rockchip64-current_23.08.0-trunk--6.1.34-Sca87-Dbeb1-Pa401-C3053Hfe66-HK01ba-Vc222-B76dc.tar'
    ;;
  'armv7l')
    prom_url='https://github.com/prometheus/prometheus/releases/download/v2.53.1/prometheus-2.53.1.linux-armv7.tar.gz'
    go_url='https://go.dev/dl/go1.22.3.linux-armv6l.tar.gz'
    kern_url='https://github.com/atomic77/nethadone/releases/download/btf-kernel/kernel-legacy-sunxi-orangepi-r1.tar.gz'
    ;;
  'x86_64')
    prom_url='https://github.com/prometheus/prometheus/releases/download/v2.53.1/prometheus-2.53.1.linux-amd64.tar.gz'
    go_url='https://go.dev/dl/go1.22.3.linux-arm64.tar.gz'
    # Most x86_64 builds for virtual machine testing use should have BTF enabled
  '*')
    echo "Unsupported architecture $(arch), exiting."
    exit 1
    ;;

esac

apt-get update -y

apt-get install -y apt-transport-https ca-certificates curl clang llvm jq \
        libelf-dev libpcap-dev libbfd-dev binutils-dev build-essential make \
        vim libbpf-dev avahi-daemon linux-tools-common dnsmasq 

wget ${go_url}
rm -rf /usr/local/go && tar -C /usr/local -xzf go1.*
rm go*.tar.gz
echo "export PATH=\$PATH:/usr/local/go/bin" >> /etc/profile

#####
# Local prometheus for metrics collection
wget ${prom_url} 
tar -C /usr/local/bin --strip-components 1 -xzf prometheus-*.tar.gz
rm prometheus-*.tar.gz

cp /usr/local/bin/prometheus.yml /etc
mkdir -p /var/lib/prometheus

cat >> /etc/prometheus.yml << EOF
  - job_name: "nethadone"
    static_configs:
      - targets: ["localhost:3000"]
EOF

cat <<EOF  > /etc/systemd/system/prometheus.service
[Unit]
Description=Prometheus
Wants=network-online.target
After=network-online.target

[Service]
User=root
Restart=on-failure

ExecStart=/usr/local/bin/prometheus \
  --config.file=/etc/prometheus.yml \
  --storage.tsdb.path=/var/lib/prometheus

[Install]
WantedBy=multi-user.target

EOF

systemctl daemon-reload
systemctl enable prometheus

######
# NAT forwarding will need to be in place for routing to work,
# but the interface may change depending on your configuration.
# TODO Configure this in nethadone directly 
#echo "iptables -t nat -A POSTROUTING -o eth0 -j MASQUERADE" >> /etc/rc.local

echo "net.ipv4.ip_forward = 1" > /etc/sysctl.d/20-nethadone.conf


# For wifi-enabled devices, network-manager does a nice job of setting up an AP
# that automatically proxies DNS requests, eliminating the need for pihole.
# eg:
# sudo nmcli dev wifi hotspot ifname wlan0 ssid nethadone password "mypass"
# 

#####
# Grab the custom BTF-enabled kernel from daeuniverse' repo
if [ $(arch) != 'x86_64' ]; then
  wget ${kern_url}
  tar xvf kernel-*
  apt-get install -y ./linux-*
  rm kernel*.tar
fi

if [ $(arch) == 'armv7' ]; then
  apt-get install libc6-dev-armel-cross -y
fi 

# Build BPFTool from source - repo packages don't seem to work properly
mkdir ~/src
cd ~/src
git clone --recurse-submodules https://github.com/libbpf/bpftool.git
cd bpftool/src
make -j 4
make install
rm -rf ~/src

# TODO - Copy in pre-built nethadone binary from github to avoid git
# checkout and golang compiler

