# 2. Threat model (model pretnji)

## Aktori

**Legitimni korisnik** - deployuje i izvršava sopstvene Python funkcije.

**Maliciozni autentifikovani korisnik** - pokušava da deployuje opasan kod,
iscrpi resurse ili dođe do tuđih funkcija.

**Napadač bez autentikacije** - pokušava brute force login, koristi javni invoke
endpoint ili traži ranjivosti u API-ju.

**Napadač na mreži** - pokušava da presretne saobraćaj, JWT token ili login
podatke.

**Kompromitovan dependency ili alat** - može uticati na Bandit, ClamAV,
Firecracker binary, kernel ili rootfs image.

## Imovina koja se štiti

- host sistem i KVM okruženje;
- Firecracker binary, kernel i bazni rootfs image;
- SQLite baza;
- korisnički nalozi i bcrypt hash lozinki;
- JWT secret i izdati tokeni;
- uploadovane Python funkcije;
- audit log;
- dostupnost servisa;
- integritet verifier pipeline-a.

## Granice poverenja

```
┌─────────┐  Granica 1   ┌──────────┐  Granica 3  ┌──────────────┐
│ CLI/usr │ ───HTTPS───► │  SERVER  │ ──────────► │ verifier     │
└─────────┘              │          │             │ (Bandit,AV)  │
                         │          │             └──────────────┘
                         │          │  Granica 2  ┌──────────────┐
                         │          │ ──────────► │ lokalni disk │
                         │          │             │ (db,fajlovi) │
                         │          │             │              │
                         │          │             └──────────────┘
                         │          │   Granica 4  
                         │          │ ──Unix sock─► Firecracker API
                         └──────────┘                     │
                                          Granica 5       ▼
                                     (najkritičnija)  microVM ──► host
```

**Granica 1: korisnik/CLI – server.** Koristi HTTPS. Autentikacija preko JWT
tokena, kanal šifrovan TLS-om.

**Granica 2: server – lokalni storage.** Server upisuje fajlove funkcija, SQLite
bazu i audit log na lokalni disk. Pretpostavlja se da host fajl sistem i OS dozvole
štite ove fajlove.

**Granica 3: server – verifier alati.** Server pokreće lokalne alate (Bandit,
ClamAV). Rezultat provere utiče na to da li će funkcija moći da se izvrši.

**Granica 4: server – Firecracker API.** Server upravlja Firecracker procesom
preko Unix socketa u `/tmp/firecracker`. Visoko privilegovana lokalna kontrolna
ravan.

**Granica 5: microVM – host.** Najkritičnija granica. Korisnički kod je
nepoverljiv i mora ostati unutar VM-a. Firecracker i KVM obezbeđuju izolaciju, ali
VM escape ostaje visoko-uticajna pretnja.

## Pretpostavke

- Sistem se pokreće u kontrolisanom WSL2/Linux okruženju.
- Firecracker binary, kernel i rootfs image dolaze iz pouzdanog izvora.
- Server ima dovoljne privilegije da mountuje rootfs i pokreće Firecracker.
- VM nema eksplicitno konfigurisan mrežni interfejs u trenutnom kodu.
- SQLite baza je lokalna i nije deljena preko mreže.

