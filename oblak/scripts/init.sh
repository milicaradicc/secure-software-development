#!/bin/sh
# Pokreće se unutar Firecracker VM-a kao /sbin/init_oblak.sh

# Montiraj proc i sys
mount -t proc proc /proc
mount -t sysfs sysfs /sys

# Postavi hostname
hostname oblak-vm

# Instaliraj zavisnosti ako postoje (bez mreže — iz lokalnog cache-a)
if [ -f /function/requirements.txt ]; then
    echo "[INIT] Instalacija zavisnosti..."
    pip3 install -q --no-index -r /function/requirements.txt 2>&1 | head -5
fi

# Pokreni skriptu kao neprivilegovani korisnik, max 10 sekundi
echo "[INIT] Pokretanje..."
su -s /bin/sh runner -c "cd /function && timeout 10 python3 script.py 2>&1"

echo "[INIT] Exit: $?"

# Ugasi VM
echo o > /proc/sysrq-trigger