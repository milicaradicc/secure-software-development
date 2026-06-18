#!/bin/bash
set -e

echo "=== Provjera KVM ==="
if [ ! -e /dev/kvm ]; then
    echo "GREŠKA: /dev/kvm ne postoji. Treba bare-metal Linux sa KVM."
    exit 1
fi

sudo usermod -aG kvm $USER

echo "=== Preuzimanje Firecracker ==="
FC_VERSION="v1.7.0"
wget -q "https://github.com/firecracker-microvm/firecracker/releases/download/${FC_VERSION}/firecracker-${FC_VERSION}-x86_64.tgz"
tar -xzf "firecracker-${FC_VERSION}-x86_64.tgz"
sudo mv "release-${FC_VERSION}-x86_64/firecracker-${FC_VERSION}-x86_64" /usr/local/bin/firecracker
sudo chmod +x /usr/local/bin/firecracker
rm -rf "firecracker-${FC_VERSION}-x86_64.tgz" "release-${FC_VERSION}-x86_64"

echo "=== Preuzimanje kernela ==="
sudo mkdir -p /opt/firecracker
sudo wget -q -O /opt/firecracker/vmlinux \
    "https://s3.amazonaws.com/spec.ccfc.min/img/quickstart_guide/x86_64/kernels/vmlinux.bin"

echo "=== Gotovo ==="
firecracker --version