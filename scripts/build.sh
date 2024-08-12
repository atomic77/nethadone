#!/bin/bash

# Quick and dirty script to prep image. This should be
# migrated to the proper Armbian build process once we can
# figure out how to get a bootable image w/ the BTF kernel
# included
# https://docs.armbian.com/Developer-Guide_User-Configurations/#user-provided-image-customization-script

usage() {
    echo "Usage: $0 -i <img> -m <mnt>"
    echo ""
    echo "This script will mount the provided image into the mount point and run"
    echo "customize-image.sh on it. If you have downloaded an .xz file from the Armbian"
    echo "site it's a good idea to keep the original compressed copy and uncompress the"
    echo "image into /dev/shm if you have the memory"
    echo ""
    echo "Options:"
    echo "    -i <img>   Armbian base image file to write into (uncompressed)"
    echo "    -m <mnt>   Mount location to use for chroot "
    echo "    -a <arch>   'aarch64' for orangepi R1+ and similar, 'arm' for opizero/r1, and similar"
    exit 1
}

cleanup () {
echo "Cleaning up.."
    sudo umount ${mnt}/dev
    sudo umount --lazy ${mnt}
    sudo e2fsck -p -f ${loop}p1
    sudo losetup -d ${loop}
}

trap cleanup ERR EXIT

img=""
mnt=""
arch=""

while getopts ":i:m:a:" opt; do
    case $opt in
        i)
            img="$OPTARG"
            ;;
        m)
            mnt="$OPTARG"
            ;;
        a)
            arch="$OPTARG"
            ;;
        \?)
            echo "Invalid option: -$OPTARG" >&2
            usage
            ;;
        :)
            echo "Option -$OPTARG requires an argument." >&2
            usage
            ;;
    esac
done

if [ -z "$img" ] || [ -z "$mnt" ] || [ -z "$arch" ]; then
    echo "Error: All flags are required."
    usage
fi

set -e

# After trimming down the image see if we can reduce this to 3 or 2g
truncate -s 4G $img
loop=$(sudo losetup --partscan --show --nooverlap -f $img)
# This may fail on older versions of growpart and require
# patching, due to the kernel version in the image build
# we're using
# https://github.com/canonical/cloud-utils/pull/2/files
sudo growpart ${loop} 1
sudo e2fsck -p -f ${loop}p1
sudo resize2fs ${loop}p1
printf "Resized image to 4G - fdisk output should reflect this\n"
sudo fdisk -l $loop
sudo mount ${loop}p1 ${mnt}
sudo mount --bind /dev ${mnt}/dev

# Copy files into image
sudo mkdir ${mnt}/tmp/overlay

# To enter into chroot w/ arm64 emulation
sudo cp /usr/bin/qemu-${arch}-static ${mnt}/usr/bin/
sudo cp customize-image.sh ${mnt}/tmp/

# Ubuntu Noble images have a symlink to /run/systemd/resolve/stub-resolv.conf which 
# breaks DNS when in chroot. Swap out the current users' resolv.conf so we can 
# install packages while in chroot
sudo mv ${mnt}/etc/resolv.conf ${mnt}/etc/resolv.conf.tmp
sudo cp -L /etc/resolv.conf ${mnt}/etc/
sudo chmod +x ${mnt}/tmp/customize-image.sh
sudo chroot ${mnt} qemu-${arch}-static /bin/bash -c "/tmp/customize-image.sh"

sudo mv -f ${mnt}/etc/resolv.conf.tmp ${mnt}/etc/resolv.conf
