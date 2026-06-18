# 0. Pregled sistema i arhitektura

## Pregled

Oblak je serverless platforma za izvršavanje korisničkih Python funkcija. Sistem
je sličan osnovnoj verziji servisa kao što su AWS Lambda ili Google Cloud
Functions: korisnik deployuje Python skriptu, server je proverava, čuva
metapodatke i izvršava funkciju u izolovanom Firecracker microVM okruženju.

Glavna bezbednosna ideja sistema je da se nepoverljiv korisnički kod ne izvršava
direktno na hostu, već u kratkoživećoj microVM instanci sa ograničenim resursima.

## Arhitektura

```
┌──────────────┐      HTTPS       ┌─────────────────────────────────┐
│   CDK CLI    │ ───────────────► │            SERVER               │
│ (Go, Cobra)  │   JWT auth       │         (Go, Gin)               │
└──────────────┘                  │                                 │
                                  │  ┌──────────┐  ┌─────────────┐  │
                                  │  │   Auth   │  │   Storage   │  │
                                  │  │ JWT,bcr. │  │ SQLite+disk │  │
                                  │  └──────────┘  └─────────────┘  │
                                  │  ┌───────────────────────────┐  │
                                  │  │      Code Verifier        │  │
                                  │  │ pattern→ClamAV→Bandit→LLM │  │
                                  │  └───────────────────────────┘  │
                                  │  ┌───────────────────────────┐  │
                                  │  │  Firecracker Orchestrator │  │
                                  │  └─────────────┬─────────────┘  │
                                  └────────────────┼────────────────┘
                                       Unix socket │
                                  ┌────────────────▼────────────────┐
                                  │      Firecracker microVM         │
                                  │  1 vCPU, 128 MiB, bez mreže      │
                                  │  runner user, timeout 10s        │
                                  │      python3 script.py           │
                                  └──────────────────────────────────┘
```

## Komponente

**CLI klijent.** Go aplikacija koja omogućava registraciju, login, deploy,
listanje, brisanje i invoke funkcija. JWT token se čuva u korisničkom
konfiguracionom fajlu `~/.oblak/config.json` sa dozvolama `0600`.

**Server.** REST API napisan u Go jeziku uz Gin framework. Sluša na portu `8000`
ako nije drugačije zadato kroz `PORT` promenljivu. Odgovoran je za autentikaciju,
autorizaciju nad funkcijama, upload fajlova, statičku proveru koda, čuvanje
metapodataka i pokretanje Firecracker izvršavanja.

**Code verifier.** Proverava uploadovani Python kod pre izvršavanja. Koristi
custom pattern matching za opasne obrasce (`os.fork`, `shutil.rmtree`,
`os.remove`, pristup `/etc/passwd`, `/proc/`, `ctypes`, beskonačne petlje),
ClamAV antivirus i Bandit statičku analizu. Ako bilo koja provera blokira
funkciju, ona se čuva kao neverifikovana i ne može se izvršiti.

**Storage.** SQLite baza čuva korisnike i metapodatke o funkcijama. Kod funkcije
se čuva u `./functions/<function-id>/script.py`. Za funkciju se čuva i SHA-256
hash fajla.

**Firecracker orchestrator.** Za svako izvršavanje pravi privremeni rootfs
kopiranjem baznog image-a. Korisnički `script.py` i opcioni `requirements.txt` se
kopiraju u `/function` unutar rootfs-a, a zatim se pokreće Firecracker microVM (1
vCPU, 128 MiB, bez SMT-a, timeout). Firecracker API se kontroliše preko Unix
socketa, ne preko eksternog TCP porta.

**Audit log.** JSON audit log u `./logs/audit.log` (detaljnije u dokumentu 3).

**Rate limiting.** In-memory: login/register 5/min po IP, deploy 10/sat po
korisniku, invoke 30/min po IP.

## Tok izvršavanja

1. Korisnik se registruje ili loguje preko CLI-ja.
2. Server proverava kredencijale i izdaje JWT token.
3. Korisnik deployuje Python skriptu (opciono sa `requirements.txt`).
4. Server proverava ekstenziju `.py`, veličinu upload-a i pokreće verifier
   pipeline.
5. Server čuva funkciju, vlasnika, putanju, hash i status verifikacije u bazi.
6. Server generiše invoke URL oblika `/invoke/<function-id>`.
7. Kod invoke zahteva server učitava funkciju i proverava da li je verifikovana.
8. Orchestrator priprema privremeni rootfs i pokreće Firecracker microVM.
9. VM izvršava korisničku skriptu; server vraća output, exit code, trajanje i
   timeout status.
10. Privremeni rootfs, Unix socket i log fajl VM-a se uklanjaju nakon
    izvršavanja.

## Tehnologije

| Sloj | Tehnologija |
|------|-------------|
| Server | Go + Gin |
| CLI | Go + Cobra |
| Baza | SQLite (GORM, CGO-free driver) |
| Auth | JWT (HS256) + bcrypt |
| Statička analiza | Bandit + custom pattern matching |
| Antivirus | ClamAV |
| Izvršavanje | Firecracker microVM + KVM |
| Transport | HTTPS/TLS 1.2+ |
