#!/bin/bash

case "$(arch)" in
  'aarch64')
    # kern_url='https://github.com/daeuniverse/armbian-btf-kernel/releases/download/main-2023-06-17/kernel-rockchip64-current_23.08.0-trunk--6.1.34-Sca87-Dbeb1-Pa401-C3053Hfe66-HK01ba-Vc222-B76dc.tar'
    kern_url='https://github.com/atomic77/nethadone/releases/download/btf-kernel/kernel-legacy-rockchip64-orangepi-r1plus.tar.gz'
    neth_url='https://github.com/atomic77/nethadone/releases/download/nethadone-2024-08-12/nethadone-arm64-linux'
    ;;
  'armv7l')
    kern_url='https://github.com/atomic77/nethadone/releases/download/btf-kernel/kernel-legacy-sunxi-orangepi-r1.tar.gz'
    neth_url='https://github.com/atomic77/nethadone/releases/download/nethadone-2024-08-12/nethadone-arm-linux'
    ;;
  'x86_64')
    neth_url='https://github.com/atomic77/nethadone/releases/download/nethadone-2024-08-12/nethadone-amd64-linux'
    ;;
    # Most x86_64 builds for virtual machine testing use should have BTF enabled
  '*')
    echo "Unsupported architecture $(arch), exiting."
    exit 1
    ;;

esac

apt-get update -y

apt-get install -y apt-transport-https ca-certificates curl clang llvm jq \
        libelf-dev libpcap-dev libbfd-dev binutils-dev build-essential make \
        vim libbpf-dev avahi-daemon linux-tools-common dnsmasq prometheus

cat >> /etc/prometheus/prometheus.yml << EOF
  - job_name: "nethadone"
    static_configs:
      - targets: ["localhost:3000"]
EOF


cat <<EOF > /etc/systemd/system/nethadone.service
[Unit]
Description=Nethadone
Wants=network-online.target
After=network-online.target

[Service]
User=root
Restart=on-failure
RuntimeMaxSec=86400

ExecStart=/usr/local/bin/nethadone --config-file=/etc/nethadone.yml

[Install]
WantedBy=multi-user.target

EOF

systemctl daemon-reload
systemctl enable nethadone

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
# Grab the custom BTF-enabled kernel 
if [ $(arch) != 'x86_64' ]; then
  wget ${kern_url}
  tar xvf kernel-*
  apt-get install -y ./linux-*
  rm kernel*.tar
fi

if [ $(arch) == 'armv7l' ]; then
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

# Finally, copy in pre-built nethadone binary from github to avoid git
# checkout and golang compiler
curl -L -o /usr/local/bin/nethadone ${neth_url}
chmod +x /usr/local/bin/nethadone
