#!/bin/bash

# Quick and dirty script to prep image. This should be
# migrated to the proper Armbian build process once we can
# figure out how to get a bootable image w/ the BTF kernel
# included
# https://docs.armbian.com/Developer-Guide_User-Configurations/#user-provided-image-customization-script

usage() {
    echo "Usage: $0 -i <img> -m <mnt>"
    echo ""
    echo "Options:"
    echo "    -i <img>   Armbian base image file (uncompressed)"
    echo "    -m <mnt>   Mount point"
    exit 1
}

img=""
mnt=""

while getopts ":i:m:" opt; do
    case $opt in
        i)
            img="$OPTARG"
            ;;
        m)
            mnt="$OPTARG"
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

if [ -z "$img" ] || [ -z "$mnt" ]; then
    echo "Error: Both -i and -m options are required."
    usage
fi

set -e

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
sudo cp /usr/bin/qemu-aarch64-static ${mnt}/usr/bin/
sudo cp customize-image.sh ${mnt}/tmp/
sudo chmod +x ${mnt}/tmp/customize-image.sh
# https://stackoverflow.com/questions/8157931/bash-executing-commands-from-within-a-chroot-and-switch-user#8157973
sudo chroot ${mnt} qemu-aarch64-static /bin/bash -c "/tmp/customize-image.sh"

# Clean up
sudo umount ${mnt}
sudo e2fsck -p -f ${loop}p1
sudo losetup -d ${loop}
