#!/bin/bash
# Runs as root inside an Armbian image (chroot via build.sh, or on a freshly
# flashed board): installs the BTF-enabled kernel nethadone needs, then
# everything else through install.sh. Network setup (hotspot, NAT) needs the
# real device, so do that after first boot with install.sh --hotspot / --nat.
set -euo pipefail

case "$(uname -m)" in
  'aarch64')
    kern_url='https://github.com/atomic77/nethadone/releases/download/btf-kernel/kernel-legacy-rockchip64-orangepi-r1plus.tar.gz'
    ;;
  'armv7l')
    kern_url='https://github.com/atomic77/nethadone/releases/download/btf-kernel/kernel-legacy-sunxi-orangepi-r1.tar.gz'
    ;;
  'x86_64')
    # Most x86_64 kernels (VMs for testing) already have BTF
    kern_url=''
    ;;
  *)
    echo "Unsupported architecture $(uname -m), exiting."
    exit 1
    ;;
esac

apt-get update -y
apt-get install -y curl

if [ -n "$kern_url" ]; then
  work=$(mktemp -d)
  curl -fL -o "$work/kernel.tar.gz" "$kern_url"
  tar -C "$work" -xf "$work/kernel.tar.gz"
  apt-get install -y "$work"/linux-*.deb
  rm -rf "$work"
fi

# --no-start: in a chroot there is no running system to configure or check
bash "$(dirname "$0")/install.sh" --no-start "$@"
