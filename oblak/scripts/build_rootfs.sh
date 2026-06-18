#!/bin/bash
set -e

ROOTFS="/opt/firecracker/rootfs.ext4"
ROOTFS_SIZE_MB=512
MOUNT_DIR="/tmp/fc_rootfs_build"

echo "=== Kreiranje ext4 ==="
dd if=/dev/zero of=$ROOTFS bs=1M count=$ROOTFS_SIZE_MB status=progress
mkfs.ext4 -F $ROOTFS

echo "=== Montiranje ==="
mkdir -p $MOUNT_DIR
sudo mount -o loop $ROOTFS $MOUNT_DIR

echo "=== Instalacija minimalnog Debian + Python ==="
sudo debootstrap --variant=minbase bullseye $MOUNT_DIR https://deb.debian.org/debian
sudo chroot $MOUNT_DIR apt-get install -y python3 python3-pip --no-install-recommends
sudo chroot $MOUNT_DIR apt-get clean

echo "=== Kreiranje runner korisnika (uid=1000) ==="
sudo chroot $MOUNT_DIR useradd -m -s /bin/sh -u 1000 runner

echo "=== Kreiranje direktorijuma ==="
sudo mkdir -p $MOUNT_DIR/function
sudo chown 1000:1000 $MOUNT_DIR/function

echo "=== Kopiranje init skripte ==="
sudo cp scripts/init.sh $MOUNT_DIR/sbin/init_oblak.sh
sudo chmod +x $MOUNT_DIR/sbin/init_oblak.sh

echo "=== Unmount ==="
sudo umount $MOUNT_DIR
rmdir $MOUNT_DIR

echo "=== Rootfs kreiran: $ROOTFS ==="
du -sh $ROOTFS
