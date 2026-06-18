# 8. Uputstvo za pokretanje

## 8.1 Preduslovi

Sistem se pokreće na Linux okruženju sa KVM podrškom (WSL2 ili pravi Linux).
Firecracker NE radi na Windowsu ni macOS-u.

```bash
# Provera KVM
ls -la /dev/kvm        # mora postojati

# Alati
sudo apt update
sudo apt install -y golang-go python3 debootstrap clamav qemu-system-x86
sudo apt install -y bandit     # ili: pip3 install bandit
```

Ako je WSL2, u `%USERPROFILE%\.wslconfig` na Windowsu mora biti:

```ini
[wsl2]
nestedVirtualization=true
```

(zatim `wsl --shutdown` i ponovo otvoriti WSL)

## 8.2 Setup Firecracker (jednom)

```bash
sudo bash scripts/setup_firecracker.sh    # binary + kernel
sudo bash scripts/build_rootfs.sh         # rootfs sa Pythonom
```

## 8.3 TLS sertifikat (jednom)

```bash
bash scripts/generate_tls_cert.sh
```

## 8.4 Konfiguracija (.env)

```bash
SECRET_KEY=<random hex string, npr. openssl rand -hex 32>
DATABASE_PATH=./oblak.db
PORT=8000
TLS_CERT_FILE=./certs/server.crt
TLS_KEY_FILE=./certs/server.key
# Opciono za LLM analizu:
# ANTHROPIC_API_KEY=sk-ant-...
```

## 8.5 ClamAV baza virusa (jednom)

```bash
sudo systemctl stop clamav-freshclam 2>/dev/null || sudo pkill freshclam
sudo freshclam
```

## 8.6 Build

```bash
go mod tidy
go build -o oblak-server ./cmd/server
go build -o oblak ./cmd/cli
```

## 8.7 Pokretanje servera

Server zahteva root (mount rootfs-a, KVM):

```bash
sudo ./oblak-server
```

Očekivani izlaz: `HTTPS server slusa na portu 8000`.

## 8.8 Korišćenje CLI (drugi terminal)

```bash
export OBLAK_SERVER=https://localhost:8000
export OBLAK_INSECURE_SKIP_VERIFY=true   # samo za self-signed demo

./oblak register -u milica -p lozinka123
./oblak login -u milica -p lozinka123
./oblak deploy tests/benign/hello.py
./oblak list
./oblak invoke https://localhost:8000/invoke/<function-id>
./oblak delete <function-id>
```

## 8.9 Deploy sa zavisnostima

```bash
./oblak deploy mojaskripta.py --requirements requirements.txt
```

## 8.10 Najčešći problemi

| Problem | Uzrok | Rešenje |
|---------|-------|---------|
| `/dev/kvm` ne postoji | nested virt isključen | uključiti u `.wslconfig` / VirtualBox |
| `address already in use` | server već radi | `netstat`/`pkill` stari proces |
| `database is locked` | dva servera rade | ugasiti stari pre brisanja `oblak.db` |
| `AV not available` | ClamAV nije instaliran | `apt install clamav` + `freshclam` |
| invoke vraća grešku | nema Firecracker (npr. Windows) | pokrenuti na Linux/WSL sa KVM |
